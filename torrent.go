package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce     string
	InfoHash     [20]byte
	PieceHashes  [][20]byte
	PieceLength  int
	Length       uint64
	Name         string
	AnnounceList [][]string
}

type BencodeTorrent struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list,omitempty"`
	Comment      string     `bencode:"comment,omitempty"`
	CreatedBy    string     `bencode:"created by,omitempty"`
	CreationDate int64      `bencode:"creation date,omitempty"`
	Info         InfoDict   `bencode:"info"`
}

func (bt BencodeTorrent) ToTorrent() (Torrent, error) {
	var buf bytes.Buffer
	if err := bencode.Marshal(&buf, bt.Info); err != nil {
		return Torrent{}, err
	}
	infoHash := sha1.Sum(buf.Bytes())
	return Torrent{
		Announce:     bt.Announce,
		InfoHash:     infoHash,
		Length:       bt.Info.Length,
		AnnounceList: bt.AnnounceList,
	}, nil
}

// InfoDict represents the "info" dictionary within the torrent metainfo.
type InfoDict struct {
	Name        string     `bencode:"name"`
	PieceLength int64      `bencode:"piece length"`
	Pieces      string     `bencode:"pieces"`
	Files       []FileDict `bencode:"files,omitempty"`
	Length      uint64     `bencode:"length,omitempty"` // For single-file torrents
}

// FileDict represents an individual file entry in multi-file torrents.
type FileDict struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
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
