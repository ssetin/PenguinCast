// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (i *Server) setNotFound(w http.ResponseWriter, r *http.Request) {
	f, _ := i.loadPage(i.Options.Paths.Web + "404.html")
	w.WriteHeader(404)
	w.Write(f)
}

func (i *Server) setInternal(w http.ResponseWriter, r *http.Request) {
	f, _ := i.loadPage(i.Options.Paths.Web + "500.html")
	w.WriteHeader(500)
	w.Write(f)
}

func (i *Server) renderPage(w http.ResponseWriter, r *http.Request, tplName string) {
	t, err := template.ParseFiles(tplName)
	if err != nil {
		i.logger.Error(err.Error())
		i.setInternal(w, r)
		return
	}
	err = t.Execute(w, i)
	if err != nil {
		i.logger.Error(err.Error())
		i.setInternal(w, r)
		return
	}
}

func (i *Server) loadPage(filename string) ([]byte, error) {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		i.logger.Error(err.Error())
		return nil, err
	}
	return body, nil
}

/*
	checkPage - return request object
	filename, mount index, command index or error
*/
func (i *Server) checkPage(w http.ResponseWriter, r *http.Request) (string, int, int, error) {
	docName := path.Base(r.URL.Path)

	mountIdx := i.checkIsMount(docName)
	if mountIdx >= 0 {
		return "", mountIdx, -1, nil
	}

	cmdIdx := i.checkIsCommand(docName, r)
	if cmdIdx >= 0 {
		return "", -1, cmdIdx, nil
	}

	filename := filepath.Join(i.Options.Paths.Web, filepath.Clean(r.URL.Path))
	i.logger.Debug("checkPage filename=%s", filename)

	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			i.logger.Error(err.Error())
			i.setNotFound(w, r)
			return "", -1, -1, err
		}
	}

	if info.IsDir() {
		http.Redirect(w, r, "/info.html", 301)
		return "", -1, -1, errors.New("Redirected to root from " + filename)
	}

	return filename, -1, -1, nil
}
