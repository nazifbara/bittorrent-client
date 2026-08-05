package main

import (
	"net"
	"sync"
)

type ConnectedPeer struct {
	*net.TCPConn
	*net.TCPAddr
}
type Client struct {
	Torrent        *Torrent
	TrackerConn    *net.UDPConn
	TrackerAddr    *net.UDPAddr
	Retries        int
	connectedPeers []ConnectedPeer
	mu             sync.Mutex
}

func newClient(torrent *Torrent, trackerConn *net.UDPConn, TrackerAddr *net.UDPAddr) Client {
	return Client{
		Torrent:     torrent,
		TrackerConn: trackerConn,
		TrackerAddr: TrackerAddr,
		Retries:     20,
	}
}
