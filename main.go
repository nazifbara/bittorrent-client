package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

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
