// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

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

type metaData struct {
	MetaInt      int
	StreamTitle  string
	meta         []byte
	metaSizeByte int
}

type mountInfo struct {
	Name      string
	Listeners int32
	UpTime    string
	Buff      bufferInfo
}

type mount struct {
	Name         string `yaml:"Name"`
	User         string `yaml:"User"`
	Password     string `yaml:"Password"`
	Description  string `yaml:"Description"`
	BitRate      int    `yaml:"BitRate"`
	Genre        string `yaml:"Genre"`
	BurstSize    int    `yaml:"BurstSize"`
	DumpFile     string `yaml:"DumpFile"`
	MaxListeners int    `yaml:"MaxListeners"`

	ContentType string
	StreamURL   string

	defaultStreamUrl string
	serverName       string
	serverVersion    string

	State struct {
		Started     bool
		StartedTime time.Time
		MetaInfo    metaData
		Listeners   int32
	}

	mux      sync.Mutex
	buffer   bufferQueue
	dumpFile *os.File
}

//Init ...
func (m *mount) Init(serverName, serverVersion, host string, port int, poolManager PoolManager) error {
	m.defaultStreamUrl = fmt.Sprintf("http://%s:%d/%s", host, port, m.Name)
	m.Clear()
	m.State.MetaInfo.MetaInt = m.BitRate * 1024 / 8 * 10
	m.serverName = serverName
	m.serverVersion = serverVersion

	if m.DumpFile > "" {
		var err error
		m.dumpFile, err = os.OpenFile(m.DumpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}

	p := poolManager.Init(m.BitRate * 1024 / 8)
	m.buffer.Init(m.BurstSize/(m.BitRate*1024/8)+2, p)
	return nil
}

//Close ...
func (m *mount) Close() {
	if m.dumpFile != nil {
		_ = m.dumpFile.Close()
	}
}

//Clear ...
func (m *mount) Clear() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.State.Started = false
	m.State.StartedTime = time.Time{}
	m.zeroListeners()
	m.State.MetaInfo.StreamTitle = ""
	m.StreamURL = m.defaultStreamUrl
}

func (m *mount) incListeners() {
	atomic.AddInt32(&m.State.Listeners, 1)
}

func (m *mount) decListeners() {
	if atomic.LoadInt32(&m.State.Listeners) > 0 {
		atomic.AddInt32(&m.State.Listeners, -1)
	}
}

func (m *mount) zeroListeners() {
	atomic.StoreInt32(&m.State.Listeners, 0)
}

func (m *mount) auth(w http.ResponseWriter, r *http.Request) error {
	strAuth := r.Header.Get("authorization")

	if strAuth == "" {
		m.saySourceHello(w, r)
		return errors.New("no authorization field")
	}

	s := strings.SplitN(strAuth, " ", 2)
	if len(s) != 2 {
		http.Error(w, "Not authorized", 401)
		return errors.New("not authorized")
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return err
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		http.Error(w, "Not authorized", 401)
		return errors.New("not authorized")
	}

	if m.Password != pair[1] && m.User != pair[0] {
		http.Error(w, "Not authorized", 401)
		return errors.New("wrong user or password")
	}

	m.saySourceHello(w, r)

	return nil
}

func (m *mount) getParams(paramStr string) map[string]string {
	var rex = regexp.MustCompile("(\\w+)=(\\w+)")
	data := rex.FindAllStringSubmatch(paramStr, -1)
	params := make(map[string]string)
	for _, kv := range data {
		k := kv[1]
		v := kv[2]
		params[k] = v
	}
	return params
}

func (m *mount) sayHello(w *bufio.ReadWriter, icyMeta bool) {
	_, _ = w.WriteString("HTTP/1.0 200 OK\r\n")
	_, _ = w.WriteString("Server: ")
	_, _ = w.WriteString(m.serverName)
	_, _ = w.WriteString(" ")
	_, _ = w.WriteString(m.serverVersion)
	_, _ = w.WriteString("\r\n")
	_, _ = w.WriteString("Content-Type: ")
	_, _ = w.WriteString(m.ContentType)
	_, _ = w.WriteString("\r\nConnection: Keep-Alive\r\n")
	_, _ = w.WriteString("X-Audiocast-Bitrate: ")
	_, _ = w.WriteString(strconv.Itoa(m.BitRate))
	_, _ = w.WriteString("\r\nX-Audiocast-Name: ")
	_, _ = w.WriteString(m.Name)
	_, _ = w.WriteString("\r\nX-Audiocast-Genre: ")
	_, _ = w.WriteString(m.Genre)
	_, _ = w.WriteString("\r\nX-Audiocast-Url: ")
	_, _ = w.WriteString(m.StreamURL)
	_, _ = w.WriteString("\r\nX-Audiocast-Public: 0\r\n")
	_, _ = w.WriteString("X-Audiocast-Description: ")
	_, _ = w.WriteString(m.Description)
	_, _ = w.WriteString("\r\n")
	if icyMeta {
		_, _ = w.WriteString("Icy-Metaint: ")
		_, _ = w.WriteString(strconv.Itoa(m.State.MetaInfo.MetaInt))
		_, _ = w.WriteString("\r\n")
	}
	_, _ = w.WriteString("\r\n")
	w.Flush()
}

func (m *mount) saySourceHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", m.serverName+"/"+m.serverVersion)
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

func (m *mount) writeICEHeaders(w http.ResponseWriter, r *http.Request) {
	var bitRateStr string
	bitRateStr = r.Header.Get("ice-bitrate")
	if bitRateStr == "" {
		audioInfo := r.Header.Get("ice-audio-info")
		if len(audioInfo) > 3 {
			params := m.getParams(audioInfo)
			bitRateStr = params["bitrate"]
		}
	}

	bRate, err := strconv.Atoi(bitRateStr)
	if err != nil {
		m.BitRate = 0
	} else {
		m.BitRate = bRate
	}

	m.Genre = r.Header.Get("ice-genre")
	m.ContentType = r.Header.Get("content-type")
	m.Description = r.Header.Get("ice-description")
}

func (m *mount) updateMeta(w http.ResponseWriter, r *http.Request) {
	err := m.auth(w, r)
	if err != nil {
		return
	}

	var metaSize byte
	var mStr string
	song := r.URL.Query().Get("song")
	songReader := strings.NewReader(song)
	enc, _, _ := charset.DetermineEncoding(([]byte)(song), "")
	utf8Reader := transform.NewReader(songReader, enc.NewDecoder())
	result, err := ioutil.ReadAll(utf8Reader)

	if err != nil {
		m.mux.Lock()
		m.State.MetaInfo.StreamTitle = ""
		m.mux.Unlock()
		return
	}

	m.mux.Lock()
	m.State.MetaInfo.StreamTitle = string(result[:])

	if m.State.MetaInfo.StreamTitle > "" {
		mStr = "StreamTitle='" + m.State.MetaInfo.StreamTitle + "';"
	} else {
		mStr += "StreamTitle='" + m.Description + "';"
	}

	metaSize = byte(math.Ceil(float64(len(mStr)) / 16.0))
	m.State.MetaInfo.metaSizeByte = int(metaSize)*16 + 1
	m.State.MetaInfo.meta = make([]byte, m.State.MetaInfo.metaSizeByte)
	m.State.MetaInfo.meta[0] = metaSize

	for idx := 0; idx < len(mStr); idx++ {
		m.State.MetaInfo.meta[idx+1] = mStr[idx]
	}
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

func (m *mount) getMountsInfo() mountInfo {
	var t mountInfo
	t.Listeners = atomic.LoadInt32(&m.State.Listeners)
	m.mux.Lock()
	t.Name = m.Name
	if m.State.Started {
		t.UpTime = fmtDuration(time.Since(m.State.StartedTime))
		t.Buff = m.buffer.Info()
	}
	m.mux.Unlock()
	return t
}

// icy style metadata
func (m *mount) getIcyMeta() ([]byte, int) {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.State.MetaInfo.meta, m.State.MetaInfo.metaSizeByte
}
