package iceserver

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// MetaData ...
type MetaData struct {
	MetaInt     int
	StreamTitle string
}

// Mount ...
type Mount struct {
	Name        string `json:"Name"`
	User        string `json:"User"`
	Password    string `json:"Password"`
	Description string `json:"Description"`
	BitRate     int    `json:"BitRate"`
	ContentType string `json:"ContentType"`
	StreamURL   string `json:"StreamURL"`
	Genre       string `json:"Genre"`
	BurstSize   int    `json:"BurstSize"`
	DumpFile    string `json:"DumpFile"`

	State struct {
		Status    string
		Started   string
		MetaInfo  MetaData
		Listeners int
	} `json:"-"`

	mux      sync.Mutex
	Server   *IceServer `json:"-"`
	buffer   BufferQueue
	dumpFile *os.File
}

//Init ...
func (m *Mount) Init(srv *IceServer) error {
	m.Server = srv
	m.Clear()
	m.State.MetaInfo.MetaInt = m.BitRate * 1024 / 8 * 10
	m.buffer.Init(m.BurstSize/(m.BitRate*1024/8) + 5)
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
	m.mux.Lock()
	defer m.mux.Unlock()
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
	m.State.Listeners = 0
}

func (m *Mount) auth(w http.ResponseWriter, r *http.Request) error {
	strAuth := r.Header.Get("authorization")

	if strAuth == "" {
		m.Server.sayHello(w, r)
		return errors.New("No authorization field")
	}

	s := strings.SplitN(strAuth, " ", 2)
	if len(s) != 2 {
		http.Error(w, "Not authorized", 401)
		return errors.New("Not authorized")
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return err
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		http.Error(w, "Not authorized", 401)
		return errors.New("Not authorized")
	}

	if m.Password != pair[1] && m.User != pair[0] {
		http.Error(w, "Not authorized", 401)
		return errors.New("Wrong user or password")
	}

	m.Server.sayHello(w, r)

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
	//m.StreamURL = r.Header.Get("ice-url")
	m.Description = r.Header.Get("ice-description")
}

func (m *Mount) updateMeta(w http.ResponseWriter, r *http.Request) {
	m.mux.Lock()
	defer m.mux.Unlock()

	err := m.auth(w, r)
	if err != nil {
		return
	}

	song := r.URL.Query().Get("song")
	songReader := strings.NewReader(song)

	enc, _, _ := charset.DetermineEncoding(([]byte)(song), "")
	utf8Reader := transform.NewReader(songReader, enc.NewDecoder())
	result, err := ioutil.ReadAll(utf8Reader)
	if err != nil {
		m.State.MetaInfo.StreamTitle = ""
		return
	}
	m.State.MetaInfo.StreamTitle = string(result[:])
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
