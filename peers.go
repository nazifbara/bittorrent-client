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
	if c.Handshake(existingPeer) == nil {
		return existingPeer, nil
	}
	if existingPeer != nil {
		existingPeer.Close()
	}
	conn, err := net.DialTimeout("tcp", address.String(), 3*time.Second)
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
	if time.Since(peer.lastConnected) > time.Second*30 {
		c.ActivePeers = slices.Delete(c.ActivePeers, index, index+1)
	}
}

func (c *Client) TrackActivePeers(addresses []*net.TCPAddr) {
	for {
		var wg sync.WaitGroup
		wg.Add(len(addresses))
		for _, addr := range addresses {
			addr := *addr
			go func() {
				defer wg.Done()
				peer, err := c.connectToPeer(&addr)
				if err == nil {
					c.HandlePeerSuccess(peer)
				} else {
					c.HandlePeerFailure(&addr)
				}
			}()
		}
		wg.Wait()
		fmt.Println(c.ActivePeers)
		time.Sleep(time.Second * 10)
	}
}
