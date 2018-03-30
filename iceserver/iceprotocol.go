package iceserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	strICYOk    = "ICY 200 OK\r\n"
	strICYOk2   = "OK2\r\n"
	strICYCaps  = "icy-caps:11\r\n\r\n"
	strICYNote2 = "icy-notice2:%s/%s\r\n"
	strICYName  = "icy-name:%s\r\n"
	strICYPub   = "icy-pub:0\r\n"
	strICYMeta  = "icy-metaint:%d\r\n"
	strICYEol   = "\r\n"
)

func getHost(addr string) string {
	i := strings.Index(addr, ":")
	if i == -1 {
		return addr
	}
	return addr[:i]
}

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *IceServer) closeMount(idx int, issource bool, bytessended *int, start time.Time, r *http.Request) {
	if issource {
		i.Props.Mounts[idx].Clear()
	} else {
		i.Props.Mounts[idx].decListeners()
	}
	t := time.Now()
	elapsed := t.Sub(start)
	i.printAccess("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", getHost(r.RemoteAddr), start.Format(time.RFC1123Z), r.Method+" "+r.RequestURI+" "+r.Proto, "200", *bytessended, r.Referer(), r.UserAgent(), int(elapsed.Seconds()))
}

func (i *IceServer) logHeaders(w http.ResponseWriter, r *http.Request) {
	request := r.Method + " " + r.RequestURI + " " + r.Proto + "\n"
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.printError(4, "\n"+request)
}

func (i *IceServer) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	if i.Props.Logging.Loglevel == 4 {
		i.logHeaders(w, r)
	}

	if r.Method == "SOURCE" {
		i.writeMount(idx, w, r)
	} else {
		i.readMount(idx, w, r)
	}

}

func (i *IceServer) readMount(idx int, w http.ResponseWriter, r *http.Request) {
	bytessended := 0
	writed := 0
	var err error
	start := time.Now()

	i.printError(3, "readMount "+i.Props.Mounts[idx].Name)
	i.Props.Mounts[idx].Status.Listeners++
	defer i.closeMount(idx, false, &bytessended, start, r)

	w.Header().Set("Server", i.serverName+" "+i.version)
	w.Header().Set("Content-Type", i.Props.Mounts[idx].ContentType)
	w.Header().Set("x-audiocast-name", i.Props.Mounts[idx].Name)
	w.Header().Set("x-audiocast-genre", i.Props.Mounts[idx].Genre)
	w.Header().Set("x-audiocast-url", i.Props.Mounts[idx].StreamURL)
	w.Header().Set("x-audiocast-public", "0")
	w.Header().Set("x-audiocast-bitrate", strconv.Itoa(i.Props.Mounts[idx].BitRate))
	w.Header().Set("x-audiocast-description", i.Props.Mounts[idx].Description)

	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	for {
		writed, err = w.Write(i.Props.Mounts[idx].buffer)

		if err != nil {
			i.printError(1, err.Error())
			break
		}
		bytessended += writed

		flusher, _ := w.(http.Flusher)
		flusher.Flush()
		time.Sleep(1 * time.Second)
	}
}

func (i *IceServer) writeMount(idx int, w http.ResponseWriter, r *http.Request) {
	if i.Props.Mounts[idx].Status.Status != "On air" {
		err := i.Props.Mounts[idx].auth(w, r)
		if err != nil {
			i.printError(1, err.Error())
			return
		}
		i.Props.Mounts[idx].writeICEHeaders(w, r)
		i.Props.Mounts[idx].Status.Status = "On air"
		i.Props.Mounts[idx].Status.Started = time.Now().Format(time.RFC1123Z)
	}

	bytessended := 0
	readed := 0
	var err error
	start := time.Now()

	i.printError(3, "writeMount "+i.Props.Mounts[idx].Name)
	defer i.closeMount(idx, true, &bytessended, start, r)

	hj, ok := w.(http.Hijacker)
	if !ok {
		i.printError(1, "webserver doesn't support hijacking")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	for {
		readed, err = bufrw.Read(i.Props.Mounts[idx].buffer)
		if err != nil {
			i.printError(3, err.Error())
		}

		bytessended += readed

		i.printError(3, "writeMount readed="+strconv.Itoa(readed))
		time.Sleep(1 * time.Second)
	}
}
