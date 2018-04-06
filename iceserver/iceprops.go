package iceserver

import (
	"encoding/json"
	"io/ioutil"
)

// Properties ...
type Properties struct {
	Name     string
	Admin    string
	Location string
	Host     string

	Socket struct {
		Port int
	}

	Limits struct {
		Clients                int
		Sources                int
		SourceIdleTimeOut      int
		EmptyBufferIdleTimeOut int
		MaxBufferLength        int
	}

	Auth struct {
		AdminPassword string
	}

	Paths struct {
		Base string
		Web  string
		Log  string
	}

	Logging struct {
		Loglevel int
		Logsize  int
	}

	Mounts []Mount
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
