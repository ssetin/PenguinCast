package icyserver

import (
	"encoding/base64"
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

//Mount*******************************************************************************************

func (m *Mount) writeBuffer(data []byte, len int) error {
	copy(m.buffer, data)
	return nil
}

func (m *Mount) readBuffer(data []byte, len int) ([]byte, error) {
	return m.buffer, nil
}

func (m *Mount) auth(w http.ResponseWriter, r *http.Request) error {
	strAuth := r.Header.Get("authorization")

	s := strings.SplitN(strAuth, " ", 2)

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return err
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		http.Error(w, "Not authorized", 401)
		return err
	}

	if m.Password != pair[1] && m.User != pair[0] {
		http.Error(w, "Not authorized", 401)
		return err
	}

	w.Header().Set("Server", m.Server.serverName+"/"+m.Server.version)
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Allow", "GET, SOURCE")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	flusher.Flush()

	return nil
}

func (m *Mount) writeICEHeaders(w http.ResponseWriter, r *http.Request) {
	brate, err := strconv.Atoi(r.Header.Get("ice-bitrate"))
	if err != nil {
		m.BitRate = 0
	} else {
		m.BitRate = brate
	}
	m.Genre = r.Header.Get("ice-genre")
	m.ContentType = r.Header.Get("content-type")
	m.StreamURL = r.Header.Get("ice-url")
	m.Description = r.Header.Get("ice-description")
}

//IcyServer*******************************************************************************************

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *IcyServer) closeMount(idx int, issource bool, bytessended *int, start time.Time, r *http.Request) {
	if issource {
		i.Props.Mounts[idx].Clear()
	} else {
		i.Props.Mounts[idx].Status.Listeners--
	}
	t := time.Now()
	elapsed := t.Sub(start)
	i.printAccess("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", getHost(r.RemoteAddr), start.Format(time.RFC1123Z), r.Method+" "+r.RequestURI+" "+r.Proto, "200", *bytessended, r.Referer(), r.UserAgent(), int(elapsed.Seconds()))
}

func (i *IcyServer) logHeaders(w http.ResponseWriter, r *http.Request) {
	request := r.Method + " " + r.RequestURI + " " + r.Proto + "\n"
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.printError(2, "\n"+request)
}

func (i *IcyServer) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	// Temporary
	i.logHeaders(w, r)

	if r.Method == "SOURCE" {
		i.writeMount(idx, w, r)
	} else {
		i.readMount(idx, w, r)
	}

}

func (i *IcyServer) readMount(idx int, w http.ResponseWriter, r *http.Request) {
	bytessended := 0
	readbuffer := i.Props.Mounts[idx].BufferSize
	start := time.Now()

	i.printError(2, "readMount "+i.Props.Mounts[idx].Name)
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
	//defer r.Body.Close()

	for pos := 0; pos <= i.Props.Mounts[idx].BufferSize-readbuffer; pos += readbuffer {
		buffer := i.Props.Mounts[idx].buffer[pos : pos+readbuffer]
		s, err := w.Write(buffer)

		if err != nil {
			i.printError(1, err.Error())
			break
		}
		bytessended += s

		//flusher, _ := w.(http.Flusher)
		//flusher.Flush()
		time.Sleep(1 * time.Second)
	}
	i.printError(2, "readMount bytessended="+strconv.Itoa(bytessended))
}

func (i *IcyServer) writeMount(idx int, w http.ResponseWriter, r *http.Request) {
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
	start := time.Now()

	i.printError(2, "writeMount "+i.Props.Mounts[idx].Name)
	defer i.closeMount(idx, true, &bytessended, start, r)
	//defer r.Body.Close()

	for {
		readed, _ := r.Body.Read(i.Props.Mounts[idx].buffer)

		bytessended += readed
		//time.Sleep(1 * time.Second)
		//flusher, _ := w.(http.Flusher)
		//flusher.Flush()
	}
	i.printError(2, "writeMount bytessended="+strconv.Itoa(bytessended))
}
