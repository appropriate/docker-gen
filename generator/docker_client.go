package generator

import (
	"log"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/jwilder/docker-gen/utils"
)

func splitDockerImage(img string) (string, string, string) {
	index := 0
	repository := img
	var registry, tag string
	if strings.Contains(img, "/") {
		separator := strings.Index(img, "/")
		registry = img[index:separator]
		index = separator + 1
		repository = img[index:]
	}

	if strings.Contains(repository, ":") {
		separator := strings.Index(repository, ":")
		tag = repository[separator+1:]
		repository = repository[0:separator]
	}

	return registry, repository, tag
}

func getContainers(client *docker.Client) ([]*RuntimeContainer, error) {

	apiContainers, err := client.ListContainers(docker.ListContainersOptions{
		All:  false,
		Size: false,
	})
	if err != nil {
		return nil, err
	}

	containers := []*RuntimeContainer{}
	for _, apiContainer := range apiContainers {
		container, err := client.InspectContainer(apiContainer.ID)
		if err != nil {
			log.Printf("error inspecting container: %s: %s\n", apiContainer.ID, err)
			continue
		}

		registry, repository, tag := splitDockerImage(container.Config.Image)
		runtimeContainer := &RuntimeContainer{
			ID: container.ID,
			Image: DockerImage{
				Registry:   registry,
				Repository: repository,
				Tag:        tag,
			},
			Name:         strings.TrimLeft(container.Name, "/"),
			Hostname:     container.Config.Hostname,
			Gateway:      container.NetworkSettings.Gateway,
			Addresses:    []Address{},
			Env:          make(map[string]string),
			Volumes:      make(map[string]Volume),
			Node:         SwarmNode{},
			Labels:       make(map[string]string),
			IP:           container.NetworkSettings.IPAddress,
			IP6LinkLocal: container.NetworkSettings.LinkLocalIPv6Address,
			IP6Global:    container.NetworkSettings.GlobalIPv6Address,
		}
		for k, v := range container.NetworkSettings.Ports {
			address := Address{
				IP:           container.NetworkSettings.IPAddress,
				IP6LinkLocal: container.NetworkSettings.LinkLocalIPv6Address,
				IP6Global:    container.NetworkSettings.GlobalIPv6Address,
				Port:         k.Port(),
				Proto:        k.Proto(),
			}
			if len(v) > 0 {
				address.HostPort = v[0].HostPort
				address.HostIP = v[0].HostIP
			}
			runtimeContainer.Addresses = append(runtimeContainer.Addresses,
				address)

		}
		for k, v := range container.Volumes {
			runtimeContainer.Volumes[k] = Volume{
				Path:      k,
				HostPath:  v,
				ReadWrite: container.VolumesRW[k],
			}
		}
		if container.Node != nil {
			runtimeContainer.Node.ID = container.Node.ID
			runtimeContainer.Node.Name = container.Node.Name
			runtimeContainer.Node.Address = Address{
				IP: container.Node.IP,
			}
		}

		runtimeContainer.Env = utils.SplitKeyValueSlice(container.Config.Env)
		runtimeContainer.Labels = container.Config.Labels
		containers = append(containers, runtimeContainer)
	}
	return containers, nil

}
