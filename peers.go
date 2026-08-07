package main

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"
)

func (c *Client) connectToPeer(address *net.TCPAddr) (*Peer, error) {
	if address == nil {
		return nil, errors.New("address is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, existingPeer := c.getPeerByAddr(address)
	if existingPeer != nil {
		fmt.Printf("keep-alive-->%v\n", existingPeer.TCPAddr)
		if _, err := existingPeer.Write(c.BuildKeepAlive()); err != nil {
			fmt.Printf("keep-alive xxx %v\n", existingPeer.TCPAddr)
			return nil, fmt.Errorf("connection lost with %v", existingPeer.TCPAddr)
		}
		fmt.Printf("keep-alive<--%v\n", existingPeer.TCPAddr)
		return existingPeer, nil
	}
	conn, err := net.DialTimeout("tcp", address.String(), 300*time.Millisecond)
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("unexpected connection type")
	}
	peer := &Peer{TCPConn: tcpConn, TCPAddr: address}
	if err := c.Handshake(peer); err != nil {
		peer.Close()
		return nil, fmt.Errorf("handshake failed with %v", peer.TCPAddr)
	}
	return peer, nil
}

func (c *Client) getPeerByAddr(address *net.TCPAddr) (int, *Peer) {
	index := slices.IndexFunc(c.ActivePeers, func(ap *Peer) bool {
		return address.IP.Equal(ap.IP) && address.Port == ap.Port
	})
	if index == -1 {
		return index, nil
	}
	return index, c.ActivePeers[index]
}

func (c *Client) HandlePeerSuccess(p *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, existingPeer := c.getPeerByAddr(p.TCPAddr)
	if existingPeer == nil {
		p.lastConnected = time.Now()
		c.ActivePeers = append(c.ActivePeers, p)
		return
	}
	existingPeer.lastConnected = time.Now()
	p.Close()
}

func (c *Client) HandlePeerFailure(address *net.TCPAddr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index, peer := c.getPeerByAddr(address)
	if peer == nil {
		return
	}
	if time.Since(peer.lastConnected) > time.Minute*3 {
		c.ActivePeers = slices.Delete(c.ActivePeers, index, index+1)
	}
}

func (c *Client) HealthcheckPeers(addresses []*net.TCPAddr) {
	sem := make(chan struct{}, 10)
	for {
		var wg sync.WaitGroup
		wg.Add(len(addresses))
		for _, addr := range addresses {
			addr := *addr
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				peer, err := c.connectToPeer(&addr)
				if err == nil {
					c.HandlePeerSuccess(peer)
				} else {
					c.HandlePeerFailure(&addr)
				}
			}()
		}
		wg.Wait()
		fmt.Printf("active peers: %d\ninactive peers: %d\n", len(c.ActivePeers), len(addresses)-len(c.ActivePeers))
		time.Sleep(2 * time.Minute)
	}
}
