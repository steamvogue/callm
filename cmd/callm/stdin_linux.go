//go:build linux

package main

import (
	"syscall"
)

func isStdinReadable() bool {
	var rdfs syscall.FdSet
	rdfs.Bits[0] = 1
	tv := syscall.Timeval{Sec: 0, Usec: 0}
	n, err := syscall.Select(1, &rdfs, nil, nil, &tv)
	return err == nil && n > 0
}
