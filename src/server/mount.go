// Package iceserver - icecast streaming server
package iceserver

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// MetaData ...
type MetaData struct {
	MetaInt     int
	StreamTitle string
}

// MountInfo ...
type MountInfo struct {
	Name      string
	Listeners int32
	UpTime    string
	Buff      BufferInfo
}

// Mount ...
type Mount struct {
	Name         string `json:"Name"`
	User         string `json:"User"`
	Password     string `json:"Password"`
	Description  string `json:"Description"`
	BitRate      int    `json:"BitRate"`
	ContentType  string `json:"ContentType"`
	StreamURL    string `json:"StreamURL"`
	Genre        string `json:"Genre"`
	BurstSize    int    `json:"BurstSize"`
	DumpFile     string `json:"DumpFile"`
	MaxListeners int    `json:"MaxListeners"`

	State struct {
		Started         bool
		StartedTime     time.Time
		MetaInfo        MetaData
		metaInfoCounter int32
		Listeners       int32
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
	m.buffer.Init(m.BurstSize/(m.BitRate*1024/8) + 2)
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
	m.State.Started = false
	m.State.StartedTime = time.Time{}
	m.zeroListeners()
	m.State.MetaInfo.StreamTitle = ""
	m.StreamURL = "http://" + m.Server.Props.Host + ":" + strconv.Itoa(m.Server.Props.Socket.Port) + "/" + m.Name
}

func (m *Mount) metaInfoChanged(val int32) bool {
	storedVal := atomic.LoadInt32(&m.State.metaInfoCounter)
	if storedVal == val {
		return false
	}
	return true
}

func (m *Mount) incListeners() {
	atomic.AddInt32(&m.State.Listeners, 1)
}

func (m *Mount) decListeners() {
	if atomic.LoadInt32(&m.State.Listeners) > 0 {
		atomic.AddInt32(&m.State.Listeners, -1)
	}
}

func (m *Mount) zeroListeners() {
	atomic.StoreInt32(&m.State.Listeners, 0)
}

func (m *Mount) auth(w http.ResponseWriter, r *http.Request) error {
	strAuth := r.Header.Get("authorization")

	if strAuth == "" {
		m.saySourceHello(w, r)
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

	m.saySourceHello(w, r)

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

func (m *Mount) sayListenerHello(w *bufio.Writer, icymeta bool) {
	w.WriteString("HTTP/1.0 200 OK\r\n")
	w.WriteString("Server: " + m.Server.serverName + " " + m.Server.version + "\r\n")
	w.WriteString("Content-Type: " + m.ContentType + "\r\n")
	w.WriteString("Connection: Keep-Alive\r\n")
	w.WriteString("X-Audiocast-Bitrate: " + strconv.Itoa(m.BitRate) + "\r\n")
	w.WriteString("X-Audiocast-Name: " + m.Name + "\r\n")
	w.WriteString("X-Audiocast-Genre: " + m.Genre + "\r\n")
	w.WriteString("X-Audiocast-Url: " + m.StreamURL + "\r\n")
	w.WriteString("X-Audiocast-Public: 0\r\n")
	w.WriteString("X-Audiocast-Description: " + m.Description + "\r\n")
	if icymeta {
		w.WriteString("Icy-Metaint: " + strconv.Itoa(m.State.MetaInfo.MetaInt) + "\r\n")
	}
	w.WriteString("\r\n")
	w.Flush()
}

func (m *Mount) saySourceHello(w http.ResponseWriter, r *http.Request) {
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
	m.Description = r.Header.Get("ice-description")
}

func (m *Mount) updateMeta(w http.ResponseWriter, r *http.Request) {
	err := m.auth(w, r)
	if err != nil {
		return
	}

	song := r.URL.Query().Get("song")
	songReader := strings.NewReader(song)
	enc, _, _ := charset.DetermineEncoding(([]byte)(song), "")
	utf8Reader := transform.NewReader(songReader, enc.NewDecoder())
	result, err := ioutil.ReadAll(utf8Reader)

	atomic.AddInt32(&m.State.metaInfoCounter, 1)

	if err != nil {
		m.mux.Lock()
		m.State.MetaInfo.StreamTitle = ""
		m.mux.Unlock()
		return
	}
	m.mux.Lock()
	m.State.MetaInfo.StreamTitle = string(result[:])
	m.mux.Unlock()
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func (m *Mount) getMountsInfo() MountInfo {
	var t MountInfo
	t.Listeners = atomic.LoadInt32(&m.State.Listeners)
	m.mux.Lock()
	t.Name = m.Name
	if m.State.Started {
		t.UpTime = fmtDuration(time.Since(m.State.StartedTime))
	}
	m.mux.Unlock()
	t.Buff = m.buffer.Info()
	return t
}

// icy style metadata
func (m *Mount) getIcyMeta() ([]byte, int, int32) {
	var metaSize byte
	var metaSizeByte int
	var result string

	m.mux.Lock()
	if m.State.MetaInfo.StreamTitle > "" {
		result = "StreamTitle='" + m.State.MetaInfo.StreamTitle + "';"
	} else {
		result = "StreamTitle='" + m.Description + "';"
	}
	m.mux.Unlock()

	metaSize = byte(math.Ceil(float64(len(result)) / 16.0))
	metaSizeByte = int(metaSize)*16 + 1
	meta := make([]byte, metaSizeByte)
	meta[0] = metaSize

	for idx := 0; idx < len(result); idx++ {
		meta[idx+1] = result[idx]
	}

	return meta, metaSizeByte, atomic.LoadInt32(&m.State.metaInfoCounter)
}
