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

func TestBuildKeepAlive(t *testing.T) {
	client := Client{}
	ka := client.BuildKeepAlive()
	if len(ka) != 4 {
		t.Fatalf("expected keep-alive length 4, got %d", len(ka))
	}
	if ka[0] != 0 {
		t.Fatalf("expected keep-alive byte 0, got %d", ka[0])
	}
}

func TestBuildChoke(t *testing.T) {
	client := Client{}

	tests := []struct {
		name string
		want []byte
	}{
		{name: "choke", want: []byte{0, 0, 0, 1, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := client.BuildChoke()
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestBuildUnchoke(t *testing.T) {
	client := Client{}

	tests := []struct {
		name string
		want []byte
	}{
		{name: "unchoke", want: []byte{0, 0, 0, 1, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := client.BuildUnchoke()
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestBuildInterested(t *testing.T) {
	client := Client{}

	tests := []struct {
		name string
		want []byte
	}{
		{name: "interested", want: []byte{0, 0, 0, 1, 2}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := client.BuildInterested()
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
