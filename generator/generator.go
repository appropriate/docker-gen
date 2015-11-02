package generator

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/jwilder/docker-gen/utils"
)

type stringslice []string

var (
	tlsCert   string
	tlsKey    string
	tlsCaCert string
	tlsVerify bool
)

type Event struct {
	ContainerID string `json:"id"`
	Status      string `json:"status"`
	Image       string `json:"from"`
}

type Address struct {
	IP           string
	IP6LinkLocal string
	IP6Global    string
	Port         string
	HostPort     string
	Proto        string
	HostIP       string
}

type Volume struct {
	Path      string
	HostPath  string
	ReadWrite bool
}

type RuntimeContainer struct {
	ID           string
	Addresses    []Address
	Gateway      string
	Name         string
	Hostname     string
	Image        DockerImage
	Env          map[string]string
	Volumes      map[string]Volume
	Node         SwarmNode
	Labels       map[string]string
	IP           string
	IP6LinkLocal string
	IP6Global    string
}

type DockerImage struct {
	Registry   string
	Repository string
	Tag        string
}

type SwarmNode struct {
	ID      string
	Name    string
	Address Address
}

func (strings *stringslice) String() string {
	return "[]"
}

func (strings *stringslice) Set(value string) error {
	// TODO: Throw an error for duplicate `dest`
	*strings = append(*strings, value)
	return nil
}

func (i *DockerImage) String() string {
	ret := i.Repository
	if i.Registry != "" {
		ret = i.Registry + "/" + i.Repository
	}
	if i.Tag != "" {
		ret = ret + ":" + i.Tag
	}
	return ret
}

type Config struct {
	Template         string
	Dest             string
	Watch            bool
	NotifyCmd        string
	NotifyContainers map[string]docker.Signal
	OnlyExposed      bool
	OnlyPublished    bool
	Interval         int
	KeepBlankLines   bool
}

type ConfigFile struct {
	Config []Config
}

type Generator struct {
	config GeneratorConfig

	client *docker.Client
	wg     sync.WaitGroup
}

func NewGenerator(config GeneratorConfig) (*Generator, error) {
	generator := &Generator{config: config}

	client, err := newDockerClient(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Unable to create docker client: %s", err)
	}

	generator.client = client

	return generator, nil
}

func (g *Generator) Generate() {
	g.generateFromContainers()
	g.generateAtInterval()
	g.generateFromEvents()
	g.wg.Wait()
}

type Context []*RuntimeContainer

func (c *Context) Env() map[string]string {
	return utils.SplitKeyValueSlice(os.Environ())
}

func (c *ConfigFile) filterWatches() ConfigFile {
	configWithWatches := []Config{}

	for _, config := range c.Config {
		if config.Watch {
			configWithWatches = append(configWithWatches, config)
		}
	}
	return ConfigFile{
		Config: configWithWatches,
	}
}

func (r *RuntimeContainer) Equals(o RuntimeContainer) bool {
	return r.ID == o.ID && r.Image == o.Image
}

func (r *RuntimeContainer) PublishedAddresses() []Address {
	mapped := []Address{}
	for _, address := range r.Addresses {
		if address.HostPort != "" {
			mapped = append(mapped, address)
		}
	}
	return mapped
}

func tlsEnabled() bool {
	for _, v := range []string{tlsCert, tlsCaCert, tlsKey} {
		if e, err := utils.PathExists(v); e && err == nil {
			return true
		}
	}
	return false
}

func newDockerClient(endpoint string) (*docker.Client, error) {
	if strings.HasPrefix(endpoint, "unix:") {
		return docker.NewClient(endpoint)
	} else if tlsVerify || tlsEnabled() {
		if tlsVerify {
			if e, err := utils.PathExists(tlsCaCert); !e || err != nil {
				return nil, errors.New("TLS verification was requested, but CA cert does not exist")
			}
		}

		return docker.NewTLSClient(endpoint, tlsCert, tlsKey, tlsCaCert)
	}
	return docker.NewClient(endpoint)
}

func (g *Generator) generateFromContainers() {
	containers, err := getContainers(g.client)
	if err != nil {
		log.Printf("error listing containers: %s\n", err)
		return
	}
	for _, config := range g.configs.Config {
		changed := generateFile(config, containers)
		if !changed {
			log.Printf("Contents of %s did not change. Skipping notification '%s'", config.Dest, config.NotifyCmd)
			continue
		}
		runNotifyCmd(config)
		sendSignalToContainer(g.client, config)
	}
}

func runNotifyCmd(config Config) {
	if config.NotifyCmd == "" {
		return
	}

	log.Printf("Running '%s'", config.NotifyCmd)
	cmd := exec.Command("/bin/sh", "-c", config.NotifyCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running notify command: %s, %s\n", config.NotifyCmd, err)
		log.Print(string(out))
	}
}

func sendSignalToContainer(client *docker.Client, config Config) {
	if len(config.NotifyContainers) < 1 {
		return
	}

	for container, signal := range config.NotifyContainers {
		log.Printf("Sending container '%s' signal '%v'", container, signal)
		killOpts := docker.KillContainerOptions{
			ID:     container,
			Signal: signal,
		}
		if err := client.KillContainer(killOpts); err != nil {
			log.Printf("Error sending signal to container: %s", err)
		}
	}
}

func (g *Generator) generateAtInterval() {
	for _, config := range g.configs.Config {

		if config.Interval == 0 {
			continue
		}

		log.Printf("Generating every %d seconds", config.Interval)
		g.wg.Add(1)
		ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
		quit := make(chan struct{})
		configCopy := config
		go func() {
			defer g.wg.Done()
			for {
				select {
				case <-ticker.C:
					containers, err := getContainers(g.client)
					if err != nil {
						log.Printf("Error listing containers: %s\n", err)
						continue
					}
					// ignore changed return value. always run notify command
					generateFile(configCopy, containers)
					runNotifyCmd(configCopy)
					sendSignalToContainer(g.client, configCopy)
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
	}
}

func (g *Generator) generateFromEvents() {
	g.configs = g.configs.filterWatches()
	if len(g.configs.Config) == 0 {
		return
	}

	g.wg.Add(1)
	defer g.wg.Done()

	for {
		if g.client == nil {
			var err error
			endpoint, err := utils.GetEndpoint(g.endpoint)
			if err != nil {
				log.Printf("Bad endpoint: %s", err)
				time.Sleep(10 * time.Second)
				continue
			}

			g.client, err = newDockerClient(endpoint)
			if err != nil {
				log.Printf("Unable to connect to docker daemon: %s", err)
				time.Sleep(10 * time.Second)
				continue
			}
			g.generateFromContainers()
		}

		eventChan := make(chan *docker.APIEvents, 100)
		defer close(eventChan)

		watching := false
		for {

			if g.client == nil {
				break
			}
			err := g.client.Ping()
			if err != nil {
				log.Printf("Unable to ping docker daemon: %s", err)
				if watching {
					g.client.RemoveEventListener(eventChan)
					watching = false
					g.client = nil
				}
				time.Sleep(10 * time.Second)
				break

			}

			if !watching {
				err = g.client.AddEventListener(eventChan)
				if err != nil && err != docker.ErrListenerAlreadyExists {
					log.Printf("Error registering docker event listener: %s", err)
					time.Sleep(10 * time.Second)
					continue
				}
				watching = true
				log.Println("Watching docker events")
			}

			select {

			case event := <-eventChan:
				if event == nil {
					if watching {
						g.client.RemoveEventListener(eventChan)
						watching = false
						g.client = nil
					}
					break
				}

				if event.Status == "start" || event.Status == "stop" || event.Status == "die" {
					log.Printf("Received event %s for container %s", event.Status, event.ID[:12])
					g.generateFromContainers()
				}
			case <-time.After(10 * time.Second):
				// check for docker liveness
			}

		}
	}
}
