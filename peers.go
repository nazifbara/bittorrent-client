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

func (c *Client) AddConnectedPeer(p *Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := slices.IndexFunc(c.ActivePeers, func(ap *Peer) bool { return p.IP.Equal(ap.IP) })
	if index == -1 {
		c.ActivePeers = append(c.ActivePeers, p)
	}
}

func (c *Client) DeleteConnectedPeer(address *net.TCPAddr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ActivePeers = slices.DeleteFunc(c.ActivePeers, func(p *Peer) bool { return p.IP.Equal(address.IP) })
}

func (c *Client) TrackConnectedPeers(addresses []*net.TCPAddr) {
	for {
		var wg sync.WaitGroup
		wg.Add(len(addresses))
		for _, peer := range addresses {
			p := *peer
			go func() {
				defer wg.Done()
				conn, err := connectToPeer(&p)
				if err == nil {
					c.AddConnectedPeer(&Peer{TCPConn: conn, TCPAddr: &p})
				} else {
					c.DeleteConnectedPeer(&p)
				}
			}()
		}
		wg.Wait()
		fmt.Println(c.ActivePeers)
		time.Sleep(time.Second * 10)
	}
}
