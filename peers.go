package main

import (
	"errors"
	"fmt"
	"net"
	"slices"
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

	peer := &Peer{TCPConn: tcpConn, TCPAddr: address, Bitfield: make([]bool, c.torrent.numOfPieces), rtt: *newRTTTracker(100)}

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
	c.addrQeue = make(chan *net.TCPAddr, len(c.peerAddresses))
	for _, addr := range c.peerAddresses {
		c.addrQeue <- addr
	}

	for i := range c.piecesGrid {
		c.addPieceJobs(uint32(i))
	}

	go func() {
		initialized := false
		time.AfterFunc(30*time.Second, func() {
			initialized = true
		})
		for {
			select {
			case <-c.done:
				return
			case addr, ok := <-c.addrQeue:
				if ok {
					peer, err := c.connectToPeer(addr)
					if err != nil {
						c.addrQeue <- addr
						continue
					}
					peer = c.Handshake(peer)
					if peer == nil {
						c.addrQeue <- addr
						continue
					}
					peer.done = make(chan struct{})
					if _, err := peer.Write(buildInterested()); err != nil {
						c.addrQeue <- addr
						continue
					}
					go c.readPeerMessages(peer)
				}
			}
			if initialized {
				time.Sleep(2 * time.Minute)
			}
		}
	}()

	<-c.done
	c.shutdown()
	c.mu.Lock()
	for _, peer := range c.activePeers {
		peer.Close()
	}
	c.mu.Unlock()
	return nil
}
