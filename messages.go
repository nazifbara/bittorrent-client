package main

import (
	"encoding/binary"
	"errors"
)

type Message struct {
	size    uint32
	id      uint8
	payload []byte
}

type HavePayload struct {
	Index uint32
}

type BitfieldPayload struct {
	Bitfield []bool
}

type PortPayload struct {
	Port uint16
}

type RequestCancelPayload struct {
	Index  uint32
	Begin  uint32
	Length uint32
}

type BlockPayload struct {
	Index uint32
	Begin uint32
	Block []byte
}

var payloadSizeErr = errors.New("incorrect payload size")

func parseMessage(b []byte) (Message, error) {
	if len(b) < 4 {
		return Message{}, errors.New("message unexpectedly short")
	}

	message := Message{size: binary.BigEndian.Uint32(b[:4])}
	if message.size == 0 {
		return message, nil
	}

	payloadLength := int(message.size) - 1
	minSize := 4 + 1 + payloadLength
	if len(b) < minSize {
		return Message{size: message.size}, errors.New("message unexpectedly short")
	}

	message.id = uint8(b[4])
	message.payload = b[5 : 5+payloadLength]
	return message, nil
}

func parseRequestCancelPayload(b []byte) (RequestCancelPayload, error) {
	if len(b) != 12 {
		return RequestCancelPayload{}, payloadSizeErr
	}
	return RequestCancelPayload{
		Index:  binary.BigEndian.Uint32(b[:4]),
		Begin:  binary.BigEndian.Uint32(b[4:8]),
		Length: binary.BigEndian.Uint32(b[8:]),
	}, nil
}

func parseBlockPayload(b []byte) (BlockPayload, error) {
	if len(b) < 8 {
		return BlockPayload{}, payloadSizeErr
	}
	return BlockPayload{
		Index: binary.BigEndian.Uint32(b[:4]),
		Begin: binary.BigEndian.Uint32(b[4:8]),
		Block: b[8:],
	}, nil
}

func parsePortPayload(b []byte) (PortPayload, error) {
	if len(b) != 2 {
		return PortPayload{}, payloadSizeErr
	}
	return PortPayload{Port: binary.BigEndian.Uint16(b)}, nil
}

func parseBitfieldPayload(b []byte) BitfieldPayload {
	return BitfieldPayload{Bitfield: bytesToBits(b)}
}

func parseHavePayload(b []byte) (HavePayload, error) {
	if len(b) != 4 {
		return HavePayload{}, payloadSizeErr
	}
	return HavePayload{Index: binary.BigEndian.Uint32(b)}, nil
}

func (c *Client) buildBlockSize(pieceIdx uint32, currentBegin uint32) uint32 {
	totalPieceSize := uint32(c.torrent.PieceSize)
	if pieceIdx == uint32(c.torrent.NumOfPieces-1) {
		totalPieceSize = uint32(c.torrent.FinalPieceSize)
	}
	rem := totalPieceSize - currentBegin
	if rem < blockSize {
		return rem
	}
	return blockSize
}

func buildHandShake(infohash []byte) []byte {
	b := make([]byte, 0, 68)
	// pstrlen
	b = append(b, byte(19))
	// pstr
	b = append(b, []byte("BitTorrent protocol")...)
	// reserved
	b = binary.BigEndian.AppendUint64(b, 0)
	// info hash
	b = append(b, infohash...)
	// peer id
	b = append(b, randomPeerID()...)
	return b
}

func buildKeepAlive() []byte {
	return make([]byte, 4)
}

func buildChoke() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 0)
	return b
}

func buildUnchoke() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 1)
	return b
}

func buildInterested() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 2)
	return b
}

func buildUninterested() []byte {
	b := make([]byte, 0, 5)
	// length
	b = binary.BigEndian.AppendUint32(b, 1)
	// message id
	b = append(b, 3)
	return b
}

func buildHave(pieceIdx uint32) []byte {
	b := make([]byte, 0, 9)
	// length
	b = binary.BigEndian.AppendUint32(b, 5)
	// message id
	b = append(b, 4)
	// piece index
	b = binary.BigEndian.AppendUint32(b, pieceIdx)
	return b
}

func buildBitfield(payload []bool) ([]byte, error) {
	bitBytes, err := bitsToBytes(payload)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 0, 5+len(bitBytes))
	b = binary.BigEndian.AppendUint32(b, uint32(1+len(bitBytes)))
	b = append(b, 5)
	b = append(b, bitBytes...)
	return b, nil
}

func buildRequest(pieceIdx, begin, pieceLength uint32) []byte {
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

func buildPiece(pieceIdx, begin uint32, block []byte) []byte {
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

func buildCancel(pieceIdx, begin, pieceLength uint32) []byte {
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

func buildPort(port uint16) []byte {
	b := make([]byte, 0, 7)
	// length
	b = binary.BigEndian.AppendUint32(b, 3)
	// message id
	b = append(b, 9)
	// port
	b = binary.BigEndian.AppendUint16(b, port)
	return b
}
