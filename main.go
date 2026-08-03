package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce string
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
	fmt.Println(torrent.Announce)
	return 0, nil
}

func openTorrent(filePath string) (Torrent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return Torrent{}, err
	}
	defer f.Close()

	value, err := bencode.Decode(f)
	if err != nil {
		return Torrent{}, err
	}
	root := value.(map[string]any)
	return Torrent{
		Announce: root["announce"].(string),
	}, nil
}
