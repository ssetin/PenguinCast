package iceserver

import (
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func (i *IceServer) setNotFound(w http.ResponseWriter, r *http.Request) {
	f, _ := i.loadPage(i.Props.Paths.Web + "404.html")
	w.WriteHeader(404)
	w.Write(f)
}

func (i *IceServer) setInternal(w http.ResponseWriter, r *http.Request) {
	f, _ := i.loadPage(i.Props.Paths.Web + "500.html")
	w.WriteHeader(500)
	w.Write(f)
}

func (i *IceServer) renderMounts(w http.ResponseWriter, r *http.Request, tplname string) {
	t, err := template.ParseFiles(tplname)
	if err != nil {
		i.printError(1, err.Error())
		i.setInternal(w, r)
		return
	}
	err = t.Execute(w, &i.Props.Mounts)
	if err != nil {
		i.printError(1, err.Error())
		i.setInternal(w, r)
		return
	}
}

func (i *IceServer) loadPage(filename string) ([]byte, error) {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		i.printError(1, err.Error())
		return nil, err
	}
	return body, nil
}

func (i *IceServer) checkPage(w http.ResponseWriter, r *http.Request) (string, int, error) {
	filename := filepath.Join(i.Props.Paths.Web, filepath.Clean(r.URL.Path))
	mountidx := i.checkIsMount(filename)

	i.printError(2, "checkPage filename="+filename)

	if mountidx >= 0 {
		return "", mountidx, nil
	}

	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			i.printError(1, err.Error())
			i.setNotFound(w, r)
			return "", -1, err
		}
	}

	if info.IsDir() {
		http.Redirect(w, r, "/info.html", 301)
		return "", -1, errors.New("Redirected to root from " + filename)
	}

	return filename, -1, nil
}
