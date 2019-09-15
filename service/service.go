package service

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

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
	RemoteAddrs []*net.TCPAddr
	StablePorxy *net.TCPAddr
	Cipher     *encryption.Cipher
}


func (s *Service) HttpDecode(conn *net.TCPConn, src []byte, cs Type) (n int, err error) {
	var length, bufLen int
	if cs == SERVER {
		bufLen = REQUESTBUFFSIZE
	} else {
		bufLen = RESPONSEBUFFSIZE
	}

	source := make([]byte, bufLen)
	nRead, err := conn.Read(source)
	if nRead == 0 || err != nil {
		return
	}
	for nRead != bufLen {
		length, err = conn.Read(source[nRead:])
		if err != nil {
			return
		}
		nRead += length
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
	var bufLen int
	if cs == SERVER {
		httpMsg = http.NewHttpResponse(encrypted)
		bufLen = RESPONSEBUFFSIZE
	} else {
		httpMsg = http.NewHttpRequest(encrypted)
		bufLen = REQUESTBUFFSIZE
	}

	// Padding with 0x00
	if len(httpMsg) <  bufLen {
		padding := make([]byte, bufLen-len(httpMsg))
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
	var bufLen int
	if cs == SERVER {
		bufLen = REQUESTBUFFSIZE
	} else {
		bufLen = RESPONSEBUFFSIZE
	}
	buf := make([]byte, bufLen)

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
	d := net.Dialer{Timeout: 5 * time.Second}
	remoteConn, err := d.Dial("tcp", s.StablePorxy.String())
	if err != nil {
		log.Printf("连接到远程服务器 %s 失败:%s", s.StablePorxy.String(), err)

		// Try other proxies
		for _, proxy := range s.RemoteAddrs {
			log.Printf("尝试其他远程服务器: %s", proxy.String())
			remoteConn, err := d.Dial("tcp", proxy.String())
			if err == nil {
				s.StablePorxy = proxy
				tcpConn, _ := remoteConn.(*net.TCPConn)
				return tcpConn, nil

			}
		}
		return nil, errors.New(fmt.Sprintf("所有远程服务器连接均失败"))
	}
	log.Printf("连接到远程服务器 %s 成功", s.StablePorxy.String())
	tcpConn, _ := remoteConn.(*net.TCPConn)
	return tcpConn, nil
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
		if buf[0] != 0x05 {
			/* Version Number */
			return &net.TCPAddr{}, nil, errors.New("Only Support SOCKS5")
		} else {
			/* [SOCKS5, NO AUTHENTICATION REQUIRED]  */
			errWrite := s.CustomWrite(userConn, []byte{0x05, 0x00}, 2)
			if errWrite != nil {
				return &net.TCPAddr{}, nil, errors.New("Response SOCKS5 failed at the first stage.")
			}
		}
	}

	readCount, errRead = s.CustomRead(userConn, buf)
	if readCount > 0 && errRead == nil {
		if buf[1] != 0x01 {
			/* Only support CONNECT*/
			return &net.TCPAddr{}, nil, errors.New("Only support CONNECT and UDP ASSOCIATE method.")
		}

		var desIP []byte
		switch buf[3] { /* checking ATYPE */
		case 0x01: /* IPv4 */
			desIP = buf[4 : 4+net.IPv4len]
		case 0x03: /* DOMAINNAME */
			ipAddr, err := net.ResolveIPAddr("ip", string(buf[5:readCount-2]))
			if err != nil {
				return &net.TCPAddr{}, nil, errors.New("Parse IP failed")
			}
			desIP = ipAddr.IP
		case 0x04: /* IPV6 */
			desIP = buf[4 : 4+net.IPv6len]
		default:
			return &net.TCPAddr{}, nil, errors.New("Wrong DST.ADDR and DST.PORT")
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