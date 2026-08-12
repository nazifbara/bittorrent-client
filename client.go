package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type PieceState struct {
	Begin int
	Bytes []byte
	Done  bool
}

type Peer struct {
	*net.TCPConn
	*net.TCPAddr
	IsInactive bool
	IsChoke    bool
	AliveAt    time.Time
	Bitfield   []bool
}
type Job struct {
	Index         uint32
	LastRequested time.Time
}
type Client struct {
	Torrent       *Torrent
	TrackerConn   *net.UDPConn
	TrackerAddr   *net.UDPAddr
	ActivePeers   []*Peer
	PiecesGrid    []*PieceState
	PeerAddresses []*net.TCPAddr
	Finished      bool
	Queue         []*Job
	File          *os.File
	CompletedJobs uint32
	BlockSize     uint32
	mu            sync.Mutex
	startedAt     time.Time
}

const blockSize uint32 = 16384

func newClient(torrent *Torrent) Client {
	return Client{
		Torrent:   torrent,
		BlockSize: blockSize,
	}
}

func (c *Client) Start(annnounceList [][]string) error {
	err := os.Mkdir(c.Torrent.Name, 0755)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	file, err := os.OpenFile(fmt.Sprintf("%s/%s.download", c.Torrent.Name, c.Torrent.Name), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	c.File = file

	c.PiecesGrid = make([]*PieceState, c.Torrent.NumOfPieces)
	for i := range c.PiecesGrid {
		c.PiecesGrid[i] = &PieceState{Begin: 0}
	}
	addresses, err := c.GetPeerAddresses(annnounceList)
	if err == nil {
		c.PeerAddresses = addresses
	}
	c.startedAt = time.Now()
	log.Println("⬇️ Downloading...")
	go c.HealthcheckPeers()
	return nil
}
