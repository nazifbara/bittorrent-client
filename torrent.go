package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type BencodeTorrent struct {
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list,omitempty"`
	Comment      string     `bencode:"comment,omitempty"`
	CreatedBy    string     `bencode:"created by,omitempty"`
	CreationDate int64      `bencode:"creation date,omitempty"`
	Info         InfoDict   `bencode:"info"`
}

// InfoDict represents the "info" dictionary within the torrent metainfo.
type InfoDict struct {
	Name        string     `bencode:"name"`
	PieceLength int64      `bencode:"piece length"`
	Pieces      string     `bencode:"pieces"`
	Files       []FileDict `bencode:"files,omitempty"`
	Length      int64      `bencode:"length,omitempty"` // For single-file torrents
}

// FileDict represents an individual file entry in multi-file torrents.
type FileDict struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

func createUDPConn(torrent *Torrent) (*net.UDPConn, error) {
	parsedURL, err := url.Parse(torrent.Announce)
	if err != nil {
		return &net.UDPConn{}, err
	}
	if parsedURL.Scheme != "udp" {
		return nil, fmt.Errorf("unsupported announce scheme: %s", parsedURL.Scheme)
	}
	host := parsedURL.Host
	port := parsedURL.Port()
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, port))
	if err != nil {
		addr, err = net.ResolveUDPAddr("udp4", "open.stealth.si:80")
	}
	if err != nil {
		return &net.UDPConn{}, err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return &net.UDPConn{}, err
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return conn, nil
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
	fmt.Println(bt.AnnounceList)
	return &Torrent{Announce: bt.Announce}, nil
}
