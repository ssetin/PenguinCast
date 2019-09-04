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

	"github.com/ssetin/PenguinCast/src/config"

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
	Buff      BufferInfo
}

// Mount ...
type Mount struct {
	Options     config.MountOptions
	ContentType string
	StreamURL   string

	State struct {
		Started     bool
		StartedTime time.Time
		MetaInfo    metaData
		Listeners   int32
	}

	mux      sync.Mutex
	buffer   BufferQueue
	server   *Server
	dumpFile *os.File
}

//Init ...
func (m *Mount) Init(srv *Server, opt config.MountOptions) error {
	m.server = srv
	m.Options = opt
	m.Clear()
	m.State.MetaInfo.MetaInt = opt.BitRate * 1024 / 8 * 10

	if opt.DumpFile > "" {
		var err error
		m.dumpFile, err = os.OpenFile(opt.DumpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}

	pool := m.server.poolManager.Init(opt.BitRate * 1024 / 8)
	m.buffer.Init(opt.BurstSize/(opt.BitRate*1024/8)+2, pool)

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
	m.StreamURL = "http://" + m.server.Options.Host + ":" + strconv.Itoa(m.server.Options.Socket.Port) + "/" + m.Options.Name
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

	if m.Options.Password != pair[1] && m.Options.User != pair[0] {
		http.Error(w, "Not authorized", 401)
		return errors.New("wrong user or password")
	}

	m.saySourceHello(w, r)

	return nil
}

func (m *Mount) getParams(paramStr string) map[string]string {
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

func (m *Mount) sayListenerHello(w *bufio.ReadWriter, icyMeta bool) {
	w.WriteString("HTTP/1.0 200 OK\r\n")
	w.WriteString("Server: ")
	w.WriteString(m.server.serverName)
	w.WriteString(" ")
	w.WriteString(m.server.version)
	w.WriteString("\r\nContent-Type: ")
	w.WriteString(m.ContentType)
	w.WriteString("\r\nConnection: Keep-Alive\r\n")
	w.WriteString("X-Audiocast-Bitrate: ")
	w.WriteString(strconv.Itoa(m.Options.BitRate))
	w.WriteString("\r\nX-Audiocast-Name: ")
	w.WriteString(m.Options.Name)
	w.WriteString("\r\nX-Audiocast-Genre: ")
	w.WriteString(m.Options.Genre)
	w.WriteString("\r\nX-Audiocast-Url: ")
	w.WriteString(m.StreamURL)
	w.WriteString("\r\nX-Audiocast-Public: 0\r\n")
	w.WriteString("X-Audiocast-Description: ")
	w.WriteString(m.Options.Description)
	w.WriteString("\r\n")
	if icyMeta {
		w.WriteString("Icy-Metaint: ")
		w.WriteString(strconv.Itoa(m.State.MetaInfo.MetaInt))
		w.WriteString("\r\n")
	}
	w.WriteString("\r\n")
	w.Flush()
}

func (m *Mount) saySourceHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", m.server.serverName+"/"+m.server.version)
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
		m.Options.BitRate = 0
	} else {
		m.Options.BitRate = bRate
	}

	m.Options.Genre = r.Header.Get("ice-genre")
	m.ContentType = r.Header.Get("content-type")
	m.Options.Description = r.Header.Get("ice-description")
}

func (m *Mount) updateMeta(w http.ResponseWriter, r *http.Request) {
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
		mStr += "StreamTitle='" + m.Options.Description + "';"
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

func (m *Mount) getMountsInfo() mountInfo {
	var t mountInfo
	t.Listeners = atomic.LoadInt32(&m.State.Listeners)
	m.mux.Lock()
	t.Name = m.Options.Name
	if m.State.Started {
		t.UpTime = fmtDuration(time.Since(m.State.StartedTime))
		t.Buff = m.buffer.Info()
	}
	m.mux.Unlock()
	return t
}

// icy style metadata
func (m *Mount) getIcyMeta() ([]byte, int) {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.State.MetaInfo.meta, m.State.MetaInfo.metaSizeByte
}
