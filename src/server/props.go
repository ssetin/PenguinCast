// Package iceserver - icecast streaming server
package iceserver

import (
	"encoding/json"
	"io/ioutil"
)

// Properties ...
type Properties struct {
	Name     string `json:"Name"`
	Admin    string `json:"Admin,omitempty"`
	Location string `json:"Location,omitempty"`
	Host     string `json:"Host"`

	Socket struct {
		Port int `json:"Port"`
	} `json:"Socket"`

	Limits struct {
		Clients                int32 `json:"Clients"`
		Sources                int32 `json:"Sources"`
		SourceIdleTimeOut      int   `json:"SourceIdleTimeOut"`
		EmptyBufferIdleTimeOut int   `json:"EmptyBufferIdleTimeOut"`
	} `json:"Limits"`

	Auth struct {
		AdminPassword string `json:"AdminPassword"`
	} `json:"Auth"`

	Paths struct {
		Base string `json:"Base"`
		Web  string `json:"Web"`
		Log  string `json:"Log"`
	} `json:"Paths"`

	Logging struct {
		Loglevel   int  `json:"Loglevel"`
		Logsize    int  `json:"Logsize"`
		UseMonitor bool `json:"UseMonitor"`
		UseStat    bool `json:"UseStat"`
	} `json:"Logging"`

	Mounts []Mount `json:"Mounts"`
}

func (i *IceServer) initConfig() error {
	cfile, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(cfile, &i.Props)
	if err != nil {
		return err
	}

	return nil
}
