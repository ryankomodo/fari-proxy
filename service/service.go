package service

import (
	"crypto/aes"
	"errors"
	"fmt"
	"github.com/fari-proxy/encryption"
	"github.com/fari-proxy/http"
	"io"
	"net"
)

const BUFFSIZE = 1024 * 2

var READBUFFERSIZE = BUFFSIZE + http.BodyLength + 8 // 8 is the lenght of http content, that is "Content-Length:"

type Service struct {
	ListenAddr *net.TCPAddr
	RemoteAddr *net.TCPAddr
	Cipher     *encryption.Cipher
}

// Decode
func (s *Service) Decode(conn *net.TCPConn, src []byte) (n int, err error) {
	var length int
	source := make([]byte, READBUFFERSIZE)
	nread, err := conn.Read(source)
	if nread == 0 || err != nil {
		return
	}
	for nread != READBUFFERSIZE {
		length, err = conn.Read(source[nread:])
		if err != nil {
			return
		}
		nread += length
	}
	// Parse http packet
	encrypted := http.ParseHttp(source)
	n = len(encrypted)
	iv := []byte(s.Cipher.Password)[:aes.BlockSize]
	(*s.Cipher).AesDecrypt(src[:n], encrypted, iv)
	return
}

// Encode
func (s *Service) Encode(conn *net.TCPConn, src []byte) (n int, err error) {
	iv := []byte(s.Cipher.Password)[:aes.BlockSize]
	encrypted := make([]byte, len(src))
	(*s.Cipher).AesEncrypt(encrypted, src, iv)

	// Wrap http packet
	httpMsg := http.NewHttp(encrypted)

  // If the size of packet less than the Buffer size, we need padding with 0x00
	if len(httpMsg) < READBUFFERSIZE {
		padding := make([]byte, READBUFFERSIZE-len(httpMsg))
		for i := range padding {
			padding[i] = 0x00
		}
		httpMsg = append(httpMsg, padding...)
	}
	return conn.Write(httpMsg)
}

// Read data from destination server or source server to the peer-end
func (s *Service) EncodeTransfer(dst *net.TCPConn, src *net.TCPConn) error {
	buf := make([]byte, BUFFSIZE)
	for {
		readCount, errRead := src.Read(buf)
		if errRead != nil {
			if errRead != io.EOF {
				return errRead
			} else {
				return nil
			}
		}
		if readCount > 0 {
			_, errWrite := s.Encode(dst, buf[0:readCount])
			if errWrite != nil {
				return errWrite
			}
		}
	}
}

// Read data from the the peer-end to destination server or source server
func (s *Service) DecodeTransfer(dst *net.TCPConn, src *net.TCPConn) error {
	buf := make([]byte, READBUFFERSIZE)
	for {
		readCount, errRead := s.Decode(src, buf)
		if errRead != nil {
			if errRead != io.EOF {
				return errRead
			} else {
				return nil
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

func (s *Service) DialRemote() (*net.TCPConn, error) {
	remoteConn, err := net.DialTCP("tcp", nil, s.RemoteAddr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("连接到远程服务器 %s 失败:%s", s.RemoteAddr, err))
	}
	return remoteConn, nil
}
