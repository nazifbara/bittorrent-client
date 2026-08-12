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

	peer := &Peer{TCPConn: tcpConn, TCPAddr: address}
	if err := c.Handshake(peer); err != nil {
		peer.Close()
		return nil, fmt.Errorf("handshake failed with %v", peer.TCPAddr)
	}
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

func (c *Client) HandlePeerSuccess(p *Peer) {
	_, existingPeer := c.getPeerByAddr(p.TCPAddr)
	if existingPeer == nil {
		p.Bitfield = make([]bool, c.Torrent.NumOfPieces)
		p.AliveAt = time.Now()
		c.ActivePeers = append(c.ActivePeers, p)
		return
	}
	existingPeer.AliveAt = time.Now()
	p.Close()
}

func (c *Client) HandlePeerFailure(address *net.TCPAddr) {
	index, peer := c.getPeerByAddr(address)
	if peer == nil {
		return
	}
	if time.Since(peer.AliveAt) > time.Minute*2 {
		peer.IsInactive = true
		c.ActivePeers = slices.Delete(c.ActivePeers, index, index+1)
	}
}

func (c *Client) HealthcheckPeers() {
	sem := make(chan struct{}, 10)
	handlingDownloadMap := make(map[string]bool)
	for {
		if c.Finished {
			break
		}
		if len(c.PeerAddresses) == 0 || (len(c.Queue) > 0 && len(c.ActivePeers) > 0) {
			continue
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
					if !handlingDownloadMap[peer.TCPAddr.String()] {
						handlingDownloadMap[peer.TCPAddr.String()] = true
						fmt.Printf("Downloading from %v\n", peer.TCPAddr)
						go c.Download(peer)
						c.HandlePeerSuccess(peer)
					}
				} else {
					c.HandlePeerFailure(&addr)
				}
			}()
		}
		wg.Wait()
		if len(c.ActivePeers) > 0 {
			fmt.Printf("%d active peers out of %d\n", len(c.ActivePeers), len(c.PeerAddresses)-len(c.ActivePeers))
		}
	}
}
