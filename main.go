package main

import (
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

type Torrent struct {
	Announce string
}

func main() {
	code, err := run()
	if err != nil {
		fmt.Println(err.Error())
	}
	os.Exit(code)
}

func run() (int, error) {
	torrent, err := openTorrent("ikigai.torrent")
	if err != nil {
		return 1, err
	}
	fmt.Println(torrent.Announce)
	return 0, nil
}

func openTorrent(filePath string) (Torrent, error) {
	torrent := Torrent{}
	f, err := os.Open(filePath)
	if err != nil {
		return torrent, err
	}
	defer f.Close()

	value, err := bencode.Decode(f)
	if err != nil {
		return torrent, err
	}
	root := value.(map[string]any)
	torrent.Announce = root["announce"].(string)
	return torrent, nil
}
