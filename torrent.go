package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce        string
	InfoHash        [20]byte
	PieceHashes     [][20]byte
	PieceSize       uint64
	FinalPieceSize  uint64
	NumOfPieces     uint64
	NumOfBlocks     uint64
	ContentSize     uint64
	TotalDownloaded uint64
	Files           []FileDict
	Name            string
	AnnounceList    [][]string
}

type BencodeTorrent struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list,omitempty"`
	Comment      string     `bencode:"comment,omitempty"`
	CreatedBy    string     `bencode:"created by,omitempty"`
	CreationDate uint64     `bencode:"creation date,omitempty"`
	Info         InfoDict   `bencode:"info"`
}

// InfoDict represents the "info" dictionary within the torrent metainfo.
type InfoDict struct {
	Name        string     `bencode:"name"`
	PieceLength uint64     `bencode:"piece length"`
	Pieces      string     `bencode:"pieces"`
	Files       []FileDict `bencode:"files,omitempty"`
	Length      uint64     `bencode:"length,omitempty"` // For single-file torrents
}

// FileDict represents an individual file entry in multi-file torrents.
type FileDict struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

func (bt BencodeTorrent) ToTorrent() (Torrent, error) {
	var buf bytes.Buffer
	if err := bencode.Marshal(&buf, bt.Info); err != nil {
		return Torrent{}, err
	}
	infoHash := sha1.Sum(buf.Bytes())
	pieceHashes, err := getPieceHashes(bt.Info)
	if err != nil {
		return Torrent{}, err
	}
	contentSize := 0
	if len(bt.Info.Files) == 0 {
		contentSize = int(bt.Info.Length)
	} else {
		for _, file := range bt.Info.Files {
			contentSize += int(file.Length)
		}
	}
	finalPieceSize := int(contentSize) % int(bt.Info.PieceLength)
	numOfBlocks := contentSize / int(blockSize)
	if contentSize%int(blockSize) != 0 {
		numOfBlocks++
	}
	return Torrent{
		Name:           bt.Info.Name,
		Announce:       bt.Announce,
		InfoHash:       infoHash,
		NumOfPieces:    uint64(len(bt.Info.Pieces) / 20),
		ContentSize:    uint64(contentSize),
		AnnounceList:   bt.AnnounceList,
		PieceHashes:    pieceHashes,
		PieceSize:      bt.Info.PieceLength,
		Files:          bt.Info.Files,
		NumOfBlocks:    uint64(numOfBlocks),
		FinalPieceSize: uint64(finalPieceSize),
	}, nil
}

func getPieceHashes(info InfoDict) ([][20]byte, error) {
	// A SHA-1 hash is exactly 20 bytes long
	const hashSize = 20

	// Quick validation: the total length must be a multiple of 20
	if len(info.Pieces)%hashSize != 0 {
		return nil, fmt.Errorf("malformed pieces string: length %d is not a multiple of %d", len(info.Pieces), hashSize)
	}

	numPieces := len(info.Pieces) / hashSize
	hashes := make([][20]byte, 0, numPieces)

	// Loop through the string 20 bytes at a time
	for i := 0; i < len(info.Pieces); i += hashSize {
		var hash [20]byte

		// Copy 20 bytes from the string into the fixed-size array
		copy(hash[:], info.Pieces[i:i+hashSize])

		hashes = append(hashes, hash)
	}

	return hashes, nil
}

func openTorrent(filePath string) (*Torrent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return &Torrent{}, err
	}
	defer f.Close()
	var bt BencodeTorrent
	err = bencode.Unmarshal(f, &bt)
	if err != nil {
		return &Torrent{}, errors.New("malformed torrent bencode")
	}
	torrent, err := bt.ToTorrent()
	return &torrent, nil
}
