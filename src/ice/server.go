// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssetin/PenguinCast/src/log"
	"github.com/ssetin/PenguinCast/src/pool"

	"github.com/gorilla/mux"
	"github.com/ssetin/PenguinCast/src/stat"
)

const (
	cServerName = "PenguinCast"
	cVersion    = "0.3.0"
)

type Server struct {
	serverName string
	version    string

	Options options

	mux            sync.Mutex
	Started        int32
	StartedTime    time.Time
	ListenersCount int32
	SourcesCount   int32

	statReader stat.ProcStatsReader
	// for monitor
	cpuUsage float64
	memUsage int

	srv         *http.Server
	poolManager PoolManager
	logger      Logger
}

// Init - Load params from config.yaml
func NewServer() (*Server, error) {
	srv := &Server{
		serverName:  cServerName,
		version:     cVersion,
		poolManager: pool.NewPoolManager(),
	}

	err := srv.Options.Load()
	if err != nil {
		return nil, err
	}
	srv.logger, err = log.NewLogger(srv.Options.Logging.LogLevel, srv.Options.Paths.Log)
	if err != nil {
		return nil, err
	}
	err = srv.initMounts()
	if err != nil {
		return nil, err
	}

	srv.logger.Log("%s %s", srv.serverName, srv.version)

	srv.srv = &http.Server{
		Addr:    ":" + strconv.Itoa(srv.Options.Socket.Port),
		Handler: srv.configureRouter(),
	}

	if srv.Options.Logging.UseStat {
		srv.statReader.Init()
		go srv.processStats()
	}

	return srv, nil
}

func (i *Server) configureRouter() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	for _, mnt := range i.Options.Mounts {
		r.HandleFunc("/"+mnt.Name, mnt.write).Methods("SOURCE", "PUT")
		r.HandleFunc("/"+mnt.Name, mnt.read).Methods("GET")
		// TODO: tutu
		r.HandleFunc("/mode=updinfo&mount="+mnt.Name, mnt.metaDataHandler).Methods("GET")
	}

	r.HandleFunc("/info", i.infoHandler).Methods("GET")
	r.HandleFunc("/info.json", i.jsonHandler).Methods("GET")
	r.HandleFunc("/monitor", i.monitorHandler).Methods("GET")
	if i.Options.Logging.UseMonitor {
		r.HandleFunc("/updateMonitor", i.updateMonitorHandler)
	}

	r.PathPrefix("/").Handler(NewFsHook(i.Options.Paths.Web))

	return r
}

func (i *Server) initMounts() error {
	var err error
	for idx := range i.Options.Mounts {
		err = i.Options.Mounts[idx].Init(i, i.logger, i.poolManager)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Server) incListeners() {
	atomic.AddInt32(&i.ListenersCount, 1)
}

func (i *Server) decListeners() {
	if atomic.LoadInt32(&i.ListenersCount) > 0 {
		atomic.AddInt32(&i.ListenersCount, -1)
	}
}

func (i *Server) checkListeners() bool {
	clientsLimit := atomic.LoadInt32(&i.Options.Limits.Clients)
	if atomic.LoadInt32(&i.ListenersCount) > clientsLimit {
		return false
	}
	return true
}

func (i *Server) incSources() {
	atomic.AddInt32(&i.SourcesCount, 1)
}

func (i *Server) decSources() {
	if atomic.LoadInt32(&i.SourcesCount) > 0 {
		atomic.AddInt32(&i.SourcesCount, -1)
	}
}

func (i *Server) checkSources() bool {
	sourcesLimit := atomic.LoadInt32(&i.Options.Limits.Sources)
	if atomic.LoadInt32(&i.SourcesCount) > sourcesLimit {
		return false
	}
	return true
}

// Close - finish
func (i *Server) Close() {
	if err := i.srv.Shutdown(context.Background()); err != nil {
		i.logger.Error(err.Error())
		i.logger.Log("Error: %s\n", err.Error())
	} else {
		i.logger.Log("Stopped")
	}

	for idx := range i.Options.Mounts {
		i.Options.Mounts[idx].Close()
	}

	i.statReader.Close()
	i.logger.Close()
}

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *Server) writeAccessLog(host string, startTime time.Time, request string, bytesSend int, refer, userAgent string, seconds int) {
	i.logger.Access("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", host, startTime.Format(time.RFC1123Z), request, "200", bytesSend, refer, userAgent, seconds)
}

func (i *Server) getHost(addr string) string {
	idx := strings.Index(addr, ":")
	if idx == -1 {
		return addr
	}
	return addr[:idx]
}

func (i *Server) logHeaders(r *http.Request) {
	request := r.Method + " " + r.RequestURI + " " + r.Proto + "\n"
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.logger.Error("\n" + request)
}

/*Start - start listening port ...*/
func (i *Server) Start() {
	if atomic.LoadInt32(&i.Started) == 1 {
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)

	go func() {
		i.mux.Lock()
		i.StartedTime = time.Now()
		i.mux.Unlock()
		atomic.StoreInt32(&i.Started, 1)
		i.logger.Log("Started on %s,", i.srv.Addr)

		if err := i.srv.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-stop
	atomic.StoreInt32(&i.Started, 0)
}
