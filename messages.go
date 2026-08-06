package main

import "encoding/binary"

func (c *Client) BuildHandShake() []byte {
	b := make([]byte, 0, 68)
	// pstrlen
	b = append(b, byte(19))
	// pstr
	b = append(b, []byte("BitTorrent protocol")...)
	// reserved
	b = binary.BigEndian.AppendUint64(b, 0)
	// info hash
	b = append(b, c.Torrent.InfoHash[:]...)
	// peer id
	b = append(b, randomPeerID()...)
	return b
}
