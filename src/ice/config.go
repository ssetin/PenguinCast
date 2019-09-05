package ice

import (
	"io/ioutil"

	"github.com/ssetin/PenguinCast/src/log"

	"gopkg.in/yaml.v3"
)

type options struct {
	Name     string `yaml:"Name"`
	Admin    string `yaml:"Admin,omitempty"`
	Location string `yaml:"Location,omitempty"`
	Host     string `yaml:"Host"`

	Socket struct {
		Port int `yaml:"Port"`
	} `yaml:"Socket"`

	Limits struct {
		Clients                int32 `yaml:"Clients"`
		Sources                int32 `yaml:"Sources"`
		SourceIdleTimeOut      int   `yaml:"SourceIdleTimeOut"`
		EmptyBufferIdleTimeOut int   `yaml:"EmptyBufferIdleTimeOut"`
		WriteTimeOut           int   `yaml:"WriteTimeOut"`
	} `yaml:"Limits"`

	Auth struct {
		AdminPassword string `yaml:"AdminPassword"`
	} `yaml:"Auth"`

	Paths struct {
		Base string `yaml:"Base"`
		Web  string `yaml:"Web"`
		Log  string `yaml:"Log"`
	} `yaml:"Paths"`

	Logging struct {
		LogLevel        log.LogsLevel `yaml:"LogLevel"`
		LogSize         int           `yaml:"LogSize"`
		UseMonitor      bool          `yaml:"UseMonitor"`
		MonitorInterval int           `yaml:"MonitorInterval"`
		UseStat         bool          `yaml:"UseStat"`
		StatInterval    int           `yaml:"StatInterval"`
	} `yaml:"Logging"`

	Mounts []mount `yaml:"Mounts"`
}

func (o *options) Load() error {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}
	return yaml.Unmarshal(yamlFile, o)
}
