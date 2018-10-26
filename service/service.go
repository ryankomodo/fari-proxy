package service

import (
	"crypto/aes"
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
		return nread, err
	}
	for nread != buf_len {
		length, err = conn.Read(source[nread:])
		if err != nil {
			if err != io.EOF {
				return nread, err
			}
		}
		nread += length
	}

	var encrypted []byte
	// Parsing http
	if cs == SERVER {
		encrypted = http.ParseHttpRequest(source)
	} else {
		encrypted = http.ParseHttpResponse(source)
	}

	n = len(encrypted)
	iv := []byte(s.Cipher.Password)[:aes.BlockSize]
	(*s.Cipher).AesDecrypt(src[:n], encrypted, iv)
	return n, err
}


//	Warping the http packet with data
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

	//	Padding with 0x00
	if len(httpMsg) <  buf_len{
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
				return errRead
			} else {
				return nil
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
	buf := make([]byte, REQUESTBUFFSIZE)
	for {
		readCount, errRead := s.HttpDecode(src, buf, cs)
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
