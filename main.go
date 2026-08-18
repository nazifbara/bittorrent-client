package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	torrentPath := flag.String("f", "", "path to the torrent file")
	trackerUrl := flag.String("t", "", "tracker url")
	flag.Parse()
	code, err := run(*torrentPath, *trackerUrl)
	if err != nil {
		fmt.Println(err.Error())
	}
	os.Exit(code)
}

func run(torrentPath, trackerUrl string) (int, error) {
	if torrentPath == "" {
		return 1, errors.New("path to torrent file not provided")
	}
	torrent, err := openTorrent(torrentPath)
	if err != nil {
		return 1, err
	}
	announceList := torrent.announceList
	if trackerUrl != "" {
		announceList = [][]string{{trackerUrl}}
	}
	client := NewClient(torrent)
	var clientErr error
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		clientErr = client.Start(announceList)
		cancel()
	}()
	<-ctx.Done()
	client.shutdown()
	for _, f := range client.filesGrid {
		if f != nil {
			f.file.Close()
		}
	}
	if clientErr != nil {
		return 1, err
	}
	return 0, nil
}
