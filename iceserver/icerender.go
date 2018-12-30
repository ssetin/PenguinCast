package iceserver

import (
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
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
	err = t.Execute(w, &i)
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

/*
	checkPage - return request object
	filename, mount index, command index or error
*/
func (i *IceServer) checkPage(w http.ResponseWriter, r *http.Request) (string, int, int, error) {
	docname := path.Base(r.URL.Path)

	mountidx := i.checkIsMount(docname)
	if mountidx >= 0 {
		return "", mountidx, -1, nil
	}

	cmdidx := i.checkIsCommand(docname, r)
	if cmdidx >= 0 {
		return "", -1, cmdidx, nil
	}

	filename := filepath.Join(i.Props.Paths.Web, filepath.Clean(r.URL.Path))
	i.printError(4, "checkPage filename="+filename)

	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			i.printError(1, err.Error())
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
