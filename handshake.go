package main

import (
	"errors"
	"fmt"
	"io"
	"net"
)

func (c *Client) Handshake(peer *Peer) error {
	if peer == nil {
		return errors.New("peer is nil")
	}
	if _, err := peer.Write(c.BuildHandShake()); err != nil {
		return err
	}
	fmt.Printf("handshake-->%v\n", peer.TCPAddr)
	resp, err := readHandshake(peer.TCPConn)
	if err != nil {
		fmt.Printf("handshake xxx %v\n", peer.TCPAddr)
		return err
	}
	fmt.Printf("%d<--handshake(%v)\n", len(resp), peer.TCPAddr)
	return nil
}

func readHandshake(conn *net.TCPConn) ([]byte, error) {
	b := make([]byte, 68)
	_, err := io.ReadFull(conn, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
