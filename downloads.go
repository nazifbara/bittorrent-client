package main

import (
	"crypto/sha1"
	"encoding/binary"
	"log"
	"slices"
	"sync"
	"time"
)

func (c *Client) Download(peer *Peer, wg *sync.WaitGroup) {
	defer wg.Done()
	peer.Write(c.BuildInterested())
	wg.Add(1)
	go c.HandleJobs(peer, wg)
	c.HandlePeerMessages(peer)
}

func (c *Client) HandleJobs(peer *Peer, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		if peer == nil {
			return
		}
		peer.mu.Lock()
		if peer.IsChoke {
			peer.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		job, ok := <-c.JobsChannel
		if job == nil || !ok {
			continue
		}

		pieceState := c.PiecesGrid[job.Index]
		pieceState.mu.Lock()
		ready := !pieceState.Done && peer.Bitfield[job.Index]
		if !ready {
			pieceState.mu.Unlock()
			peer.mu.Unlock()
			continue
		}
		pieceState.mu.Unlock()
		peer.mu.Unlock()
		req := c.BuildRequest(uint32(job.Index), job.Begin, c.BuildBlockSize(job.Index, job.Begin))
		if _, err := peer.Write(req); err != nil {
			log.Printf("❌ %v", err)
			job.DoneChan <- struct{}{}
			peer.writeFailure.Store(peer.writeFailure.Load() + 1)
			if peer.writeFailure.Load() > 3 {
				c.mu.Lock()
				c.ActivePeers = slices.DeleteFunc(c.ActivePeers, func(p *Peer) bool { return p.IP.Equal(peer.IP) && p.Port == peer.Port })
				c.mu.Unlock()
				return
			}
			continue
		}
		log.Printf("👆 Waiting job index=%d begin=%d to be done\n", job.Index, job.Begin)
		<-job.DoneChan
		log.Printf("👆 Job done job index=%d begin=%d moving to the next\n", job.Index, job.Begin)
	}
}

func (c *Client) HandlePeerMessages(peer *Peer) {
	var buffer []byte
	for {
		if peer.writeFailure.Load() > 3 {
			return
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
			time.Sleep(300 * time.Millisecond)
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
	payload, _ := parseHavePayload(msg.Payload)
	log.Printf("✉️ %v have %d\n", peer.TCPAddr, payload.Index)
	c.addPieceJobs(payload.Index)
}

func (c *Client) HandleUnchok(peer *Peer) {
	log.Printf("✉️ unchoke by %v\n", peer.TCPAddr)
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.IsChoke = false
}

func (c *Client) HandleChoke(peer *Peer) {
	log.Printf("✉️ choke by %v\n", peer.TCPAddr)
	peer.mu.Lock()
	go peer.mu.Unlock()
	peer.IsChoke = true
}

func (c *Client) HandleBitfield(peer *Peer, msg Message) {
	log.Printf("✉️ received %v bitfield\n", peer.TCPAddr)
	payload := parseBitfieldPayload(msg.Payload)
	peer.mu.Lock()
	peer.Bitfield = payload.Bitfield
	peer.mu.Unlock()
	for i, complete := range payload.Bitfield {
		if complete {
			c.addPieceJobs(uint32(i))
		}
	}
}

func (c *Client) HandleBlock(peer *Peer, msg Message) {
	payload, err := parseBlockPayload(msg.Payload)
	if err != nil {
		return
	}
	// if payload.Index != 0 {
	// 	return
	// }
	log.Printf("🎉 received block from %v: {begin=%d,index=%d,size=%d}\n", peer.TCPAddr, payload.Begin, payload.Index, len(payload.Block))

	pieceState := c.PiecesGrid[payload.Index]
	pieceState.mu.Lock()

	if pieceState.Done {
		pieceState.mu.Unlock()
		return
	}

	blockIdx := int(payload.Begin) / int(c.BlockSize)
	if pieceState.Received[blockIdx] {
		// duplicate delivery from a re-requested job — ignore, don't double-count
		log.Printf("🛡️ Duplicated delivery")
		pieceState.mu.Unlock()
		return
	}
	pieceState.Received[blockIdx] = true

	copy(pieceState.Bytes[payload.Begin:int(payload.Begin)+len(payload.Block)], payload.Block)
	pieceState.BlocksRead++
	c.mu.Lock()
	job := c.getJob(payload.Index, payload.Begin)
	c.Queue = slices.DeleteFunc(c.Queue, func(j *Job) bool { return j.Index == job.Index && j.Begin == job.Begin })
	job.DoneChan <- struct{}{}
	c.mu.Unlock()
	pieceState.Done = pieceState.BlocksRead == pieceState.TotalBlocks
	c.mu.Lock()
	c.Torrent.TotalDownloaded += uint64(len(payload.Block))
	c.mu.Unlock()
	log.Printf("🍰 Progress %d / %d\n", c.Torrent.TotalDownloaded, c.Torrent.ContentSize)

	if pieceState.Done {
		pieceState.mu.Unlock()
		offset := int64(payload.Index) * int64(c.Torrent.PieceSize)
		isValid := sha1.Sum(pieceState.Bytes) == c.Torrent.PieceHashes[payload.Index]
		if isValid {
			log.Printf("✅ Piece %d completed with valid hash\n", payload.Index)
		} else {
			log.Printf("❌ Piece %d completed with invalid hash\n", payload.Index)
		}
		log.Printf("🔧 Job left: %d", len(c.Queue))

		go func(data []byte) {
			n, err := c.File.WriteAt(pieceState.Bytes, offset)
			if err != nil {
				log.Printf("❌ couldn't write piece to file: %v", err)
			} else {
				log.Printf("✍️ %d written to file\n", n)
				pieceState.mu.Lock()
				pieceState.Bytes = nil
				pieceState.mu.Unlock()
				if c.Torrent.TotalDownloaded == c.Torrent.ContentSize {
					log.Printf("🔥 Download completed in %v/n", time.Since(c.startedAt).Minutes())
				}
			}
		}(payload.Block)
		return
	}
	pieceState.mu.Unlock()
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

func (c *Client) addPieceJobs(pieceIdx uint32) {
	pieceState := c.PiecesGrid[pieceIdx]
	pieceState.mu.Lock()
	total := pieceState.TotalBlocks
	pieceState.mu.Unlock()
	for i := range total {
		offset := i * int(c.BlockSize)
		c.AddToQueue(pieceIdx, uint32(offset))
	}
}

// AddToQueue takes pieceState.mu itself before checking pieceState.Done,
// then c.mu for the queue check-and-append. Lock order is always
// pieceState.mu -> c.mu, matching HandleBlock, to avoid deadlocks.
func (c *Client) AddToQueue(pieceIndex uint32, offset uint32) {
	pieceState := c.PiecesGrid[pieceIndex]
	pieceState.mu.Lock()
	if pieceState.Done {
		pieceState.mu.Unlock()
		return
	}
	pieceState.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if job := c.getJob(pieceIndex, offset); job != nil {
		return
	}
	c.Queue = append(c.Queue, &Job{Index: pieceIndex, Begin: offset, DoneChan: make(chan struct{})})
	// log.Printf("👨‍🔧 new job {index=%d, begin:%d}", pieceIndex, offset)
}

// getJob must be called with c.mu already held.
func (c *Client) getJob(pieceId, offset uint32) *Job {
	jobIdx := slices.IndexFunc(c.Queue, func(j *Job) bool { return j.Index == pieceId && j.Begin == offset })
	if jobIdx == -1 {
		return nil
	}
	return c.Queue[jobIdx]
}
