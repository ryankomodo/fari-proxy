package service

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/fari-proxy/encryption"
	"github.com/fari-proxy/http"
)

const BUFFSIZE = 1024 * 2

var REQUESTBUFFSIZE = BUFFSIZE + http.RequestLength + 8 // 8 is the lenght of http content, that is "Content-Length:"
var RESPONSEBUFFSIZE = BUFFSIZE + http.ResponseLength + 8

type Type int

const (
	SERVER = iota
	CLIENT
)

type Service struct {
	ListenAddr *net.TCPAddr
	RemoteAddr *net.TCPAddr
	Cipher     *encryption.Cipher
}


func (s *Service) HttpDecode(conn *net.TCPConn, src []byte, cs Type) (n int, err error) {
	var length, buf_len int
	if cs == SERVER {
		buf_len = REQUESTBUFFSIZE
	} else {
		buf_len = RESPONSEBUFFSIZE
	}

	source := make([]byte, buf_len)
	nread, err := conn.Read(source)
	if nread == 0 || err != nil {
		return
	}
	for nread != buf_len {
		length, err = conn.Read(source[nread:])
		if err != nil {
			return
		}
		nread += length
	}

	var encrypted []byte
	// Parsing packet
	if cs == SERVER {
		encrypted = http.ParseHttpRequest(source)
	} else {
		encrypted = http.ParseHttpResponse(source)
	}

	n = len(encrypted)
	iv := []byte(s.Cipher.Password)[:aes.BlockSize]
	(*s.Cipher).AesDecrypt(src[:n], encrypted, iv)
	return
}


// Warping the http packet with data
func (s *Service) HttpEncode(conn *net.TCPConn, src []byte, cs Type) (n int, err error) {
	iv := []byte(s.Cipher.Password)[:aes.BlockSize]
	encrypted := make([]byte, len(src))
	(*s.Cipher).AesEncrypt(encrypted, src, iv)

	var httpMsg []byte
	var buf_len int
	if cs == SERVER {
		httpMsg = http.NewHttpResponse(encrypted)
		buf_len = RESPONSEBUFFSIZE
	} else {
		httpMsg = http.NewHttpRequest(encrypted)
		buf_len = REQUESTBUFFSIZE
	}

	// Padding with 0x00
	if len(httpMsg) <  buf_len {
		padding := make([]byte, buf_len-len(httpMsg))
		for i := range padding {
			padding[i] = 0x00
		}
		httpMsg = append(httpMsg, padding...)
	}
	return conn.Write(httpMsg)
}


func (s *Service) EncodeTransfer(dst *net.TCPConn, src *net.TCPConn, cs Type) error {
	buf := make([]byte, BUFFSIZE)

	for {
		readCount, errRead := src.Read(buf)
		if errRead != nil {
			if errRead != io.EOF {
				return nil
			} else {
				return errRead
			}
		}
		if readCount > 0 {
			_, errWrite := s.HttpEncode(dst, buf[0:readCount], cs)
			if errWrite != nil {
				return errWrite
			}
		}
	}
}


func (s *Service) DecodeTransfer(dst *net.TCPConn, src *net.TCPConn, cs Type) error {
	var buf_len int
	if cs == SERVER {
		buf_len = REQUESTBUFFSIZE
	} else {
		buf_len = RESPONSEBUFFSIZE
	}
	buf := make([]byte, buf_len)

	for {
		readCount, errRead := s.HttpDecode(src, buf, cs)
		if errRead != nil {
			if errRead != io.EOF {
				return nil
			} else {
				return errRead
			}
		}
		if readCount > 0 {
			writeCount, errWrite := dst.Write(buf[0:readCount])
			if errWrite != nil {
				return errWrite
			}
			if readCount != writeCount {
				return io.ErrShortWrite
			}
		}
	}
}


func (s *Service) Transfer(srcConn *net.TCPConn, dstConn *net.TCPConn) error {
	buf := make([]byte, BUFFSIZE * 2)
	for {
		readCount, errRead := srcConn.Read(buf)
		if errRead != nil {
			if errRead != io.EOF {
				return nil
			} else {
				return errRead
			}
		}
		if readCount > 0 {
			_, errWrite := dstConn.Write(buf[0:readCount])
			if errWrite != nil {
				return errWrite
			}
		}
	}
}

func (s *Service) DialRemote() (*net.TCPConn, error) {
	remoteConn, err := net.DialTCP("tcp", nil, s.RemoteAddr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("连接到远程服务器 %s 失败:%s", s.RemoteAddr, err))
	}
	return remoteConn, nil
}


func (s *Service) CustomRead(userConn *net.TCPConn, buf [] byte) (int, error) {
	readCount, errRead := userConn.Read(buf)
	if errRead != nil {
		if errRead != io.EOF {
			return readCount, nil
		} else {
			return readCount, errRead
		}
	}
	return readCount, nil
}

func (s *Service) CustomWrite(userConn *net.TCPConn, buf [] byte, bufLen int) error {
	writeCount, errWrite := userConn.Write(buf)
	if errWrite != nil {
		return errWrite
	}
	if bufLen != writeCount {
		return io.ErrShortWrite
	}
	return nil
}

func (s *Service) ParseSOCKS5(userConn *net.TCPConn) (*net.TCPAddr, []byte, error){
	buf := make([]byte, BUFFSIZE)

	readCount, errRead := s.CustomRead(userConn, buf)
	if readCount > 0 && errRead == nil {
		if (buf[0] != 0x05) {
			return &net.TCPAddr{}, nil, errors.New("Only Support SOCKS5")
		} else {
			// Send to client 0x05,0x00 [version, method]
			errWrite := s.CustomWrite(userConn, []byte{0x05, 0x00}, 2)
			if errWrite != nil {
				return &net.TCPAddr{}, nil, errors.New("Send the version and method failed")
			}
		}
	}

	readCount, errRead = s.CustomRead(userConn, buf)
	if readCount > 0 && errRead == nil {
		if buf[1] != 0x01 { // Only support connect
			return &net.TCPAddr{}, nil, errors.New("Only support connect method")
		}

		// Parsing destination addr and port
		var desIP []byte
		switch buf[3] {
		case 0x01:
			desIP = buf[4 : 4+net.IPv4len]
		case 0x03:
			ipAddr, err := net.ResolveIPAddr("ip", string(buf[5:readCount-2]))
			if err != nil {
				return &net.TCPAddr{}, nil, errors.New("Parse IP failed")
			}
			desIP = ipAddr.IP
		case 0x04:
			desIP = buf[4 : 4+net.IPv6len]
		default:
			return &net.TCPAddr{}, nil, errors.New("Not support address")
		}
		dstPort := buf[readCount-2 : readCount]
		dstAddr := &net.TCPAddr{
			IP:   desIP,
			Port: int(binary.BigEndian.Uint16(dstPort)),
		}
		return dstAddr, buf[:readCount], errRead
	}
	return &net.TCPAddr{}, nil, errRead
}