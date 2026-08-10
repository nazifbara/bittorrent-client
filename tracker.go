package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
)

type ConnResp struct {
	Action        []byte
	TransactionID []byte
	ConnectionID  []byte
}

type AnnounceResp struct {
	Action        []byte
	TransactionID []byte
	Interval      []byte
	Leechers      uint32
	Seeders       uint32
	PeerAddresses []*net.TCPAddr
}

func (c *Client) GetPeerAddresses() ([]*net.TCPAddr, error) {
	connResp, err := retry(10, c.TrackerConn, func() (ConnResp, error) {
		connResp, err := c.RequestConn()
		if err != nil {
			return ConnResp{}, err
		}
		fmt.Println("connected to tracker")
		return connResp, nil
	})
	if err != nil {
		return nil, err
	}
	return retry(10, c.TrackerConn, func() ([]*net.TCPAddr, error) {
		peers, err := c.Announce(connResp)
		if err != nil {
			return []*net.TCPAddr{}, err
		}
		fmt.Println("announced!")
		return peers, nil
	})
}

func (c *Client) Announce(connResp ConnResp) ([]*net.TCPAddr, error) {
	if _, err := c.TrackerConn.Write(c.BuildAnnounceReq(connResp.ConnectionID)); err != nil {
		return []*net.TCPAddr{}, err
	}
	respBuffer := make([]byte, 1024)
	n, err := io.ReadFull(c.TrackerConn, respBuffer)
	if err != nil {
		return []*net.TCPAddr{}, err
	}
	announcResp := parseAnnounceResp(respBuffer[:n])
	return announcResp.PeerAddresses, nil
}

func (c *Client) RequestConn() (ConnResp, error) {
	respBuffer := make([]byte, 16)
	if _, err := c.TrackerConn.Write(buildConnReq()); err != nil {
		return ConnResp{}, err
	}
	n, err := io.ReadFull(c.TrackerConn, respBuffer)
	if err != nil {
		return ConnResp{}, err
	}
	if n < 16 {
		return ConnResp{}, fmt.Errorf("tracker response is too short: %d", n)
	}
	return parseConnResp(respBuffer[:n]), nil
}

func (c *Client) BuildAnnounceReq(connectID []byte) []byte {
	b := make([]byte, 0, 98)
	// connection id
	b = append(b, connectID...)
	// action 1 for announcement
	b = binary.BigEndian.AppendUint32(b, 1)
	// transaction id
	b = binary.BigEndian.AppendUint32(b, rand.Uint32())
	// infohash
	b = append(b, c.Torrent.InfoHash[:]...)
	// peer id
	b = append(b, randomPeerID()...)
	// downloaded
	b = binary.BigEndian.AppendUint64(b, 0)
	// left
	b = binary.BigEndian.AppendUint64(b, c.Torrent.Length)
	// uploaded
	b = binary.BigEndian.AppendUint64(b, 0)
	// event 0 none
	b = binary.BigEndian.AppendUint32(b, 0)
	// ip address 0 default
	b = binary.BigEndian.AppendUint32(b, 0)
	// key
	b = binary.BigEndian.AppendUint32(b, rand.Uint32())
	// num want
	b = binary.BigEndian.AppendUint32(b, uint32(0xFFFFFFFF))
	// port
	b = binary.BigEndian.AppendUint16(b, uint16(c.TrackerConn.LocalAddr().(*net.UDPAddr).Port))
	return b
}

func buildConnReq() []byte {
	b := make([]byte, 0, 16)
	// connection id
	b = binary.BigEndian.AppendUint64(b, 0x041727101980)
	// connect action
	b = binary.BigEndian.AppendUint32(b, 0)
	// transaction id
	b = binary.BigEndian.AppendUint32(b, rand.Uint32())
	return b
}

func parseAnnounceResp(resp []byte) AnnounceResp {
	peerAddresses := []*net.TCPAddr{}
	peersBytes := resp[20:]
	for i := 0; i+6 <= len(peersBytes); i += 6 {
		ipBytes := peersBytes[i : i+4]
		portBytes := peersBytes[i+4 : i+6]
		ip := net.IPv4(ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
		port := binary.BigEndian.Uint16(portBytes)
		peerAddresses = append(peerAddresses, &net.TCPAddr{IP: ip, Port: int(port)})
	}
	return AnnounceResp{
		Action:        resp[0:4],
		TransactionID: resp[4:8],
		Interval:      resp[8:12],
		Leechers:      binary.BigEndian.Uint32(resp[12:16]),
		Seeders:       binary.BigEndian.Uint32(resp[16:20]),
		PeerAddresses: peerAddresses,
	}
}

func parseConnResp(resp []byte) ConnResp {
	return ConnResp{
		Action:        resp[0:4],
		TransactionID: resp[4:8],
		ConnectionID:  resp[8:16],
	}
}

func connectToTracker(torrent *Torrent) (*net.UDPConn, *net.UDPAddr, error) {
	addr, err := findTracker(torrent)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, nil, err
	}
	return conn, addr, nil
}

func findTracker(torrent *Torrent) (*net.UDPAddr, error) {
	var trackerAddr *net.UDPAddr
	for _, announce := range torrent.AnnounceList {
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
