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
	strICYPub   = "icy-pub:1\r\n"
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

	if m.Password != pair[1] {
		http.Error(w, "Not authorized", 401)
		return err
	}

	fmt.Fprint(w, strICYOk2)
	fmt.Fprint(w, strICYCaps)

	fmt.Println("Auth Ok!")

	return nil
}

func (m *Mount) readHeaders(w http.ResponseWriter, r *http.Request) {
	brate, err := strconv.Atoi(r.Header.Get("ice-bitrate"))
	if err != nil {
		m.BitRate = 0
	} else {
		m.BitRate = brate
	}
	m.Genre = r.Header.Get("ice-genre")
	m.ContentType = r.Header.Get("content-type")
	m.Description = r.Header.Get("ice-description")
}

//IcyServer*******************************************************************************************

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *IcyServer) closeMount(idx int, bytessended *int, start time.Time, r *http.Request) {
	i.Props.Mounts[idx].Status.Listeners--
	t := time.Now()
	elapsed := t.Sub(start)
	i.printAccess("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", getHost(r.RemoteAddr), start.Format(time.RFC1123Z), r.Method+" "+r.RequestURI+" "+r.Proto, "200", *bytessended, r.Referer(), r.UserAgent(), int(elapsed.Seconds()))
}

func (i *IcyServer) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	var request string
	// Temporary
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.printError(2, "\n"+request)

	if r.Header.Get("ice-name") > "" {
		i.writeMount(idx, w, r)
	} else {
		i.readMount(idx, w, r)
	}

}

func (i *IcyServer) readMount(idx int, w http.ResponseWriter, r *http.Request) {
	bufferSize := 65536
	bytessended := 0
	start := time.Now()

	i.printError(2, "readMount "+i.Props.Mounts[idx].Name)
	i.Props.Mounts[idx].Status.Listeners++
	defer i.closeMount(idx, &bytessended, start, r)

	fmt.Fprint(w, strICYOk)
	fmt.Fprintf(w, strICYNote2, i.serverName, i.version)
	fmt.Fprintf(w, strICYName, i.Props.Name)
	fmt.Fprint(w, strICYPub)
	fmt.Fprintf(w, strICYMeta, i.Props.Mounts[idx].MetaInt)
	fmt.Fprint(w, strICYEol)

	mp3, err := i.loadPage("D:\\Sergey\\Misic\\the_stranglers_-_golden_brownsnatch-ost.mp3")
	if err != nil {
		i.printError(1, err.Error())
		return
	}

	for pos := 0; pos < len(mp3)-bufferSize; pos += bufferSize {
		buffer := mp3[pos : pos+bufferSize]
		s, err := w.Write(buffer)
		if err != nil {
			i.printError(1, err.Error())
			break
		}
		bytessended += s
		time.Sleep(1 * time.Second)
	}

	/*for {
		//buffer := mp3[pos : pos+bufferSize]
		s, err := w.Write(i.Props.Mounts[idx].buffer)
		if err != nil {
			i.printError(1, err.Error())
			break
		}
		bytessended += s
		time.Sleep(1 * time.Second)
	}*/

}

func (i *IcyServer) writeMount(idx int, w http.ResponseWriter, r *http.Request) {
	err := i.Props.Mounts[idx].auth(w, r)
	if err != nil {
		i.printError(1, err.Error())
		return
	}
	i.Props.Mounts[idx].readHeaders(w, r)
	i.Props.Mounts[idx].Status.Status = "On air"

	bufferSize := 65536
	var buffer []byte
	buffer = make([]byte, bufferSize)
	bytessended := 0
	start := time.Now()

	i.printError(2, "writeMount "+i.Props.Mounts[idx].Name)
	defer i.closeMount(idx, &bytessended, start, r)
	defer r.Body.Close()

	for {
		r.Body.Read(buffer)
		i.Props.Mounts[idx].writeBuffer(buffer, bufferSize)
		time.Sleep(1 * time.Second)
	}

}
