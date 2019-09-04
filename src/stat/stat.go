// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

// Package stat - read current process stat from /proc/[pid]/stat.
// Used for CPU and memory usage monitoring
package stat

/* #include <unistd.h> */
import "C"
import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// ProcStats - Status information about the process
// /proc/[pid]/stat
type ProcStats struct {
	Utime     float64 // Amount of time that this process has been scheduled in user mode
	Stime     float64 // Amount of time that this process has been scheduled in kernel mode
	Cutime    float64 // Amount of time that this process's waited-for chil‐dren have been scheduled in user mode
	Cstime    float64 // Amount of time that this process's waited-for chil‐dren have been scheduled in kernel mode
	Rss       int     // Resident Set Size: number of pages the process has in real memory
	Starttime float64 // The time the process started after system boot

	uptime float64 // for calculating between stat samples
}

// ProcStatsReader read info from /proc/[pid]/stat file
type ProcStatsReader struct {
	Pid      int
	clkTck   float64
	pageSize int
	numCPU   int

	mux  sync.Mutex
	prev ProcStats

	procFile *os.File
}

// Init - initialize proc stat reader
func (p *ProcStatsReader) Init() error {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.Pid = os.Getpid()

	p.clkTck = float64(C.sysconf(C._SC_CLK_TCK))
	p.pageSize = int(C.sysconf(C._SC_PAGESIZE))
	p.numCPU = int(C.sysconf(C._SC_NPROCESSORS_ONLN))

	procFile, err := os.Open(fmt.Sprintf("/proc/%d/stat", p.Pid))
	if err != nil {
		procFile.Close()
		return err
	}
	p.procFile = procFile
	return nil
}

// Close - close proc/[pid]/stat file
func (p *ProcStatsReader) Close() {
	p.mux.Lock()
	defer p.mux.Unlock()
	if p.procFile != nil {
		p.procFile.Close()
	}
}

func parseFloat(val string) float64 {
	floatVal, _ := strconv.ParseFloat(val, 64)
	return floatVal
}

func parseInt(val string) int {
	intVal, _ := strconv.Atoi(val)
	return intVal
}

// read - reads a /proc/[pid]/stat file and returns statistics
func (p *ProcStatsReader) read() (*ProcStats, error) {
	if p.procFile == nil {
		return nil, errors.New("ProcStatsReader have to be initialized")
	}

	_, err := p.procFile.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 8192)
	n, err := p.procFile.Read(buf)
	if err != nil {
		return nil, err
	}
	splitAfter := strings.SplitAfter(string(buf[:n]), ")")
	if len(splitAfter) == 0 || len(splitAfter) == 1 {
		return nil, errors.New("error parsing /proc/%d/stat")
	}

	uptimeBytes, err := ioutil.ReadFile(path.Join("/proc", "uptime"))
	if err != nil {
		return nil, err
	}
	uptime, _ := strconv.ParseFloat(strings.Split(string(uptimeBytes), " ")[0], 64)

	fields := strings.Split(splitAfter[1], " ")
	stat := &ProcStats{
		Utime:     parseFloat(fields[12]), // [idx+2]
		Stime:     parseFloat(fields[13]),
		Cutime:    parseFloat(fields[14]),
		Cstime:    parseFloat(fields[15]),
		Starttime: parseFloat(fields[20]),
		Rss:       parseInt(fields[22]),
		uptime:    uptime,
	}

	return stat, nil
}

// GetCPUAndMem - returns CPU and memory usage
func (p *ProcStatsReader) GetCPUAndMem() (float64, int, error) {
	var CPU float64
	var Memory int

	if runtime.GOOS == "linux" {

		stat, err := p.read()
		if err != nil {
			return 0, 0, err
		}

		p.mux.Lock()
		prev := p.prev
		p.mux.Unlock()

		systemTime := 0.0
		userTime := 0.0
		seconds := 0.0
		if prev.Stime != 0.0 {
			systemTime = prev.Stime
		}
		if prev.Utime != 0.0 {
			userTime = prev.Utime
		}

		total := stat.Stime - systemTime + stat.Utime - userTime
		total = total / p.clkTck

		if prev.uptime != 0.0 {
			seconds = stat.uptime - prev.uptime
		} else {
			seconds = stat.Starttime/p.clkTck - stat.uptime
		}

		seconds = math.Abs(seconds)
		if seconds == 0 {
			seconds = 1
		}

		CPU = (total / seconds) * 100 / float64(p.numCPU)
		Memory = stat.Rss * p.pageSize

		p.mux.Lock()
		p.prev = *stat
		p.mux.Unlock()
	}
	return CPU, Memory, nil

}
