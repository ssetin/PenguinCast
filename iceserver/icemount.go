package iceserver

import (
	"encoding/base64"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// MetaData ...
type MetaData struct {
	MetaInt     int
	StreamTitle string
}

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
	BurstSize   int
	DumpFile    string

	State struct {
		Status    string
		Started   string
		MetaInfo  MetaData
		Listeners int
	}

	mux      sync.Mutex
	Server   *IceServer
	buffer   BufferQueue
	dumpFile *os.File
}

//Init ...
func (m *Mount) Init(srv *IceServer) error {
	m.Server = srv
	m.Clear()
	m.State.MetaInfo.MetaInt = m.BitRate * 1024 / 8 * 10
	m.buffer.Init(m.BurstSize/(m.BitRate*1024/8) + 1)
	if m.DumpFile > "" {
		var err error
		m.dumpFile, err = os.OpenFile(srv.Props.Paths.Log+m.DumpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}
	return nil
}

//Close ...
func (m *Mount) Close() {
	if m.dumpFile != nil {
		m.dumpFile.Close()
	}
}

//Clear ...
func (m *Mount) Clear() {
	m.State.Status = "Offline"
	m.State.Started = ""
	m.zeroListeners()
	m.State.MetaInfo.StreamTitle = ""
	m.StreamURL = m.Server.Props.Host + ":" + strconv.Itoa(m.Server.Props.Socket.Port) + "/" + m.Name
}

func (m *Mount) incListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.State.Listeners++
}

func (m *Mount) decListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.State.Listeners > 0 {
		m.State.Listeners--
	}
}

func (m *Mount) zeroListeners() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.State.Listeners = 0
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

func (m *Mount) getParams(paramstr string) map[string]string {
	var rex = regexp.MustCompile("(\\w+)=(\\w+)")
	data := rex.FindAllStringSubmatch(paramstr, -1)
	params := make(map[string]string)
	for _, kv := range data {
		k := kv[1]
		v := kv[2]
		params[k] = v
	}
	return params
}

func (m *Mount) writeICEHeaders(w http.ResponseWriter, r *http.Request) {
	var bitratestr string
	bitratestr = r.Header.Get("ice-bitrate")
	if bitratestr == "" {
		audioinfo := r.Header.Get("ice-audio-info")
		if len(audioinfo) > 3 {
			params := m.getParams(audioinfo)
			bitratestr = params["bitrate"]
		}
	}

	brate, err := strconv.Atoi(bitratestr)
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

func (m *Mount) updateMeta(w http.ResponseWriter, r *http.Request) {
	m.mux.Lock()
	if m.State.Status != "On air" {
		err := m.auth(w, r)
		if err != nil {
			m.mux.Unlock()
			return
		}
	}

	//song, _ := url.QueryUnescape(r.URL.Query().Get("song"))
	m.State.MetaInfo.StreamTitle = r.URL.Query().Get("song")

	m.mux.Unlock()
}

// icy style metadata
func (m *Mount) getIcyMeta() []byte {
	m.mux.Lock()
	defer m.mux.Unlock()

	var metaSize byte
	var result string
	if m.State.MetaInfo.StreamTitle > "" {
		result = "StreamTitle='" + m.State.MetaInfo.StreamTitle + "';"
	} else {
		result = "StreamTitle='" + m.Description + "';"
	}
	metaSize = byte(math.Ceil(float64(len(result)) / 16.0))

	meta := make([]byte, metaSize*16+1)
	meta[0] = metaSize

	for idx := 0; idx < len(result); idx++ {
		meta[idx+1] = result[idx]
	}

	return meta
}
