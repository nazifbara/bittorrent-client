package main

import (
	"encoding/binary"
	"fmt"
	"time"
)

func (c *Client) HandleDownloads() {
	for {
		handlingMap := make(map[string]bool)
		for _, peer := range c.ActivePeers {
			if !handlingMap[peer.TCPAddr.String()] {
				handlingMap[peer.TCPAddr.String()] = true
				go c.Download(peer)
			}
		}
		time.Sleep(time.Second)
	}
}

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
	// if message.ID > 0 {
	// 	fmt.Printf("%v message ID: %v, size: %d\n", peer.TCPAddr, message.ID, message.Size)
	// }
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
	go func() {
		for {
			peer.Write(c.BuildRequest(25, 0, 16384))
		}
	}()
}

func (c *Client) HandleBitfield(peer *Peer, msg Message) {
	payload := parseBitfieldPayload(msg.Payload)
	if payload.Bitfield[25] {
		peer.Write(c.BuildInterested())
		fmt.Printf("%v has piece  from bitfield\n", peer.TCPAddr)
	}
}

func (c *Client) HandleBlock(peer *Peer, msg Message) {
	payload, err := parseBlockPayload(msg.Payload)
	if err != nil {
		return
	}
	fmt.Printf("%v sent sent a block: {begin=%d,index=%d,size=%d}\n", peer.TCPAddr, payload.Begin, payload.Index, len(payload.Block))
}
