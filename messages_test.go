package main

import (
	"bytes"
	"testing"
)

func TestBuildHandShake(t *testing.T) {
	infoHash := [20]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	client := Client{Torrent: &Torrent{InfoHash: infoHash}}

	handshake := client.BuildHandShake()
	if len(handshake) != 68 {
		t.Fatalf("expected handshake length 68, got %d", len(handshake))
	}

	if handshake[0] != 19 {
		t.Fatalf("expected pstrlen 19, got %d", handshake[0])
	}

	if string(handshake[1:20]) != "BitTorrent protocol" {
		t.Fatalf("expected pstr %q, got %q", "BitTorrent protocol", string(handshake[1:20]))
	}

	if !bytes.Equal(handshake[20:28], make([]byte, 8)) {
		t.Fatal("expected reserved bytes to be all zero")
	}

	if !bytes.Equal(handshake[28:48], infoHash[:]) {
		t.Fatalf("info hash mismatch: got %x", handshake[28:48])
	}

	peerID := handshake[48:68]
	if len(peerID) != 20 {
		t.Fatalf("expected peer id length 20, got %d", len(peerID))
	}
	if !bytes.HasPrefix(peerID, []byte("-qB0001-")) {
		t.Fatalf("unexpected peer id prefix: %q", string(peerID[:8]))
	}
}
