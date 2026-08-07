package main

import (
	"crypto/rand"
	"net"
	"time"
)

func randomPeerID() []byte {
	peerID := make([]byte, 20)
	copy(peerID, []byte("-qB0001-"))
	if _, err := rand.Read(peerID[8:]); err != nil {
		panic(err)
	}
	return peerID
}

func retry[T any](retries int, conn net.Conn, operation func() (T, error)) (T, error) {
	var lastErr error
	var result T
	for range retries {
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		result, lastErr = operation()
		conn.SetDeadline(time.Time{})
		if ne, ok := lastErr.(net.Error); ok && ne.Timeout() {
			continue
		}
		break
	}
	conn.SetDeadline(time.Time{})
	return result, lastErr
}
