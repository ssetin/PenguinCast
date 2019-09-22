// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"html/template"
	"io/ioutil"
	"net/http"
)

func (i *Server) internalHandler(w http.ResponseWriter, r *http.Request) {
	f, _ := ioutil.ReadFile(i.Options.Paths.Web + "500.html")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(f)
}

func (i *Server) infoHandler(w http.ResponseWriter, r *http.Request) {
	i.renderPage(w, r, "templates/info.gohtml")
}

func (i *Server) jsonHandler(w http.ResponseWriter, r *http.Request) {
	i.renderPage(w, r, "templates/json.gohtml")
}

func (i *Server) monitorHandler(w http.ResponseWriter, r *http.Request) {
	i.renderPage(w, r, "templates/monitor.gohtml")
}

func (i *Server) updateMonitorHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	go i.sendMonitorInfo(i.Options.Logging.MonitorInterval, ws)
}

func (i *Server) renderPage(w http.ResponseWriter, r *http.Request, tplName string) {
	t, err := template.ParseFiles(tplName)
	if err != nil {
		i.logger.Error(err.Error())
		i.internalHandler(w, r)
		return
	}
	err = t.Execute(w, i)
	if err != nil {
		i.logger.Error(err.Error())
		i.internalHandler(w, r)
		return
	}
}
