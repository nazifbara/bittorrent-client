package main

import (
	"io"
	"net"
)

func (c *Client) Handshake(peer *Peer) {
	if peer == nil {
		return
	}
	if _, err := peer.Write(buildHandShake(c.torrent.InfoHash[:])); err != nil {
		return
	}
	// fmt.Printf("handshake-->%v\n", peer.TCPAddr)
	_, err := readHandshake(peer.TCPConn)
	if err != nil {
		// fmt.Printf("handshake xxx %v\n", peer.TCPAddr)
		return
	}
	// fmt.Printf("%d<--handshake(%v)\n", len(n), peer.TCPAddr)
	c.mu.Lock()
	c.activePeers = append(c.activePeers, peer)
	c.mu.Unlock()
}

func readHandshake(conn *net.TCPConn) ([]byte, error) {
	b := make([]byte, 68)
	_, err := io.ReadFull(conn, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
