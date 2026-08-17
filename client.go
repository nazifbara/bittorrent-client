package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Job struct {
	index uint32
	begin uint32
}

type pendingRequest struct {
	job    *Job
	peer   *Peer
	timer  *time.Timer
	sentAt time.Time
}

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
	done         chan struct{}
	failureCount atomic.Int32
	closeOnce    sync.Once
	rtt          rttTracker
	mu           sync.Mutex
}

func (p *Peer) recordFailure() {
	p.failureCount.Add(1)
	if p.failureCount.Load() > 5 {
		p.markDone()
		return
	}
}

func (p *Peer) markDone() {
	p.closeOnce.Do(func() {
		close(p.done)
	})
}

func (j *Job) String() string {
	return fmt.Sprintf("Job{Index:%d, Begin:%d}", j.index, j.begin)
}

const blockSize uint32 = 16384

type Client struct {
	mu              sync.Mutex
	torrent         *Torrent
	queue           chan *Job
	pending         map[string]*pendingRequest
	activePeers     []*Peer
	piecesGrid      []*PieceState
	peerAddresses   []*net.TCPAddr
	startedAt       time.Time
	totalDownloaded atomic.Uint64
	done            chan struct{}
	doneOnce        sync.Once
	file            *os.File
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

func NewClient(torrent *Torrent) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	piecesGrid := make([]*PieceState, torrent.NumOfPieces)
	for i := range piecesGrid {
		pieceSize := torrent.PieceSize
		if i == int(torrent.NumOfPieces)-1 {
			pieceSize = torrent.FinalPieceSize
		}
		numOfBlock := pieceSize / uint64(blockSize)
		if pieceSize%uint64(blockSize) != 0 {
			numOfBlock++
		}
		bytes := make([]byte, pieceSize)
		piecesGrid[i] = &PieceState{Bytes: bytes, PieceSize: pieceSize, TotalBlocks: int(numOfBlock)}
	}
	return &Client{
		torrent:    torrent,
		piecesGrid: piecesGrid,
		queue:      make(chan *Job, torrent.NumOfBlocks),
		pending:    make(map[string]*pendingRequest),
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

func (c *Client) Start(annnounceList [][]string) error {
	err := os.Mkdir(c.torrent.Name, 0755)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	file, err := os.OpenFile(fmt.Sprintf("%s/%s.download", c.torrent.Name, c.torrent.Name), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	c.file = file

	addresses, err := c.GetPeerAddresses(annnounceList)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return errors.New("coudn't find peers")
	}
	c.peerAddresses = addresses
	c.startedAt = time.Now()
	log.Println("⬇️ Downloading...")
	return c.download()
}
