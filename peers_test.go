package main

import (
	"net"
	"testing"
)

func TestGetPeerByAddrFound(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 6881}
	peer := &Peer{TCPAddr: addr}
	c := &Client{activePeers: []*Peer{peer}}

	idx, got := c.getPeerByAddr(addr)
	if idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
	if got != peer {
		t.Fatal("expected to get back the same peer pointer")
	}
}

func TestGetPeerByAddrNotFound(t *testing.T) {
	known := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 6881}
	c := &Client{activePeers: []*Peer{{TCPAddr: known}}}

	lookup := &net.TCPAddr{IP: net.ParseIP("198.51.100.9"), Port: 6881}
	idx, got := c.getPeerByAddr(lookup)
	if idx != -1 {
		t.Fatalf("expected index -1, got %d", idx)
	}
	if got != nil {
		t.Fatalf("expected nil peer, got %+v", got)
	}
}

func TestGetPeerByAddrMatchesOnIPAndPort(t *testing.T) {
	samePort := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 6881}
	c := &Client{activePeers: []*Peer{{TCPAddr: samePort}}}

	// same IP, different port -- must not match
	differentPort := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 6882}
	idx, got := c.getPeerByAddr(differentPort)
	if idx != -1 || got != nil {
		t.Fatalf("expected no match for differing port, got idx=%d peer=%+v", idx, got)
	}
}
