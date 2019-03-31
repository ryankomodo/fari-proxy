package client

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/fari-proxy/encryption"
	"github.com/fari-proxy/service"
)

const maxBlock = 5000

type block struct {
	item map[string]int
	mu    *sync.RWMutex
}

type client struct {
	*service.Service
	*block
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
		&block{
			mu:		&sync.RWMutex{},
			item: 	make(map[string]int),
			},
	}
}

func (c *client) Listen() error {
	log.Printf("Server监听地址: %s:%d", c.RemoteAddr.IP, c.RemoteAddr.Port)
	listener, err := net.ListenTCP("tcp", c.ListenAddr)
	if err != nil {
		return err
	}
	log.Printf("Client启动成功, 监听地址: %s:%d, 密码: %s", c.ListenAddr.IP, c.ListenAddr.Port, c.Cipher.Password)


	defer listener.Close()

	for {
		userConn, err := listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		/* Discard any unsent or unacknowledged data. */
		userConn.SetLinger(0)
		go c.handleConn(userConn)
	}
	return nil
}

var proxyPool = make(chan *net.TCPConn, 10)

func init() {
	go func() {
		for range time.Tick(5 * time.Second) {
			p := <-proxyPool	/* Discard the idle connection */
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

func (c *client) directDial(userConn *net.TCPConn, dstAddr *net.TCPAddr) (*net.TCPConn, error){
	conn, errDial := net.DialTimeout("tcp", dstAddr.String(), time.Millisecond * 300)

	if errDial != nil {
		return &net.TCPConn{}, errDial
	} else {
		defer conn.Close()
		dstConn, errDialTCP := net.DialTCP("tcp", nil, dstAddr)
		if errDialTCP != nil {
			return &net.TCPConn{}, errDial
		} else {
			dstConn.SetLinger(0)
			errDialTCP = c.CustomWrite(userConn, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 10)
		}
		return dstConn, errDialTCP
	}
}

func (c *client) directConnect(userConn *net.TCPConn, dstConn *net.TCPConn) {
	go func() {
		err := c.Transfer(userConn, dstConn)
		if err != nil {
			userConn.Close()
			dstConn.Close()
		}
	}()
	c.Transfer(dstConn, userConn)
}

func (c *client) searchBlockList(ip string) bool {
	c.block.mu.RLock()
	defer c.block.mu.RUnlock()

	if _, ok := c.block.item[ip]; ok {
		return true
	} else{
		return false
	}
}

func (c *client) addBlockList(ip string) {
	c.block.mu.Lock()
	defer c.block.mu.Unlock()

	if len(c.block.item) > maxBlock {
		for ip := range c.block.item {
			delete(c.block.item, ip)
			break
		}
	}
	c.block.item[ip] = 1
}

func (c *client) tryProxy(userConn *net.TCPConn, lastUserRequest []byte) {
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

	_, errWrite := c.HttpEncode(proxy, lastUserRequest, service.CLIENT)
	if errWrite != nil {
		return
	}

	go func() {
		err := c.DecodeTransfer(userConn, proxy, service.CLIENT)
		if err != nil {
			userConn.Close()
			proxy.Close()
		}
	}()
	c.EncodeTransfer(proxy, userConn, service.CLIENT)
}

func (c *client) handleConn(userConn *net.TCPConn) {
	defer userConn.Close()

	dstAddr, lastUserRequest, errParse := c.ParseSOCKS5(userConn)
	if errParse != nil {
		return
	}

	block := c.searchBlockList(dstAddr.IP.String())
	if block {
		log.Printf("Can't directly connect to %s, Try to use Porxy", dstAddr.String())
		c.tryProxy(userConn, lastUserRequest)
	} else {
		dstConn, errDirect := c.directDial(userConn, dstAddr)
		if errDirect != nil {
			log.Printf("Can't directly connect to %s, Try to use Proxy and Put it into the block list", dstAddr.String())
			go c.addBlockList(dstAddr.IP.String())
			c.tryProxy(userConn, lastUserRequest)
		} else {
			log.Printf("Directly connect to %s", dstAddr.String())
			c.directConnect(userConn, dstConn)
		}
	}
}
