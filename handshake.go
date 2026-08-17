package main

import (
	"io"
	"net"
)

func (c *Client) Handshake(peer *Peer) *Peer {
	if peer == nil {
		return nil
	}
	if _, err := peer.Write(buildHandShake(c.torrent.InfoHash[:])); err != nil {
		return nil
	}
	_, err := readHandshake(peer.TCPConn)
	if err != nil {
		return nil
	}
	return peer
}

func readHandshake(conn *net.TCPConn) ([]byte, error) {
	b := make([]byte, 68)
	_, err := io.ReadFull(conn, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
