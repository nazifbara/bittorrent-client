package main

import (
	"crypto/rand"
	"errors"
	"net"
	"time"
)

func bitsToBytes(bits []bool) ([]byte, error) {
	if len(bits)%8 != 0 {
		return []byte{}, errors.New("bit length must be a multiple of 8")
	}

	out := make([]byte, 0, len(bits)/8)
	for i := 0; i < len(bits); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if bits[i+j] {
				b |= 1 << (7 - j)
			}
		}
		out = append(out, b)
	}
	return out, nil
}

func bytesToBits(b []byte) []bool {
	bits := make([]bool, 0, len(b)*8)
	for _, v := range b {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (v>>i)&1 == 1)
		}
	}
	return bits
}

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
	return result, lastErr
}
