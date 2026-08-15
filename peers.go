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

	peer := &Peer{TCPConn: tcpConn, TCPAddr: address, Bitfield: make([]bool, c.Torrent.NumOfPieces)}

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

	const maxSearchRounds = 5
	rounds := 0
	for {
		if len(c.PeerAddresses) == 0 {
			return
		}
		c.mu.Lock()
		activeCount := len(c.ActivePeers)
		c.mu.Unlock()
		if activeCount > 0 {
			break
		}

		rounds++
		if rounds > maxSearchRounds {
			log.Println("❌ no active peers found after max search rounds, giving up")
			return
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

		c.mu.Lock()
		activeCount = len(c.ActivePeers)
		totalCount := len(c.PeerAddresses)
		c.mu.Unlock()
		if activeCount > 0 {
			fmt.Printf("%d active peers out of %d\n", activeCount, totalCount-activeCount)
		} else {
			time.Sleep(2 * time.Second) // back off before retrying the whole peer list
		}
	}

	c.mu.Lock()
	activePeersSnapshot := append([]*Peer(nil), c.ActivePeers...)
	c.JobsChannel = make(chan *Job, len(activePeersSnapshot))
	c.mu.Unlock()
	c.ReadyChannel = make(chan struct{}, len(activePeersSnapshot))
	queueRound := 1
	initialized := false
	for {
		c.mu.Lock()
		queueSnapshot := append([]*Job(nil), c.Queue...)
		c.mu.Unlock()
		if len(queueSnapshot) == 0 {
			break
		}
		if !initialized {
			go func() {
				initialized = true
				for _, peer := range activePeersSnapshot {
					go c.Download(peer)
				}
			}()
		}
		<-c.ReadyChannel
		for _, job := range queueSnapshot {
			c.JobsChannel <- job
			c.mu.Lock()
			time.Sleep(time.Duration(500/len(c.ActivePeers)) * time.Millisecond)
			c.mu.Unlock()
		}
		for len(c.Queue) > 0 && time.Since(c.LastBlockAt) < c.RoundtripTimeout() {
			time.Sleep(100 * time.Millisecond)
		}
		queueRound++
	}
}
