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

func (c *Client) BuildRequest(pieceIdx, begin, pieceLength uint32) []byte {
	b := make([]byte, 0, 17)
	// length
	b = binary.BigEndian.AppendUint32(b, 13)
	// message id
	b = append(b, 6)
	// piece index
	b = binary.BigEndian.AppendUint32(b, pieceIdx)
	// begin
	b = binary.BigEndian.AppendUint32(b, begin)
	// piece length
	b = binary.BigEndian.AppendUint32(b, pieceLength)
	return b
}

func (c *Client) BuildPiece(pieceIdx, begin uint32, block []byte) []byte {
	blockSize := len(block)
	b := make([]byte, 0, 13+blockSize)
	// length
	b = binary.BigEndian.AppendUint32(b, uint32(9+blockSize))
	// message id
	b = append(b, 7)
	// piece index
	b = binary.BigEndian.AppendUint32(b, pieceIdx)
	// begin
	b = binary.BigEndian.AppendUint32(b, begin)
	// block
	b = append(b, block...)
	return b
}

func (c *Client) BuildCancel(pieceIdx, begin, pieceLength uint32) []byte {
	b := make([]byte, 0, 17)
	// length
	b = binary.BigEndian.AppendUint32(b, 13)
	// message id
	b = append(b, 8)
	// piece index
	b = binary.BigEndian.AppendUint32(b, pieceIdx)
	// begin
	b = binary.BigEndian.AppendUint32(b, begin)
	// piece length
	b = binary.BigEndian.AppendUint32(b, pieceLength)
	return b
}

func (c *Client) BuildPort(port uint16) []byte {
	b := make([]byte, 0, 7)
	// length
	b = binary.BigEndian.AppendUint32(b, 3)
	// message id
	b = append(b, 9)
	// port
	b = binary.BigEndian.AppendUint16(b, port)
	return b
}
