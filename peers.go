package main

import (
	"fmt"
	"net"
	"slices"
	"sync"
	"time"
)

func connectToPeer(address *net.TCPAddr) (*net.TCPConn, error) {
	conn, err := net.DialTimeout("tcp", address.String(), 3*time.Second)
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("unexpected connection type")
	}
	return tcpConn, nil
}

func (c *Client) HandlePeerSuccess(p *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := slices.IndexFunc(c.ActivePeers, func(ap *Peer) bool {
		return p.IP.Equal(ap.IP) && p.Port == ap.Port
	})
	if index == -1 {
		p.lastConnected = time.Now()
		c.ActivePeers = append(c.ActivePeers, p)
		return
	}
	c.ActivePeers[index].lastConnected = time.Now()
	p.Close()
}

func (c *Client) HandlePeerFailure(address *net.TCPAddr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	peerIdx := slices.IndexFunc(c.ActivePeers, func(ac *Peer) bool { return ac.IP.Equal(address.IP) })
	if peerIdx == -1 {
		return
	}
	if time.Since(c.ActivePeers[peerIdx].lastConnected) > time.Second*30 {
		c.ActivePeers = slices.Delete(c.ActivePeers, peerIdx, peerIdx+1)
	}
}

func (c *Client) TrackConnectedPeers(addresses []*net.TCPAddr) {
	for {
		var wg sync.WaitGroup
		wg.Add(len(addresses))
		for _, addr := range addresses {
			addr := *addr
			go func() {
				defer wg.Done()
				conn, err := connectToPeer(&addr)
				if err == nil {
					c.HandlePeerSuccess(&Peer{TCPConn: conn, TCPAddr: &addr})
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
