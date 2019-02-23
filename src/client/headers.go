// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package iceclient

import (
	"bufio"
	"net/textproto"
	"strings"
)

type httpHeaders map[string]string

func processHTTPHeaders(reader *bufio.Reader) (httpHeaders, error) {
	headers := make(map[string]string)
	tp := textproto.NewReader(reader)

	for {
		line, err := tp.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "" {
			break
		}
		params := strings.Split(line, ":")
		if len(params) >= 3 {
			// field Address could contain few addresses delimited by [:] in case of several candidates
			// for now i process only one
			if params[0] == "Address" {
				headers["Address"] = strings.TrimSpace(params[1]) + ":" + strings.TrimSpace(params[2])
			}
		} else if len(params) == 2 {
			if params[0] == "X-Audiocast-Bitrate" {
				headers["X-Audiocast-Bitrate"] = strings.TrimSpace(params[1])
			}
		}

	}

	return headers, nil
}
