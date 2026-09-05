//go:build darwin

package main

import (
	"syscall"
)

func isStdinReadable() bool {
	var rdfs syscall.FdSet
	rdfs.Bits[0] = 1
	tv := syscall.Timeval{Sec: 0, Usec: 0}
	err := syscall.Select(1, &rdfs, nil, nil, &tv)
	return err == nil && (rdfs.Bits[0]&1 != 0)
}
