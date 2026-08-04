package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
)

type ConnResp struct {
	Action        uint32
	TransactionID uint32
	ConnectionID  uint64
}

func getPeers(conn *net.UDPConn) error {
	connResp, err := requestConn(conn)
	if err != nil {
		return err
	}
	fmt.Println(connResp)
	return nil
}

func requestConn(conn *net.UDPConn) (ConnResp, error) {
	respBuffer := make([]byte, 1024)
	if _, err := conn.Write(buildConnReq()); err != nil {
		return ConnResp{}, err
	}
	n, err := conn.Read(respBuffer)
	if n < 16 {
		return ConnResp{}, fmt.Errorf("tracker response is too short: %d", n)
	}
	if err != nil {
		return ConnResp{}, err
	}
	return parseConnRes(respBuffer[:n]), nil
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

func parseConnRes(resp []byte) ConnResp {
	return ConnResp{
		Action:        binary.BigEndian.Uint32(resp[0:4]),
		TransactionID: binary.BigEndian.Uint32(resp[4:8]),
		ConnectionID:  binary.BigEndian.Uint64(resp[8:]),
	}
}
