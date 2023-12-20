package main

import (
	"io"
	"time"
)

type timedReader struct {
	r      io.Reader
	readAt time.Time
}

func (t *timedReader) Read(p []byte) (n int, err error) {
	// FIXME: This is not perfect, since it records the time at which the
	//        first read was completed. Ideally it would record the time
	//        the first byte was read.
	n, err = t.r.Read(p)
	if t.readAt.IsZero() && n > 0 {
		t.readAt = time.Now()
	}
	return n, err
}
