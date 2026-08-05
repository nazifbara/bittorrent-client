package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

type Client struct {
	Torrent     *Torrent
	TrackerConn *net.UDPConn
	TrackerAddr *net.UDPAddr
}

func newClient(torrent *Torrent, trackerConn *net.UDPConn, TrackerAddr *net.UDPAddr) Client {
	return Client{
		Torrent:     torrent,
		TrackerConn: trackerConn,
		TrackerAddr: TrackerAddr,
	}
}

func openUDP(torrent *Torrent) (*net.UDPConn, *net.UDPAddr, error) {
	addr, err := findTracker(torrent)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, nil, err
	}
	// conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return conn, addr, nil
}

func findTracker(torrent *Torrent) (*net.UDPAddr, error) {
	var trackerAddr *net.UDPAddr
	for _, announce := range torrent.AnnounceList {
		// fmt.Printf("checking: %s\n", announce[0])
		parsedURL, err := url.Parse(announce[0])
		if err != nil {
			continue
		}
		if parsedURL.Scheme != "udp" {
			continue
		}
		addr, err := net.ResolveUDPAddr("udp4", parsedURL.Host)
		if err != nil {
			continue
		}
		trackerAddr = addr
		fmt.Printf("found tracker: %s\n", parsedURL.Host)
		break
	}
	if trackerAddr == nil {
		return nil, errors.New("couldn't find a reachable UDP tracker")
	}
	return trackerAddr, nil
}
