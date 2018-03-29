package icyserver

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
)

// Mount ...
type Mount struct {
	Name        string
	User        string
	Password    string
	Description string
	BitRate     int
	ContentType string
	StreamURL   string
	Genre       string

	Status struct {
		Status    string
		Started   string
		SongTitle string
		Listeners int
	}

	Server     *IcyServer
	MetaInt    int
	BufferSize int
	buffer     []byte
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
func (m *Mount) Init(srv *IcyServer) error {
	m.Server = srv
	m.Clear()
	m.MetaInt = 256000
	m.BufferSize = m.BitRate * 1024 / 8 * 100
	m.buffer = make([]byte, m.BufferSize)
	return nil
}

//Clear ...
func (m *Mount) Clear() {
	m.Status.Status = "Offline"
	m.Status.Started = ""
	m.Status.Listeners = 0
	m.Status.SongTitle = ""
	//m.BitRate = 0
	m.StreamURL = m.Server.Props.Host + ":" + strconv.Itoa(m.Server.Props.Socket.Port)
}
