package main

import (
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
	for {

		b := make([]byte, 1024)
		peer.Read(b)
		handleMessage(peer, b)
		time.Sleep(time.Second)
	}
}

func handleMessage(peer *Peer, msg []byte) {
	message, err := parseMessage(msg)
	if err != nil {
		return
	}
	fmt.Printf("%v message ID: %v, size: %d\n", peer.TCPAddr, message.ID, message.Size)
	switch message.ID {
	case 5:
		handleBitfield(peer, message)
	}
}

func handleBitfield(peer *Peer, msg Message) {
	payload := parseBitfieldPayload(msg.Payload)
	fmt.Printf("%v bitfield %v\n\n", peer.TCPAddr, payload.Bitfield)
}
