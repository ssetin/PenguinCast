// Package iceserver - icecast streaming server
package iceserver

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

//MonitorInfo ...
type MonitorInfo struct {
	Mounts   []MountInfo
	CPUUsage float64
	MemUsage int
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
		CPU, Memory, err := i.statReader.GetCPUAndMem()
		if err != nil {
			i.printError(1, err.Error())
			ticker.Stop()
			break
		}
		i.mux.Lock()
		fmt.Fprintf(i.statFile, time.Now().Format("2006-01-02 15:04:05")+"\t%d\t%f\t%d\n", i.ListenersCount, CPU, Memory/1024)
		i.cpuUsage = math.Floor(CPU*100) / 100
		i.memUsage = Memory / 1024
		i.mux.Unlock()
		<-ticker.C
	}
}

func (i *IceServer) sendMonitorInfo(client *websocket.Conn) {
	ticker := time.NewTicker(6 * time.Second)
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
		i.mux.Lock()
		monitorInfo.CPUUsage = i.cpuUsage
		monitorInfo.MemUsage = i.memUsage
		i.mux.Unlock()

		msg, _ := json.Marshal(monitorInfo)
		w.Write(msg)
		w.Close()

		<-ticker.C
	}
}
