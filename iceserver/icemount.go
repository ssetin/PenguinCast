package iceserver

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

	mux        sync.Mutex
	Server     *IceServer
	MetaInt    int
	BufferSize int
	buffer     []byte
}

//Init ...
func (m *Mount) Init(srv *IceServer) error {
	m.Server = srv
	m.Clear()
	m.MetaInt = 256000
	m.BufferSize = m.BitRate * 1024 / 8 * 2
	m.buffer = make([]byte, m.BufferSize)
	return nil
}

//Clear ...
func (m *Mount) Clear() {
	m.Status.Status = "Offline"
	m.Status.Started = ""
	m.zeroListeners()
	m.Status.SongTitle = ""
	//m.BitRate = 0
	m.StreamURL = m.Server.Props.Host + ":" + strconv.Itoa(m.Server.Props.Socket.Port)
}

func (m *Mount) incListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Status.Listeners++
}

func (m *Mount) decListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Status.Listeners--
}

func (m *Mount) zeroListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Status.Listeners = 0
}

func (m *Mount) auth(w http.ResponseWriter, r *http.Request) error {
	strAuth := r.Header.Get("authorization")

	s := strings.SplitN(strAuth, " ", 2)

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return err
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		http.Error(w, "Not authorized", 401)
		return err
	}

	if m.Password != pair[1] && m.User != pair[0] {
		http.Error(w, "Not authorized", 401)
		return err
	}

	w.Header().Set("Server", m.Server.serverName+"/"+m.Server.version)
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Allow", "GET, SOURCE")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	flusher.Flush()

	return nil
}

func (m *Mount) writeICEHeaders(w http.ResponseWriter, r *http.Request) {
	brate, err := strconv.Atoi(r.Header.Get("ice-bitrate"))
	if err != nil {
		m.BitRate = 0
	} else {
		m.BitRate = brate
	}
	m.Genre = r.Header.Get("ice-genre")
	m.ContentType = r.Header.Get("content-type")
	m.StreamURL = r.Header.Get("ice-url")
	m.Description = r.Header.Get("ice-description")
}
