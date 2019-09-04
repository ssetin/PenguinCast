// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *Server) writeAccessLog(host string, startTime time.Time, request string, bytesSend int, refer, userAgent string, seconds int) {
	i.logger.Access("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", host, startTime.Format(time.RFC1123Z), request, "200", bytesSend, refer, userAgent, seconds)
}

func (i *Server) closeMount(idx int, isSource bool, bytesSend *int, start time.Time, r *http.Request) {
	if isSource {
		i.decSources()
		i.Mounts[idx].Clear()
	} else {
		i.Mounts[idx].decListeners()
		i.decListeners()
	}
	t := time.Now()
	elapsed := t.Sub(start)
	i.writeAccessLog(i.getHost(r.RemoteAddr), start, r.Method+" "+r.RequestURI+" "+r.Proto, *bytesSend, r.Referer(), r.UserAgent(), int(elapsed.Seconds()))
}

func (i *Server) getHost(addr string) string {
	idx := strings.Index(addr, ":")
	if idx == -1 {
		return addr
	}
	return addr[:idx]
}

func (i *Server) logHeaders(w http.ResponseWriter, r *http.Request) {
	request := r.Method + " " + r.RequestURI + " " + r.Proto + "\n"
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.logger.Error(4, "\n"+request)
}

/*
	openMount
    Decide what to do, according to HTTP method
*/
func (i *Server) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	if r.Method == "SOURCE" || r.Method == "PUT" {
		if !i.checkSources() {
			i.logger.Error(1, "Number of sources exceeded")
			http.Error(w, "Number of sources exceeded", 403)
			return
		}
		i.writeMount(idx, w, r)
	} else {
		if !i.checkListeners() {
			i.logger.Error(1, "Number of listeners exceeded")
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

func (i *Server) closeAndUnlock(pack *BufElement, err error) {
	if te, ok := err.(net.Error); ok && te.Timeout() {
		log.Println("Write timeout " + te.Error())
		i.logger.Error(1, "Write timeout")
	} else {
		i.logger.Error(1, err.Error())
	}
	pack.UnLock()
}

/*
	readMount
	Send stream from requested mount to client
*/
func (i *Server) readMount(idx int, icyMeta bool, w http.ResponseWriter, r *http.Request) {
	var mount *Mount
	var meta []byte
	var err error
	var beginIteration time.Time
	var pack, nextpack *BufElement

	bytesSended := 0
	writed := 0
	idle := 0
	idleTimeOut := i.Options.Limits.EmptyBufferIdleTimeOut * 1000
	writeTimeOut := time.Second * time.Duration(i.Options.Limits.WriteTimeOut)
	offset := 0
	nometabytes := 0
	partwrited := 0
	nmtmp := 0
	delta := 0
	metalen := 0
	n := 0

	hj, ok := w.(http.Hijacker)
	if !ok {
		i.logger.Error(1, "webserver doesn't support hijacking")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	start := time.Now()
	mount = &i.Mounts[idx]

	i.logger.Error(3, "readMount "+mount.Options.Name)
	defer i.closeMount(idx, false, &bytesSended, start, r)

	//try to maximize unused buffer pages from beginning
	pack = mount.buffer.Start(mount.Options.BurstSize)

	if pack == nil {
		i.logger.Error(1, "readMount Empty buffer")
		return
	}

	mount.sayListenerHello(bufrw, icyMeta)

	i.incListeners()
	mount.incListeners()

	for {
		beginIteration = time.Now()
		conn.SetWriteDeadline(time.Now().Add(writeTimeOut))
		//check, if server has to be stopped
		if atomic.LoadInt32(&i.Started) == 0 {
			break
		}

		n++
		pack.Lock()
		if icyMeta {
			meta, metalen = mount.getIcyMeta()

			if nometabytes+pack.len+delta > mount.State.MetaInfo.MetaInt {
				offset = mount.State.MetaInfo.MetaInt - nometabytes - delta

				//log.Printf("*** write block with meta ***")
				//log.Printf("   offset = %d - %d(nometabytes) - %d (delta) = %d", mount.State.MetaInfo.MetaInt, nometabytes, delta, offset)

				if offset < 0 || offset >= pack.len {
					i.logger.Error(2, "Bad metainfo offset %d", offset)
					log.Printf("!!! Bad metainfo offset %d ***", offset)
					offset = 0
				}

				partwrited, err = bufrw.Write(pack.buffer[:offset])
				if err != nil {
					i.closeAndUnlock(pack, err)
					break
				}
				writed += partwrited
				partwrited, err = bufrw.Write(meta)
				if err != nil {
					i.closeAndUnlock(pack, err)
					break
				}
				writed += partwrited
				partwrited, err = bufrw.Write(pack.buffer[offset:])
				if err != nil {
					i.closeAndUnlock(pack, err)
					break
				}
				writed += partwrited

				delta = writed - offset - metalen
				nometabytes = 0
				nmtmp = 0

				//log.Printf("   delta = %d(writed) - %d(offset) - %d(metalen) = %d", writed, offset, metalen, delta)
			} else {
				writed = 0
				nmtmp, err = bufrw.Write(pack.buffer)
				nometabytes += nmtmp
			}
		} else {
			writed, err = bufrw.Write(pack.buffer)
		}

		if err != nil {
			i.closeAndUnlock(pack, err)
			break
		}

		bytesSended += writed + nmtmp

		// send burst data whithout waiting
		if bytesSended >= mount.Options.BurstSize {
			if time.Since(beginIteration) < time.Second {
				time.Sleep(time.Second - time.Since(beginIteration))
			}
		}

		nextpack = pack.Next()
		for nextpack == nil {
			time.Sleep(time.Millisecond * 250)
			idle += 250
			if idle >= idleTimeOut {
				i.closeAndUnlock(pack, errors.New("empty Buffer idle time is reached"))
				break
			}
			nextpack = pack.Next()
		}
		idle = 0
		pack.UnLock()

		pack = nextpack
	}
}

/*
	writeMount
	Authenticate SOURCE and write stream from it to appropriate mount buffer
*/
func (i *Server) writeMount(idx int, w http.ResponseWriter, r *http.Request) {
	var mount *Mount
	mount = &i.Mounts[idx]

	if !mount.State.Started {
		mount.mux.Lock()
		err := mount.auth(w, r)
		if err != nil {
			mount.mux.Unlock()
			i.logger.Error(1, err.Error())
			return
		}
		mount.writeICEHeaders(w, r)
		mount.State.Started = true
		mount.State.StartedTime = time.Now()
		mount.mux.Unlock()
	} else {
		i.logger.Error(1, "SOURCE already connected")
		http.Error(w, "SOURCE already connected", 403)
		return
	}

	bytesSended := 0
	idle := 0
	readed := 0
	var err error
	start := time.Now()

	i.logger.Error(3, "writeMount "+mount.Options.Name)
	defer i.closeMount(idx, true, &bytesSended, start, r)

	hj, ok := w.(http.Hijacker)
	if !ok {
		i.logger.Error(1, "webserver doesn't support hijacking")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	bufrw := bufio.NewReaderSize(conn, 1024*mount.Options.BitRate/8)

	i.incSources()
	// max bytes per second according to bitrate
	buff := make([]byte, mount.Options.BitRate*1024/8)

	for {
		//check, if server has to be stopped
		if atomic.LoadInt32(&i.Started) == 0 {
			break
		}

		readed, err = bufrw.Read(buff)
		if err != nil {
			if err == io.EOF {
				idle++
				if idle >= i.Options.Limits.SourceIdleTimeOut {
					i.logger.Error(1, "Source idle time is reached")
					break
				}
			}
			i.logger.Error(3, err.Error())
		} else {
			idle = 0
		}
		// append to the buffer's queue based on actual readed bytes
		mount.buffer.Append(buff, readed)
		bytesSended += readed
		i.logger.Error(4, "writeMount "+strconv.Itoa(readed)+"")

		if mount.dumpFile != nil {
			mount.dumpFile.Write(mount.buffer.Last().buffer)
		}

		time.Sleep(1000 * time.Millisecond)

		//check if maxbuffersize reached and truncate it
		mount.buffer.checkAndTruncate()
	}
}
