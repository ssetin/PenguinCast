// Package iceserver - icecast streaming server
// Copyright 2018 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")
package iceserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/struCoder/pidusage"
)

//MonitorInfo ...
type MonitorInfo struct {
	Mounts []MountInfo
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (i *IceServer) processStats() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		sysInfo, err := pidusage.GetStat(os.Getpid())
		if err != nil {
			i.printError(1, err.Error())
			ticker.Stop()
			break
		}
		i.mux.Lock()
		if i.ListenersCount > 0 {
			fmt.Fprintf(i.statFile, time.Now().Format("2006-01-02 15:04:05")+"\t%d\t%f\t%f\n", i.ListenersCount, sysInfo.CPU, sysInfo.Memory/1024)
		}
		i.mux.Unlock()
		<-ticker.C
	}
}

func (i *IceServer) sendMonitorInfo(client *websocket.Conn) {
	ticker := time.NewTicker(7 * time.Second)
	for {
		w, err := client.NextWriter(websocket.TextMessage)
		if err != nil {
			ticker.Stop()
			break
		}

		monitorInfo := &MonitorInfo{}
		monitorInfo.Mounts = make([]MountInfo, 0, len(i.Props.Mounts))

		for idx := range i.Props.Mounts {
			inf := i.Props.Mounts[idx].getMountsInfo()
			monitorInfo.Mounts = append(monitorInfo.Mounts, inf)
		}

		msg, _ := json.Marshal(monitorInfo)
		w.Write(msg)
		w.Close()

		<-ticker.C
	}
}
