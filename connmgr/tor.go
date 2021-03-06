// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package connmgr

import (
	"encoding/binary"
	"errors"
	"net"

	"github.com/pkt-cash/pktd/btcutil/er"
)

const (
	torSucceeded         = 0x00
	torGeneralError      = 0x01
	torNotAllowed        = 0x02
	torNetUnreachable    = 0x03
	torHostUnreachable   = 0x04
	torConnectionRefused = 0x05
	torTTLExpired        = 0x06
	torCmdNotSupported   = 0x07
	torAddrNotSupported  = 0x08
)

var (
	// ErrTorInvalidAddressResponse indicates an invalid address was
	// returned by the Tor DNS resolver.
	ErrTorInvalidAddressResponse = errors.New("invalid address response")

	// ErrTorInvalidProxyResponse indicates the Tor proxy returned a
	// response in an unexpected format.
	ErrTorInvalidProxyResponse = errors.New("invalid proxy response")

	// ErrTorUnrecognizedAuthMethod indicates the authentication method
	// provided is not recognized.
	ErrTorUnrecognizedAuthMethod = errors.New("invalid proxy authentication method")

	torStatusErrors = map[byte]er.R{
		torSucceeded:         er.New("tor succeeded"),
		torGeneralError:      er.New("tor general error"),
		torNotAllowed:        er.New("tor not allowed"),
		torNetUnreachable:    er.New("tor network is unreachable"),
		torHostUnreachable:   er.New("tor host is unreachable"),
		torConnectionRefused: er.New("tor connection refused"),
		torTTLExpired:        er.New("tor TTL expired"),
		torCmdNotSupported:   er.New("tor command not supported"),
		torAddrNotSupported:  er.New("tor address type not supported"),
	}
)

// TorLookupIP uses Tor to resolve DNS via the SOCKS extension they provide for
// resolution over the Tor network. Tor itself doesn't support ipv6 so this
// doesn't either.
func TorLookupIP(host, proxy string) ([]net.IP, er.R) {
	conn, err := net.Dial("tcp", proxy)
	if err != nil {
		return nil, er.E(err)
	}
	defer conn.Close()

	buf := []byte{'\x05', '\x01', '\x00'}
	_, err = conn.Write(buf)
	if err != nil {
		return nil, er.E(err)
	}

	buf = make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		return nil, er.E(err)
	}
	if buf[0] != '\x05' {
		return nil, er.E(ErrTorInvalidProxyResponse)
	}
	if buf[1] != '\x00' {
		return nil, er.E(ErrTorUnrecognizedAuthMethod)
	}

	buf = make([]byte, 7+len(host))
	buf[0] = 5      // protocol version
	buf[1] = '\xF0' // Tor Resolve
	buf[2] = 0      // reserved
	buf[3] = 3      // Tor Resolve
	buf[4] = byte(len(host))
	copy(buf[5:], host)
	buf[5+len(host)] = 0 // Port 0

	_, err = conn.Write(buf)
	if err != nil {
		return nil, er.E(err)
	}

	buf = make([]byte, 4)
	_, err = conn.Read(buf)
	if err != nil {
		return nil, er.E(err)
	}
	if buf[0] != 5 {
		return nil, er.E(ErrTorInvalidProxyResponse)
	}
	if buf[1] != 0 {
		if int(buf[1]) >= len(torStatusErrors) {
			return nil, er.E(ErrTorInvalidProxyResponse)
		} else if err := torStatusErrors[buf[1]]; err != nil {
			return nil, err
		}
		return nil, er.E(ErrTorInvalidProxyResponse)
	}
	if buf[3] != 1 {
		err := torStatusErrors[torGeneralError]
		return nil, err
	}

	buf = make([]byte, 4)
	bytes, err := conn.Read(buf)
	if err != nil {
		return nil, er.E(err)
	}
	if bytes != 4 {
		return nil, er.E(ErrTorInvalidAddressResponse)
	}

	r := binary.BigEndian.Uint32(buf)

	addr := make([]net.IP, 1)
	addr[0] = net.IPv4(byte(r>>24), byte(r>>16), byte(r>>8), byte(r))

	return addr, nil
}
