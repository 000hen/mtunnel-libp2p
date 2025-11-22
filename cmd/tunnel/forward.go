package main

import (
	"io"
)

func pipe(a io.ReadWriteCloser, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(a, b)
		if cw, ok := a.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		} else {
			a.Close()
		}
		done <- struct{}{}
	}()

	go func() {
		io.Copy(b, a)
		if cw, ok := b.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		} else {
			b.Close()
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}
