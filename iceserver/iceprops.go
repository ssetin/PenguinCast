package icyserver

import (
	"encoding/json"
	"io/ioutil"
)

// Mount ...
type Mount struct {
	Name        string
	Password    string
	Description string
	BitRate     int
	ContentType string
	StreamURL   string
	Genre       string

	Status struct {
		Status    string
		SongTitle string
		Listeners int
	}

	MetaInt int
	buffer  []byte
}

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
		Clients int
		Sources int
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

//****************************************************

func (i *IcyServer) initConfig() error {
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

//Init ...
func (m *Mount) Init(urlprefix string, buflen int, metaint int) error {
	m.Status.Status = "Offline"
	m.StreamURL = urlprefix + "/" + m.Name
	m.MetaInt = metaint
	m.buffer = make([]byte, buflen)
	return nil
}
