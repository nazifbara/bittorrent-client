package main

import (
	"fmt"
	"os"
)

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
	fmt.Println(string(torrent))
	return 0, nil
}

func openTorrent(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}
