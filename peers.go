package main

import (
	"fmt"
	"net"
	"slices"
	"sync"
	"time"
)

func connectToPeer(peer *net.TCPAddr) (*net.TCPConn, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
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

func (c *Client) AddConnectedPeer(cp ConnectedPeer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	index := slices.IndexFunc(c.connectedPeers, func(p ConnectedPeer) bool { return p.IP.Equal(cp.IP) })
	if index == -1 {
		c.connectedPeers = append(c.connectedPeers, cp)
	}
}

func (c *Client) DeleteConnectedPeer(p net.TCPAddr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectedPeers = slices.DeleteFunc(c.connectedPeers, func(cp ConnectedPeer) bool { return cp.IP.Equal(p.IP) })
}

func (c *Client) TrackConnectedPeers(peers []*net.TCPAddr) {
	for {
		var wg sync.WaitGroup
		wg.Add(len(peers))
		for _, peer := range peers {
			p := *peer
			go func() {
				defer wg.Done()
				conn, err := connectToPeer(&p)
				if err == nil {
					c.AddConnectedPeer(ConnectedPeer{TCPConn: conn, TCPAddr: &p})
				} else {
					c.DeleteConnectedPeer(p)
				}
			}()
		}
		wg.Wait()
		fmt.Println(c.connectedPeers)
		time.Sleep(time.Second * 10)
	}
}
