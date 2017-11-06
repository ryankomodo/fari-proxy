package client

import (
	"log"
	"net"
	"time"

	"github.com/fari-proxy/encryption"
	"github.com/fari-proxy/service"
)

type client struct {
	*service.Service
}

func NewClient(remote, listen, password string) *client {
	c := encryption.NewCipher([]byte(password))
	listenAddr, _ := net.ResolveTCPAddr("tcp", listen)
	remoteAddr, _ := net.ResolveTCPAddr("tcp", remote)
	return &client{
		&service.Service{
			Cipher:     c,
			ListenAddr: listenAddr,
			RemoteAddr: remoteAddr,
		},
	}
}

func (c *client) Listen() error {
	listener, err := net.ListenTCP("tcp", c.ListenAddr)
	if err != nil {
		return err
	}
	log.Printf("启动成功,监听在 %s:%d, 密码: %s", c.ListenAddr.IP, c.ListenAddr.Port, c.Cipher.Password)
	defer listener.Close()

	for {
		userConn, err := listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		userConn.SetLinger(0)
		go c.handleConn(userConn)
	}
	return nil
}

var proxyPool = make(chan *net.TCPConn, 10)

func init() {
	go func() {
		for range time.Tick(5 * time.Second) {
			p := <-proxyPool // drop a idel conn
			p.Close()
		}
	}()
}

func (c *client) newProxyConn() (*net.TCPConn, error) {
	if len(proxyPool) < 10 {
		go func() {
			for i := 0; i < 2; i++ {
				proxy, err := c.DialRemote()
				if err != nil {
					log.Println(err)
					return
				}
				proxyPool <- proxy
			}
		}()
	}

	select {
	case pc := <-proxyPool:
		return pc, nil
	case <-time.After(100 * time.Millisecond):
		return c.DialRemote()
	}

}

func (c *client) handleConn(userConn *net.TCPConn) {
	defer userConn.Close()

	proxy, err := c.newProxyConn()
	if err != nil {
		log.Println(err)
		proxy, err = c.newProxyConn()
		if err != nil {
			log.Println(err)
			return
		}
	}
	defer proxy.Close()

	proxy.SetLinger(0)

	go func() {
		err := c.DecodeTransfer(userConn, proxy)
		if err != nil {
			userConn.Close()
			proxy.Close()
		}
	}()
	c.EncodeTransfer(proxy, userConn)
}
