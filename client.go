package main

import (
	"net"
	"sync"
	"time"
)

type Peer struct {
	*net.TCPConn
	*net.TCPAddr
	lastConnected time.Time
}
type Client struct {
	Torrent     *Torrent
	TrackerConn *net.UDPConn
	TrackerAddr *net.UDPAddr
	Retries     int
	ActivePeers []*Peer
	mu          sync.Mutex
}

func newClient(torrent *Torrent, trackerConn *net.UDPConn, TrackerAddr *net.UDPAddr) Client {
	return Client{
		Torrent:     torrent,
		TrackerConn: trackerConn,
		TrackerAddr: TrackerAddr,
		Retries:     20,
	}
}

func (c *Client) Start() error {
	addresses, err := c.GetPeerAddresses()
	if err != nil {
		return err
	}
	go c.HealthcheckPeers(addresses)
	return nil
}
