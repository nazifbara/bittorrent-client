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
	conn, addr, err := connectToTracker(torrent)
	if err != nil {
		return 1, err
	}
	defer conn.Close()
	client := newClient(torrent, conn, addr)
	var clientErr error
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		clientErr = client.Start()
	}()
	<-ctx.Done()
	if clientErr != nil {
		return 1, err
	}
	return 0, nil
}
