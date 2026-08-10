package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

func (c *Client) Download(peer *Peer) {
	var buffer []byte

	for {
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
		}
	}
}

func (c *Client) HandleMessage(peer *Peer, msg []byte) {
	message, err := parseMessage(msg)
	if err != nil {
		return
	}
	switch message.ID {
	case 1:
		c.HandleUnchock(peer)
	case 5:
		c.HandleBitfield(peer, message)
	case 7:
		c.HandleBlock(peer, message)
	}
}

func (c *Client) HandleUnchock(peer *Peer) {
	if c.PieceState.Done {
		return
	}
	peer.Write(c.BuildRequest(c.PieceState.Index, c.PieceState.Begin, c.BlockSize))
	c.PieceState.peer = peer
}

func (c *Client) HandleBitfield(peer *Peer, msg Message) {
	payload := parseBitfieldPayload(msg.Payload)
	if c.PieceState.peer != nil {
		return
	}
	for i, complete := range payload.Bitfield {
		if complete {
			c.PieceState = PieceState{Index: uint32(i), Done: false, Begin: 0}
			peer.Write(c.BuildInterested())
			fmt.Printf("%v has piece  from bitfield\n", peer.TCPAddr)
			break
		}
	}
}

func (c *Client) HandleBlock(peer *Peer, msg Message) {
	payload, err := parseBlockPayload(msg.Payload)
	if err != nil {
		return
	}
	if payload.Index == c.PieceState.Index && c.PieceState.peer.TCPAddr == peer.TCPAddr && !c.PieceState.Done {
		c.PieceState.mu.Lock()
		defer c.PieceState.mu.Unlock()
		c.PieceState.Bytes = append(c.PieceState.Bytes, payload.Block...)
		c.PieceState.Begin = uint32(len(c.PieceState.Bytes))
		c.PieceState.Done = c.Torrent.PieceLength == int(c.PieceState.Begin)
		fmt.Printf("received block from %v: {begin=%d,index=%d,size=%d}\n", peer.TCPAddr, payload.Begin, payload.Index, len(payload.Block))
		fmt.Printf("Dowonload: %d / %d\n", c.PieceState.Begin, c.Torrent.PieceLength)
		if !c.PieceState.Done {
			peer.Write(c.BuildRequest(c.PieceState.Index, c.PieceState.Begin, c.BlockSize))
		} else {
			fmt.Printf("Piece %d completed, isHashValid=%v", c.PieceState.Index, sha1.Sum(c.PieceState.Bytes) == c.Torrent.PieceHashes[c.PieceState.Index])
		}
	}
}
