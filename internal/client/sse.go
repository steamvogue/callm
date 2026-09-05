package client

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"strings"
)

const maxEventBytes = 8 << 20

// sseReader dispatches complete events, including multiline data and CR/LF framing.
type sseReader struct {
	scanner *bufio.Scanner
	first   bool
}

func newSSEReader(r io.Reader) *sseReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), maxEventBytes)
	s.Split(func(data []byte, eof bool) (int, []byte, error) {
		for i, b := range data {
			if b == '\n' {
				return i + 1, data[:i], nil
			}
			if b == '\r' {
				if i+1 == len(data) && !eof {
					return 0, nil, nil
				}
				end := i + 1
				if end < len(data) && data[end] == '\n' {
					end++
				}
				return end, data[:i], nil
			}
		}
		if eof && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	return &sseReader{scanner: s, first: true}
}

func (r *sseReader) next() (string, error) {
	var data []string
	size := 0
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if r.first {
			line = strings.TrimPrefix(line, "\ufeff")
			r.first = false
		}
		if line == "" {
			if len(data) > 0 {
				return strings.Join(data, "\n"), nil
			}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		if field != "data" {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		size += len(value) + 1
		if size > maxEventBytes {
			return "", fmt.Errorf("SSE event exceeds %d bytes", maxEventBytes)
		}
		data = append(data, value)
	}
	if err := r.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.ErrUnexpectedEOF
}

func checkSSEType(contentType string) error {
	typ, _, err := mime.ParseMediaType(contentType)
	if err != nil || typ != "text/event-stream" {
		return fmt.Errorf("expected text/event-stream, received %q", contentType)
	}
	return nil
}
