//go:build !linux && !darwin

package main

func isStdinReadable() bool {
	return false
}
