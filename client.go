package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type FileState struct {
	begin       int64
	size        int64
	file        *os.File
	blocksWrote int
	numOfBlocks int
	name        string
	mu          sync.Mutex
}

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
	bytes       []byte
	blocksRead  int
	numOfBlocks int
	pieceSize   uint64
	done        bool
	Received    []bool
	mu          sync.Mutex
}

func (p *PieceState) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blocksRead = 0
	p.bytes = make([]byte, p.numOfBlocks)
	p.done = false
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
	addrQeue        chan *net.TCPAddr
	pending         map[string]*pendingRequest
	activePeers     []*Peer
	piecesGrid      []*PieceState
	peerAddresses   []*net.TCPAddr
	startedAt       time.Time
	totalDownloaded atomic.Int64
	done            chan struct{}
	doneOnce        sync.Once
	filesGrid       []*FileState
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

func newClient(torrent *Torrent) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	piecesGrid := make([]*PieceState, torrent.numOfPieces)
	for i := range piecesGrid {
		pieceSize := torrent.pieceSize
		if i == int(torrent.numOfPieces)-1 {
			pieceSize = torrent.finalPieceSize
		}
		numOfBlock := pieceSize / uint64(blockSize)
		if pieceSize%uint64(blockSize) != 0 {
			numOfBlock++
		}
		bytes := make([]byte, pieceSize)
		piecesGrid[i] = &PieceState{bytes: bytes, pieceSize: pieceSize, numOfBlocks: int(numOfBlock)}
	}
	return &Client{
		torrent:    torrent,
		piecesGrid: piecesGrid,
		queue:      make(chan *Job, torrent.numOfBlocks),
		pending:    make(map[string]*pendingRequest),
		ctx:        ctx,
		cancel:     cancel,
		filesGrid:  make([]*FileState, len(torrent.files)),
		done:       make(chan struct{}),
	}
}

func createFileFromPath(root string, path []string) (*os.File, error) {
	builder := strings.Builder{}
	builder.WriteString(root)
	for i, p := range path {
		if len(path) > 1 {
			fmt.Fprintf(&builder, "/%s", p)
		}
		fmt.Fprintf(&builder, "%s", p)

		if i != len(path)-1 && len(path) > 1 {
			if err := os.Mkdir(builder.String(), 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return &os.File{}, err
			}
			continue
		}

		file, err := os.OpenFile(builder.String(), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Fatal(err)
			return &os.File{}, nil
		}
		return file, nil
	}
	return &os.File{}, nil
}

func (c *Client) Start(annnounceList [][]string) error {
	parrentFolderName := ""
	if len(c.torrent.files) > 1 {
		err := os.Mkdir(c.torrent.name, 0755)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		parrentFolderName = c.torrent.name
	}

	fileBegin := int64(0)
	for i, f := range c.torrent.files {
		file, err := createFileFromPath(parrentFolderName, f.Path)
		if err != nil {
			return err
		}
		if i != 0 {
			fileBegin += c.torrent.files[i-1].Length
		}
		numOfBlocks := f.Length / int64(blockSize)

		if f.Length%int64(blockSize) != 0 {
			numOfBlocks++
		}
		name := f.Path[len(f.Path)-1]
		c.filesGrid[i] = &FileState{
			begin:       fileBegin,
			size:        f.Length,
			file:        file,
			numOfBlocks: int(numOfBlocks),
			name:        name,
		}
	}

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
