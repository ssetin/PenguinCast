// Package iceserver - icecast streaming server
package iceserver

/*
	TODO:
	- readMount  write frame timeout
*/

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
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
		i.decSources()
		i.Props.Mounts[idx].Clear()
	} else {
		i.Props.Mounts[idx].decListeners()
		i.decListeners()
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
	if r.Method == "SOURCE" || r.Method == "PUT" {
		if !i.checkSources() {
			i.printError(1, "Number of sources exceeded")
			http.Error(w, "Number of sources exceeded", 403)
			return
		}
		i.writeMount(idx, w, r)
	} else {
		if !i.checkListeners() {
			i.printError(1, "Number of listeners exceeded")
			http.Error(w, "Number of listeners exceeded", 403)
			return
		}
		var im = false
		if r.Header.Get("icy-metadata") == "1" {
			im = true
		}
		i.readMount(idx, im, w, r)
	}
}

/*
	readMount
    Send stream from requested mount to client
*/
func (i *IceServer) readMount(idx int, icymeta bool, w http.ResponseWriter, r *http.Request) {
	var mount *Mount
	var metaCounter int32
	var meta []byte
	var err error
	var pack, nextpack *BufElement

	bytessended := 0
	writed := 0
	//idle := 0
	offset := 0
	nometabytes := 0
	partwrited := 0
	metaCounter = -1
	nmtmp := 0
	delta := 0
	metalen := 0
	n := 0

	start := time.Now()
	mount = &i.Props.Mounts[idx]

	i.printError(3, "readMount "+mount.Name)
	defer i.closeMount(idx, false, &bytessended, start, r)
	defer r.Body.Close()

	w.Header().Set("Server", i.serverName+" "+i.version)
	w.Header().Set("Content-Type", mount.ContentType)
	w.Header().Set("X-Audiocast-Bitrate", strconv.Itoa(mount.BitRate))
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("X-Audiocast-Name", mount.Name)
	w.Header().Set("X-Audiocast-Genre", mount.Genre)
	w.Header().Set("X-Audiocast-Url", mount.StreamURL)
	w.Header().Set("X-Audiocast-Public", "0")
	w.Header().Set("X-Audiocast-Description", mount.Description)
	if icymeta {
		w.Header().Set("icy-metaint", strconv.Itoa(mount.State.MetaInfo.MetaInt))
	}

	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	//try to maximize unused buffer pages from begining
	pack = mount.buffer.Start(mount.BurstSize)

	if pack == nil {
		i.printError(1, "readMount Empty buffer")
		return
	}

	i.incListeners()
	mount.incListeners()

	for {
		//check, if server has to be stopped
		if atomic.LoadInt32(&i.Started) == 0 {
			break
		}

		n++
		pack.Lock()
		if icymeta {
			//check, if metainfo changed
			if mount.metaInfoChanged(metaCounter) {
				meta, metalen, metaCounter = mount.getIcyMeta()
			}

			if nometabytes+pack.len+delta > mount.State.MetaInfo.MetaInt {
				offset = mount.State.MetaInfo.MetaInt - nometabytes - delta

				//log.Printf("*** write block with meta ***")
				//log.Printf("   offset = %d - %d(nometabytes) - %d (delta) = %d", mount.State.MetaInfo.MetaInt, nometabytes, delta, offset)

				if offset < 0 || offset >= pack.len {
					i.printError(3, "Bad metainfo offset %d", offset)
					log.Printf("!!! Bad metainfo offset %d ***", offset)
					offset = 0
				}

				partwrited, err = w.Write(pack.buffer[:offset])
				writed += partwrited
				partwrited, err = w.Write(meta)
				writed += partwrited
				partwrited, err = w.Write(pack.buffer[offset:])
				writed += partwrited

				delta = writed - offset - metalen
				nometabytes = 0
				nmtmp = 0

				//log.Printf("   delta = %d(writed) - %d(offset) - %d(metalen) = %d", writed, offset, metalen, delta)
			} else {
				writed = 0
				nmtmp, err = w.Write(pack.buffer)
				nometabytes += nmtmp
			}
		} else {
			writed, err = w.Write(pack.buffer)
		}

		if err != nil {
			i.printError(1, "%d readMount "+err.Error(), n)
			pack.UnLock()
			break
		}

		bytessended += writed + nmtmp

		// collect burst data in buffer whithout flashing
		if bytessended >= mount.BurstSize {
			flusher.Flush()
			//time.Sleep(1000 * time.Millisecond)
		}

		nextpack = pack.Next()
		for nextpack == nil {
			time.Sleep(time.Millisecond * 500)
			nextpack = pack.Next()
		}
		pack.UnLock()
		/*if nextpack == nil {
			idle++
			if idle >= i.Props.Limits.EmptyBufferIdleTimeOut {
				i.printError(1, "Empty Buffer idle time is reached")
				break
			}
			continue
		} else {
			idle = 0
		}*/
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

	if !mount.State.Started {
		mount.mux.Lock()
		err := mount.auth(w, r)
		if err != nil {
			mount.mux.Unlock()
			i.printError(1, err.Error())
			return
		}
		mount.writeICEHeaders(w, r)
		mount.State.Started = true
		mount.State.StartedTime = time.Now()
		mount.mux.Unlock()
	} else {
		i.printError(1, "SOURCE already connected")
		http.Error(w, "SOURCE already connected", 403)
		return
	}

	bytessended := 0
	idle := 0
	readed := 0
	var err error
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

	i.incSources()
	// max bytes per second according to bitrateclear
	buff := make([]byte, mount.BitRate*1024/8)

	for {
		//check, if server has to be stopped
		if atomic.LoadInt32(&i.Started) == 0 {
			break
		}

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

		//check if maxbuffersize reached and truncate it
		mount.buffer.checkAndTruncate()
	}
}
