// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

//223.33.152.54 - - [27/Feb/2012:13:37:21 +0300] "GET /gop_aac HTTP/1.1" 200 75638 "-" "WMPlayer/10.0.0.364 guid/3300AD50-2C39-46C0-AE0A-AC7B8159E203" 400
func (i *Server) writeAccessLog(host string, startTime time.Time, request string, bytesSend int, refer, userAgent string, seconds int) {
	i.logger.Access("%s - - [%s] \"%s\" %s %d \"%s\" \"%s\" %d\r\n", host, startTime.Format(time.RFC1123Z), request, "200", bytesSend, refer, userAgent, seconds)
}

func (i *Server) getHost(addr string) string {
	idx := strings.Index(addr, ":")
	if idx == -1 {
		return addr
	}
	return addr[:idx]
}

func (i *Server) logHeaders(r *http.Request) {
	request := r.Method + " " + r.RequestURI + " " + r.Proto + "\n"
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request += fmt.Sprintf("%v: %v\n", name, h)
		}
	}
	i.logger.Error("\n" + request)
}

/*
	openMount
    Decide what to do, according to HTTP method
*/
/*func (i *Server) openMount(idx int, w http.ResponseWriter, r *http.Request) {
	if r.Method == "SOURCE" || r.Method == "PUT" {
		if !i.checkSources() {
			i.logger.Error("Number of sources exceeded")
			http.Error(w, "Number of sources exceeded", 403)
			return
		}
		i.writeMount(idx, w, r)
	} else {
		if !i.checkListeners() {
			i.logger.Error("Number of listeners exceeded")
			http.Error(w, "Number of listeners exceeded", 403)
			return
		}
		var im = false
		if r.Header.Get("icy-metadata") == "1" {
			im = true
		}
		i.readMount(idx, im, w, r)
	}
}*/
