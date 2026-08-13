package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"sync"
	"time"
)

func (c *Client) connectToPeer(address *net.TCPAddr) (*Peer, error) {
	if address == nil {
		return nil, errors.New("address is nil")
	}
	_, existingPeer := c.getPeerByAddr(address)
	if existingPeer != nil {
		return existingPeer, nil
	}
	conn, err := net.DialTimeout("tcp", address.String(), 300*time.Millisecond)
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("connection is not *net.TCPConn: %T", conn)
	}

	peer := &Peer{TCPConn: tcpConn, TCPAddr: address, Bitfield: make([]bool, c.Torrent.NumOfPieces), offsetSeed: newPeerOffsetSeed(address.String())}

	return peer, nil
}

func (c *Client) getPeerByAddr(address *net.TCPAddr) (int, *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := slices.IndexFunc(c.ActivePeers, func(ap *Peer) bool {
		return address.IP.Equal(ap.IP) && address.Port == ap.Port
	})
	if index == -1 {
		return index, nil
	}
	return index, c.ActivePeers[index]
}

func (c *Client) HandlePeers() {
	sem := make(chan struct{}, 10)
	for {
		if len(c.PeerAddresses) == 0 {
			return
		}

		if len(c.ActivePeers) > 0 {
			break
		}
		log.Println("searching active peers...")
		var wg sync.WaitGroup
		wg.Add(len(c.PeerAddresses))
		for _, addr := range c.PeerAddresses {
			addr := *addr
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				peer, err := c.connectToPeer(&addr)
				if err == nil {
					c.Handshake(peer)
				}
			}()
		}
		wg.Wait()
		if len(c.ActivePeers) > 0 {
			fmt.Printf("%d active peers out of %d\n", len(c.ActivePeers), len(c.PeerAddresses)-len(c.ActivePeers))
		}
	}
	var peerGroup sync.WaitGroup
	go func() {
		for _, peer := range c.ActivePeers {
			peerGroup.Add(1)
			go c.Download(peer, &peerGroup)
		}
	}()
	for {
		for _, job := range c.Queue {
			c.JobsChannel <- job
			time.Sleep(100 * time.Millisecond)
		}
		if len(c.Queue) == 0 {
			break
		}
	}
	peerGroup.Wait()
}
