package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type MountOptions struct {
	Name         string `yaml:"Name"`
	User         string `yaml:"User"`
	Password     string `yaml:"Password"`
	Description  string `yaml:"Description"`
	BitRate      int    `yaml:"BitRate"`
	Genre        string `yaml:"Genre"`
	BurstSize    int    `yaml:"BurstSize"`
	DumpFile     string `yaml:"DumpFile"`
	MaxListeners int    `yaml:"MaxListeners"`
}

type Options struct {
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
		Loglevel   int  `yaml:"Loglevel"`
		LogSize    int  `yaml:"LogSize"`
		UseMonitor bool `yaml:"UseMonitor"`
		UseStat    bool `yaml:"UseStat"`
	} `yaml:"Logging"`

	Mounts []MountOptions `yaml:"Mounts"`
}

func (o *Options) Load() error {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}
	return yaml.Unmarshal(yamlFile, o)
}
