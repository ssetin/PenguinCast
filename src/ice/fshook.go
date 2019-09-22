package ice

import (
	"io/ioutil"
	"net/http"
	"strings"
)

//======================================================
func go404(basePath string, w http.ResponseWriter) {
	f, err := ioutil.ReadFile(basePath + "404.html")
	if err == nil {
		_, _ = w.Write(f)
	}
}

//======================================================

type hookedResponseWriter struct {
	http.ResponseWriter
	ignore   bool
	basePath string
}

func (hrw *hookedResponseWriter) WriteHeader(status int) {
	hrw.Header().Set("Content-Type", "text/html")
	hrw.ResponseWriter.WriteHeader(status)
	if status == http.StatusNotFound {
		hrw.ignore = true
		go404(hrw.basePath, hrw)
	}
}

func (hrw *hookedResponseWriter) Write(p []byte) (int, error) {
	if hrw.ignore {
		return len(p), nil
	}
	return hrw.ResponseWriter.Write(p)
}

type fsHook struct {
	h        http.Handler
	basePath string
}

func (fs fsHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/") {
		go404(fs.basePath, w)
		return
	}
	fs.h.ServeHTTP(&hookedResponseWriter{ResponseWriter: w, basePath: fs.basePath}, r)
}

func NewFsHook(basePath string) *fsHook {
	return &fsHook{
		h:        http.FileServer(http.Dir(basePath)),
		basePath: basePath,
	}
}
