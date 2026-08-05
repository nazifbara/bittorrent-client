package main

import (
	"crypto/rand"
)

func randomPeerID() []byte {
	peerID := make([]byte, 20)
	copy(peerID, []byte("-qB0001-"))
	if _, err := rand.Read(peerID[8:]); err != nil {
		panic(err)
	}
	return peerID
}
