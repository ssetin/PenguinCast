package iceserver

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	cServerName = "PenguinCast"
	cVersion    = "0.01f"
)

// IceServer ...
type IceServer struct {
	serverName string
	version    string

	Props Properties

	logError      *log.Logger
	logErrorFile  *os.File
	logAccess     *log.Logger
	logAccessFile *os.File
}

// Init - Load params from config.json
func (i *IceServer) Init() error {
	var err error

	i.serverName = cServerName
	i.version = cVersion

	log.Println("Starting " + i.serverName + " " + i.version)

	err = i.initConfig()
	if err != nil {
		return err
	}
	err = i.initLog()
	if err != nil {
		return err
	}
	err = i.initMounts()
	if err != nil {
		return err
	}
	return nil
}

func (i *IceServer) initMounts() error {
	var err error
	for idx := range i.Props.Mounts {
		err = i.Props.Mounts[idx].Init(i)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close - finish
func (i *IceServer) Close() {
	log.Print("Stopping " + i.serverName + "...")
	i.logErrorFile.Close()
	i.logAccessFile.Close()
	for idx := range i.Props.Mounts {
		i.Props.Mounts[idx].Close()
	}
	log.Println("Ok!")
}

func (i *IceServer) checkIsMount(page string) int {
	for idx := range i.Props.Mounts {
		if filepath.Clean(i.Props.Paths.Web+i.Props.Mounts[idx].Name) == page {
			return idx
		}
	}
	return -1
}

func (i *IceServer) handler(w http.ResponseWriter, r *http.Request) {
	if i.Props.Logging.Loglevel == 4 {
		i.logHeaders(w, r)
	}

	page, mountidx, err := i.checkPage(w, r)
	if err != nil {
		i.printError(1, err.Error())
		return
	}

	if mountidx >= 0 {
		i.openMount(mountidx, w, r)
		return
	}

	if strings.HasSuffix(page, "info.html") || strings.HasSuffix(page, "info.json") {
		i.renderMounts(w, r, page)
	} else {
		http.ServeFile(w, r, page)
	}
}

/*Start - start listening port ...*/
func (i *IceServer) Start() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	http.HandleFunc("/", i.handler)

	go func() {
		log.Fatal(http.ListenAndServe(":"+strconv.Itoa(i.Props.Socket.Port), nil))
	}()

	<-stop
}
