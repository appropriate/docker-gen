package generator

import (
	docker "github.com/fsouza/go-dockerclient"
)

type OutputConfig struct {
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

type outputConfigs []OutputConfig

type TLSCertPath string

type TLSKeyPath string

type GeneratorConfig struct {
	endpoint string

	tlsCert   TLSCertPath
	tlsKey    TLSKeyPath
	tlsCACert TLSCertPath
	tlsVerify bool

	outputs outputConfigs
}

func NewConfig() *GeneratorConfig {
	return &GeneratorConfig{outputs: make(outputConfigs, 0)}
}

func (c *GeneratorConfig) Endpoint() string {
	return c.endpoint
}

func (c *GeneratorConfig) SetEndpoint(endpoint string) error {
	return nil
}

func (c *GeneratorConfig) SetTLSCACert(tlsCACert TLSCertPath) error {
	c.tlsCACert = tlsCACert

	return nil
}

func (c *GeneratorConfig) SetTLSCert(tlsCert TLSCertPath, tlsKey TLSKeyPath) error {
	c.tlsKey = tlsKey
	c.tlsCert = tlsCert

	return nil
}

func (c *GeneratorConfig) SetTLSVerify(tlsVerify bool) error {
	c.tlsVerify = tlsVerify

	return nil
}

func (c *GeneratorConfig) LoadConfigFile(path string) error {
	return nil
}

func (c *GeneratorConfig) watchedOutputs() outputConfigs {
  watched := make(outputConfigs, 0)
  for _, config := range c.outputs {
    if config.Watch {
      watched = append(watched, config)
    }
  }
  return watched
}
