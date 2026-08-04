package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce string `bencode:"announce"`
}

func main() {
	torrentPath := flag.String("f", "", "path to the torrent file")
	flag.Parse()
	code, err := run(*torrentPath)
	if err != nil {
		fmt.Println(err.Error())
	}
	os.Exit(code)
}

func run(torrentPath string) (int, error) {
	if torrentPath == "" {
		return 1, errors.New("path to torrent file not provided")
	}
	torrent, err := openTorrent(torrentPath)
	if err != nil {
		return 1, err
	}
	conn, err := createUDPConn(torrent)
	if err != nil {
		return 1, err
	}
	defer conn.Close()

	getPeers(conn)
	return 0, nil
}

func createUDPConn(torrent Torrent) (*net.UDPConn, error) {
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
	return conn, nil
}

func openTorrent(filePath string) (Torrent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return Torrent{}, err
	}
	defer f.Close()
	var torrent Torrent
	err = bencode.Unmarshal(f, &torrent)
	if err != nil {
		return Torrent{}, errors.New("malformed torrent bencode")
	}
	return torrent, nil
}
