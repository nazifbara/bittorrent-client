package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"slices"
	"time"
)

func (c *Client) tryRequestPeer(peer *Peer, job *Job) {
	peer.mu.Lock()
	chocked := peer.IsChoke
	has := peer.Bitfield[job.index]
	peer.mu.Unlock()
	if chocked || !has {
		c.requeue(job)
		return
	}

	req := buildRequest(job.index, job.begin, c.buildBlockSize(job.index, job.begin))
	sentAt := time.Now()
	if _, err := peer.Write(req); err != nil {
		c.requeue(job)
		peer.recordFailure()
		return
	}

	timeout := peer.rtt.timeout()
	key := pendingKey(job.index, job.begin)

	c.mu.Lock()
	c.pending[key] = &pendingRequest{
		job:    job,
		peer:   peer,
		sentAt: sentAt,
		timer: time.AfterFunc(timeout, func() {
			c.onTimeout(key, job)
		}),
	}
	c.mu.Unlock()
}

func (c *Client) runPeerWorker(peer *Peer) {
	c.wg.Add(1)
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-peer.done:
			c.addrQeue <- peer.TCPAddr
			return
		case job, ok := <-c.queue:
			if !ok {
				return
			}
			c.tryRequestPeer(peer, job)
		}
	}
}

func (c *Client) readPeerMessages(peer *Peer) {
	c.wg.Add(1)
	defer c.wg.Done()

	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-c.ctx.Done():
			peer.Close()
		case <-stopWatch:
		}
	}()

	var buffer []byte
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		chunk := make([]byte, 1024)
		n, err := peer.Read(chunk)
		if err != nil {
			peer.recordFailure()
			return
		}

		buffer = append(buffer, chunk[:n]...)

		for {
			if len(buffer) < 4 {
				break
			}

			length := int(binary.BigEndian.Uint32(buffer[:4]))
			if length == 0 {
				buffer = buffer[4:]
				continue
			}

			total := 4 + length
			if len(buffer) < total {
				break
			}

			msg := buffer[:total]
			buffer = buffer[total:]

			c.onMessage(peer, msg)
		}
		time.Sleep(time.Microsecond)
	}
}

func (c *Client) onMessage(peer *Peer, msg []byte) {
	message, err := parseMessage(msg)
	if err != nil {
		return
	}
	switch message.id {
	case 0:
		c.onChoke(peer)
	case 1:
		c.onUnchoke(peer)
	case 4:
		c.onHave(peer, message)
	case 5:
		c.onBitfield(peer, message)
	case 7:
		c.onBlockReceived(peer, message)
	}
}

func (c *Client) onHave(peer *Peer, msg Message) {
	payload, _ := parseHavePayload(msg.payload)
	peer.mu.Lock()
	peer.Bitfield[payload.Index] = true
	peer.mu.Unlock()
}

func (c *Client) onUnchoke(peer *Peer) {
	// log.Printf("✉️ unchoke by %v\n", peer.TCPAddr)
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.IsChoke = false
}

func (c *Client) onChoke(peer *Peer) {
	// log.Printf("✉️ choke by %v\n", peer.TCPAddr)
	peer.mu.Lock()
	go peer.mu.Unlock()
	peer.IsChoke = true
}

func (c *Client) onBitfield(peer *Peer, msg Message) {
	// log.Printf("✉️ received %v bitfield\n", peer.TCPAddr)
	payload := parseBitfieldPayload(msg.payload)
	peer.mu.Lock()
	peer.Bitfield = payload.Bitfield
	peer.mu.Unlock()
	go c.runPeerWorker(peer)
}

func (c *Client) onBlockReceived(peer *Peer, msg Message) {
	payload, err := parseBlockPayload(msg.payload)
	if err != nil {
		return
	}
	key := pendingKey(payload.Index, payload.Begin)

	c.mu.Lock()
	pr, ok := c.pending[key]
	if ok {
		delete(c.pending, key)
	}
	c.mu.Unlock()

	if !ok {
		return
	}

	pr.timer.Stop()
	if pr.peer == peer {
		peer.rtt.record(time.Since(pr.sentAt))
	}

	pieceState := c.piecesGrid[payload.Index]
	pieceState.mu.Lock()
	copy(pieceState.Bytes[payload.Begin:int(payload.Begin)+len(payload.Block)], payload.Block)
	pieceState.BlocksRead++
	pieceState.Done = pieceState.BlocksRead == pieceState.NumOfBlocks
	pieceState.mu.Unlock()
	c.totalDownloaded.Add(uint64(len(payload.Block)))
	log.Printf("🍰 Progress %d / %d\n", c.totalDownloaded.Load(), c.torrent.ContentSize)
	if pieceState.Done {
		isValid := sha1.Sum(pieceState.Bytes) == c.torrent.PieceHashes[payload.Index]
		if !isValid {
			log.Printf("⚠️ Re-downloading piece %d due to invalid\n", payload.Index)
			pieceState.reset()
			c.totalDownloaded.Store(c.totalDownloaded.Load() - pieceState.PieceSize)
			c.addPieceJobs(pr.job.index)
			c.doneOnce.Do(func() {
				close(c.done)
			})
		}
		pieceState.Bytes = nil
	}
	go func(data []byte) {
		err := c.writeToFile(payload.Index, payload.Begin, data)
		if err != nil {
			log.Println("❌ Couldn't write to file")
			c.doneOnce.Do(func() {
				close(c.done)
			})
		}
	}(payload.Block)
}

func (c *Client) writeToFile(pieceIndex, pieceBegin uint32, block []byte) error {
	globalBegin := int64(pieceIndex)*int64(c.torrent.PieceSize) + int64(pieceBegin)
	return c.writeAtGlobal(globalBegin, block)
}

func (c *Client) writeAtGlobal(globalBegin int64, data []byte) error {
	for len(data) > 0 {
		index := slices.IndexFunc(c.filesGrid, func(f *FileState) bool {
			return f.begin <= globalBegin && globalBegin < f.begin+f.size
		})
		if index == -1 {
			return fmt.Errorf("no file found for global offset %d", globalBegin)
		}
		fs := c.filesGrid[index]

		fs.mu.Lock()
		offset := globalBegin - fs.begin
		spaceInFile := fs.size - offset
		n := min(int64(len(data)), spaceInFile)

		if _, err := fs.file.WriteAt(data[:n], offset); err != nil {
			fs.mu.Unlock()
			return err
		}
		fs.blocksWrote++
		if fs.blocksWrote == fs.numOfBlocks {
			log.Printf("✅ %s completed", fs.name)
		}
		fs.mu.Unlock()
		if c.totalDownloaded.Load() == c.torrent.ContentSize {
			log.Printf("🔥 Download completed in %.00f\n", time.Since(c.startedAt).Minutes())
			c.doneOnce.Do(func() {
				close(c.done)
			})
		}
		data = data[n:]
		globalBegin += n
	}
	return nil
}
