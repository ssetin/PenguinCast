// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"path"
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

func (i *Server) metaDataHandler(w http.ResponseWriter, r *http.Request) {

	i.logger.Log("meta %s", r.URL.RawQuery)

	mountName := path.Base(r.URL.Query().Get("mount"))
	i.logger.Debug("runCommand 0 with %s", mountName)
	mIdx := 0
	if mIdx >= 0 {
		i.Options.Mounts[mIdx].updateMeta(w, r)
	}

}
