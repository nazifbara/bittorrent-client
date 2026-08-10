package main

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type PieceState struct {
	Index uint32
	Begin uint32
	Bytes []byte
	peer  *Peer
	mu    sync.Mutex
	Done  bool
}

type Peer struct {
	*net.TCPConn
	*net.TCPAddr
	lastConnected time.Time
}
type Client struct {
	Torrent     *Torrent
	TrackerConn *net.UDPConn
	TrackerAddr *net.UDPAddr
	ActivePeers []*Peer
	PieceState  PieceState
	File        *os.File
	BlockSize   uint32
	mu          sync.Mutex
}

func newClient(torrent *Torrent, trackerConn *net.UDPConn, TrackerAddr *net.UDPAddr) Client {
	return Client{
		Torrent:     torrent,
		TrackerConn: trackerConn,
		TrackerAddr: TrackerAddr,
		BlockSize:   16384,
	}
}

func (c *Client) Start() error {
	err := os.Mkdir(c.Torrent.Name, 0755)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(fmt.Sprintf("%s/%s.download", c.Torrent.Name, c.Torrent.Name), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	c.File = file
	addresses, err := c.GetPeerAddresses()
	if err != nil {
		return err
	}
	go c.HealthcheckPeers(addresses)

	return nil
}
