package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type PieceState struct {
	Bytes       []byte
	BlocksRead  int
	TotalBlocks int
	PieceSize   uint64
	Done        bool
	Received    []bool
	mu          sync.Mutex
}

type Peer struct {
	*net.TCPConn
	*net.TCPAddr
	IsChoke      bool
	Bitfield     []bool
	writeFailure atomic.Int32
	mu           sync.Mutex
}

type Job struct {
	Index         uint32
	Begin         uint32
	LastRequested time.Time
}

func (j *Job) String() string {
	return fmt.Sprintf("Job{Index:%d, Begin:%d}", j.Index, j.Begin)
}

const blockSize uint32 = 16384

type Client struct {
	Torrent       *Torrent
	TrackerConn   *net.UDPConn
	TrackerAddr   *net.UDPAddr
	ActivePeers   []*Peer
	PiecesGrid    []*PieceState
	PeerAddresses []*net.TCPAddr
	Queue         []*Job
	JobsChannel   chan *Job
	ReadyChannel  chan struct{}
	LastBlockAt   time.Time
	AvgRTT        time.Duration
	File          *os.File
	BlockSize     uint32
	mu            sync.Mutex
	startedAt     time.Time
}

func newClient(torrent *Torrent) Client {
	return Client{
		Torrent:   torrent,
		BlockSize: blockSize,
	}
}

func (c *Client) RecordRTT(sample time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.AvgRTT == 0 {
		c.AvgRTT = sample
		return
	}
	const alpha = 0.125
	c.AvgRTT = time.Duration(float64(c.AvgRTT)*(1-alpha) + float64(sample)*alpha)
}

func (c *Client) RoundtripTimeout() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.AvgRTT == 0 {
		return 8 * time.Second
	}
	timeout := c.AvgRTT * 4
	timeout = max(timeout, 2*time.Second)
	timeout = min(timeout, 20*time.Second)
	return timeout
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
		pieceSize := c.Torrent.PieceSize
		if i == int(c.Torrent.NumOfPieces)-1 {
			pieceSize = c.Torrent.FinalPieceSize
		}
		numOfBlock := pieceSize / uint64(c.BlockSize)
		if pieceSize%uint64(c.BlockSize) != 0 {
			numOfBlock++
		}
		bytes := make([]byte, pieceSize)
		received := make([]bool, numOfBlock)
		c.PiecesGrid[i] = &PieceState{Bytes: bytes, Received: received, PieceSize: pieceSize, TotalBlocks: int(numOfBlock)}
		c.addPieceJobs(uint32(i))
	}

	addresses, err := c.GetPeerAddresses(annnounceList)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return errors.New("coudn't find peers")
	}
	c.PeerAddresses = addresses
	c.startedAt = time.Now()
	log.Println("⬇️ Downloading...")
	c.HandlePeers()
	return nil
}
