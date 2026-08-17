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

	peer := &Peer{TCPConn: tcpConn, TCPAddr: address, Bitfield: make([]bool, c.torrent.NumOfPieces), rtt: *newRTTTracker(100)}

	return peer, nil
}

func (c *Client) getPeerByAddr(address *net.TCPAddr) (int, *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := slices.IndexFunc(c.activePeers, func(ap *Peer) bool {
		return address.IP.Equal(ap.IP) && address.Port == ap.Port
	})
	if index == -1 {
		return index, nil
	}
	return index, c.activePeers[index]
}

func (c *Client) download() error {
	sem := make(chan struct{}, 10)

	const maxSearchRounds = 5
	rounds := 0
	for {
		if len(c.peerAddresses) == 0 {
			return errors.New("no peer addressed")
		}
		c.mu.Lock()
		activeCount := len(c.activePeers)
		c.mu.Unlock()
		if activeCount > 0 {
			break
		}

		rounds++
		if rounds > maxSearchRounds {
			return fmt.Errorf("❌ no active peers found after max search rounds, giving up")
		}

		log.Println("searching active peers...")
		var wg sync.WaitGroup
		wg.Add(len(c.peerAddresses))
		for _, addr := range c.peerAddresses {
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

		c.mu.Lock()
		activeCount = len(c.activePeers)
		totalCount := len(c.peerAddresses)
		c.mu.Unlock()
		if activeCount > 0 {
			fmt.Printf("%d active peers out of %d\n", activeCount, totalCount-activeCount)
		} else {
			time.Sleep(2 * time.Second)
		}
	}

	for i := range c.piecesGrid {
		c.addPieceJobs(uint32(i))
	}

	for _, peer := range c.activePeers {
		peer.done = make(chan struct{})
		if _, err := peer.Write(buildInterested()); err != nil {
			continue
		}
		go c.readPeerMessages(peer)
	}

	<-c.done
	c.shutdown()
	c.mu.Lock()
	for _, peer := range c.activePeers {
		peer.Close()
	}
	c.mu.Unlock()
	return nil
}
