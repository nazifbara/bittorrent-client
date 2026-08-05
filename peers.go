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

func (c *Client) TrackConnectedPeers(peers []*net.TCPAddr) {
	for {
		newConnected := make(chan ConnectedPeer)
		var wg sync.WaitGroup
		wg.Add(len(peers))

		go func() {
			for cp := range newConnected {
				c.AddConnectedPeer(cp)
				fmt.Println(c.connectedPeers)
			}
		}()

		for _, peer := range peers {
			p := *peer
			go func() {
				defer wg.Done()
				conn, err := connectToPeer(&p)
				if err == nil {
					newConnected <- ConnectedPeer{TCPConn: conn, TCPAddr: peer}
				}
			}()
		}

		wg.Wait()
		close(newConnected)
		time.Sleep(time.Second * 5)
	}
}
