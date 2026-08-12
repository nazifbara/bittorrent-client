package main

import (
	"encoding/binary"
	"log"
	"slices"
	"time"
)

func (c *Client) Download(peer *Peer) {
	var buffer []byte
	peer.Write(c.BuildInterested())
	go func() {
		for {
			if peer == nil {
				return
			}
			c.mu.Lock()
			n := min(len(c.ActivePeers)*8, len(c.Queue))
			queueSnapshot := append([]*Job(nil), c.Queue[:n]...)
			c.mu.Unlock()

			for _, job := range queueSnapshot {
				if job == nil {
					continue
				}
				c.mu.Lock()
				ready := !c.PiecesGrid[job.Index].Done &&
					peer.Bitfield[job.Index] && !peer.IsChoke &&
					time.Since(job.LastRequested) > 4*time.Second
				if !ready {
					c.mu.Unlock()
					continue
				}
				job.LastRequested = time.Now()
				begin := uint32(c.PiecesGrid[job.Index].Begin)
				req := c.BuildRequest(uint32(job.Index), begin, c.BuildBlockSize(job.Index, begin))
				c.mu.Unlock()

				if _, err := peer.Write(req); err == nil {
					// log.Printf("requested index=%d begin=%d from %v\n", job.Index, c.PiecesGrid[job.Index].Begin, peer.TCPAddr)
					onJobRequested(job, peer)
					break
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
	for {
		if c.Finished {
			peer.Close()
			break
		}
		chunk := make([]byte, 1024)
		n, err := peer.Read(chunk)
		if err != nil {
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

			c.HandleMessage(peer, msg)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (c *Client) HandleMessage(peer *Peer, msg []byte) {
	message, err := parseMessage(msg)
	if err != nil {
		return
	}
	switch message.ID {
	case 0:
		c.HandleChoke(peer)
	case 1:
		c.HandleUnchok(peer)
	case 4:
		c.HandleHave(peer, message)
	case 5:
		c.HandleBitfield(peer, message)
	case 7:
		c.HandleBlock(peer, message)
	}
}

func (c *Client) HandleHave(peer *Peer, msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, _ := parseHavePayload(msg.Payload)
	// fmt.Printf("%v have %d\n", peer.TCPAddr, payload.Index)
	peer.Bitfield[payload.Index] = true
	c.AddToQueue(int(payload.Index), peer)
}

func (c *Client) AddToQueue(pieceIndex int, peer *Peer) {
	if c.PiecesGrid[pieceIndex].Done {
		return
	}
	if job := c.getJobByPieceIdx(uint32(pieceIndex)); job != nil {
		return
	}
	c.Queue = append(c.Queue, &Job{Index: uint32(pieceIndex)})
}

func onJobRequested(job *Job, peer *Peer) {
	now := time.Now()
	job.LastRequested = now
	peer.AliveAt = now
}

func (c *Client) IsHot(peer *Peer) bool {
	return time.Since(peer.AliveAt) < 2*time.Minute
}

func (c *Client) HandleUnchok(peer *Peer) {
	// fmt.Printf("unchoke by %v\n", peer.TCPAddr)
	peer.IsChoke = false
}

func (c *Client) HandleChoke(peer *Peer) {
	// fmt.Printf("choke by %v\n", peer.TCPAddr)
	peer.IsChoke = true
}

func (c *Client) HandleBitfield(peer *Peer, msg Message) {
	// fmt.Printf("received %v bitfield\n", peer.TCPAddr)
	payload := parseBitfieldPayload(msg.Payload)
	peer.Bitfield = payload.Bitfield
	for i, complete := range payload.Bitfield {
		if complete {
			c.mu.Lock()
			c.AddToQueue(i, peer)
			c.mu.Unlock()
		}
	}
}

func (c *Client) HandleBlock(peer *Peer, msg Message) {
	payload, err := parseBlockPayload(msg.Payload)
	if err != nil {
		return
	}
	if len(c.Queue) == 0 {
		return
	}
	pieceState := c.PiecesGrid[payload.Index]
	if pieceState.Done {
		return
	}

	if int(payload.Begin) != pieceState.Begin {
		log.Printf("Bad offset for piece %d\n", payload.Index)
		return
	}
	expectedPieceSize := int(c.Torrent.PieceSize)
	if payload.Index == uint32(c.Torrent.NumOfPieces-1) {
		expectedPieceSize = int(c.Torrent.FinalPieceSize)
	}
	pieceState.Bytes = append(pieceState.Bytes, payload.Block...)
	pieceState.Begin = len(pieceState.Bytes)
	pieceState.Done = expectedPieceSize == int(pieceState.Begin)
	c.mu.Lock()
	c.Torrent.TotalDownloaded += uint64(len(payload.Block))
	c.mu.Unlock()
	log.Printf("Progress %d / %d\n", c.Torrent.TotalDownloaded, c.Torrent.ContentSize)
	if pieceState.Done {
		c.mu.Lock()
		c.CompletedJobs++
		c.Queue = slices.DeleteFunc(c.Queue, func(j *Job) bool { return j.Index == payload.Index })
		c.mu.Unlock()

		offset := int64(payload.Index) * int64(c.Torrent.PieceSize)
		go func() {
			n, err := c.File.WriteAt(pieceState.Bytes, offset)
			if err != nil {
				log.Printf("couldn't write piece to file: %v", err)
			} else {
				log.Printf("%d written to file\n", n)
				c.Finished = c.Torrent.TotalDownloaded == c.Torrent.ContentSize
				if c.Finished {
					log.Printf("✅ Download completed in %v/n", time.Since(c.startedAt).Minutes())
				}
			}
		}()
	} else {
		_, err := peer.Write(c.BuildRequest(payload.Index, uint32(pieceState.Begin), c.BuildBlockSize(payload.Index, uint32(pieceState.Begin))))
		if err != nil {
			log.Println(err.Error())
			return
		}
		job := c.getJobByPieceIdx(payload.Index)
		if job == nil {
			return
		}
		onJobRequested(job, peer)
	}
}

func (c *Client) BuildBlockSize(pieceIdx uint32, currentBegin uint32) uint32 {
	totalPieceSize := uint32(c.Torrent.PieceSize)
	if pieceIdx == uint32(c.Torrent.NumOfPieces-1) {
		totalPieceSize = uint32(c.Torrent.FinalPieceSize)
	}
	rem := totalPieceSize - currentBegin
	if rem < c.BlockSize {
		return rem
	}
	return c.BlockSize
}

func (c *Client) getJobByPieceIdx(pieceId uint32) *Job {
	jobIdx := slices.IndexFunc(c.Queue, func(j *Job) bool { return j.Index == pieceId })
	if jobIdx == -1 {
		return nil
	}
	return c.Queue[jobIdx]
}
