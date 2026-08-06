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

func (c *Client) BuildKeepAlive() []byte {
	return make([]byte, 4)
}

func (c *Client) BuildChoke() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 0)
	return b
}

func (c *Client) BuildUnchoke() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 1)
	return b
}

func (c *Client) BuildInterested() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 2)
	return b
}

func (c *Client) BuildUninterested() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 3)
	return b
}

func (c *Client) BuildHave(pieceIdx uint32) []byte {
	b := make([]byte, 0, 9)
	// length
	b = binary.BigEndian.AppendUint32(b, 5)
	// message id
	b = append(b, 4)
	// piece index
	b = binary.BigEndian.AppendUint32(b, pieceIdx)
	return b
}

func (c *Client) BuildBitfield(payload []byte) []byte {
	payloadSize := len(payload)
	// 4 bytes length + 1 byte message id + payload size
	b := make([]byte, 0, 5+payloadSize)
	// length
	b = binary.BigEndian.AppendUint32(b, uint32(1+payloadSize))
	// message id
	b = append(b, 5)
	// bitfield
	b = append(b, payload...)
	return b
}
