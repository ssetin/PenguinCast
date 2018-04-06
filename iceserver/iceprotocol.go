package iceserver

/*
	TODO:
	- meta info
*/

import (
	"fmt"
	"io"
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

/*
	openMount
    Decide what to do, according to HTTP method
*/
func (i *IceServer) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	if i.Props.Logging.Loglevel == 3 {
		i.logHeaders(w, r)
	}

	if r.Method == "SOURCE" {
		i.writeMount(idx, w, r)
	} else {
		i.readMount(idx, w, r)
	}

}

/*
	readMount
    Send stream from requested mount to client
*/
func (i *IceServer) readMount(idx int, w http.ResponseWriter, r *http.Request) {
	var mount *Mount
	bytessended := 0
	writed := 0
	idle := 0
	var err error
	var pack, nextpack *BufElement
	start := time.Now()
	mount = &i.Props.Mounts[idx]

	i.printError(3, "readMount "+mount.Name)
	mount.incListeners()
	defer i.closeMount(idx, false, &bytessended, start, r)

	w.Header().Set("Server", i.serverName+" "+i.version)
	w.Header().Set("Content-Type", mount.ContentType)
	w.Header().Set("x-audiocast-name", mount.Name)
	w.Header().Set("x-audiocast-genre", mount.Genre)
	w.Header().Set("x-audiocast-url", mount.StreamURL)
	w.Header().Set("x-audiocast-public", "0")
	w.Header().Set("x-audiocast-bitrate", strconv.Itoa(mount.BitRate))
	w.Header().Set("x-audiocast-description", mount.Description)
	w.Header().Set("icy-metaint", strconv.Itoa(mount.State.MetaInfo.MetaInt))

	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()
	flusher, _ := w.(http.Flusher)

	pack = mount.buffer.First()

	if pack == nil {
		i.printError(1, "readMount Empty buffer")
		return
	}

	for {
		writed, err = w.Write(pack.buffer)

		if err != nil {
			i.printError(1, err.Error())
			break
		}
		bytessended += writed

		// where we are?
		i.printError(4, "ReadBuffer "+strconv.Itoa(pack.iam)+"/"+strconv.Itoa(mount.buffer.Size()))

		// collect burst data in buffer whithout flashing
		if bytessended > mount.BurstSize {
			flusher.Flush()
			time.Sleep(1000 * time.Millisecond)
		}

		nextpack = pack.Next()
		if nextpack == nil {
			idle++
			if idle >= i.Props.Limits.EmptyBufferIdleTimeOut {
				i.printError(1, "Empty Buffer idle time is reached")
				break
			}
			continue
		} else {
			idle = 0
		}

		pack = nextpack
	}

}

/*
	writeMount
    Authenticate SOURCE and write stream from it to appropriate mount buffer
*/
func (i *IceServer) writeMount(idx int, w http.ResponseWriter, r *http.Request) {
	var mount *Mount
	mount = &i.Props.Mounts[idx]

	mount.mux.Lock()
	if mount.State.Status != "On air" {
		err := mount.auth(w, r)
		if err != nil {
			mount.mux.Unlock()
			i.printError(1, err.Error())
			return
		}
		mount.writeICEHeaders(w, r)
		mount.State.Status = "On air"
		mount.State.Started = time.Now().Format(time.RFC1123Z)
	}
	mount.mux.Unlock()

	bytessended := 0
	idle := 0
	readed := 0
	var err error
	var buff []byte
	start := time.Now()

	i.printError(3, "writeMount "+mount.Name)
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

	// max bytes per second according to bitrate
	buff = make([]byte, mount.BitRate*1024/8)

	for {
		readed, err = bufrw.Read(buff)
		if err != nil {
			if err == io.EOF {
				idle++
				if idle >= i.Props.Limits.SourceIdleTimeOut {
					i.printError(1, "Source idle time is reached")
					break
				}
			}
			i.printError(3, err.Error())
		} else {
			idle = 0
		}
		// append to the buffer's queue based on actual readed bytes
		mount.buffer.Append(buff, readed)
		bytessended += readed
		i.printError(4, "writeMount "+strconv.Itoa(readed)+"")

		if mount.dumpFile != nil {
			mount.dumpFile.Write(mount.buffer.Last().buffer)
		}

		time.Sleep(1000 * time.Millisecond)
	}
}
