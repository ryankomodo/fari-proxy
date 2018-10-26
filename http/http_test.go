package http

import (
	"bufio"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"
	"time"
)

var serverAddr = "127.0.0.1:20010"
var plaintext = "uzon57jd0v869t7w"
var incompleteHttpBody = []byte("GET /blog.html HTTP/1.1\r\n" +
	"Accept:image/gif.image/jpeg,*/*\r\n" +
	"Accept-Language:zh-cn\r\n" +
	"Connection:Keep-Alive\r\n")


func handle(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Print("test failed")
	} else {
		if string(body) == plaintext {
			log.Print("test success")
		} else {
			log.Print("test failed")
		}
	}
}

func httpServer() {
	http.HandleFunc("/blog.html", handle)
	err := http.ListenAndServe(serverAddr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func httpClient(plaintext string) {
	httpBody := NewHttpRequest([]byte(plaintext))
	tcpAddr, _ := net.ResolveTCPAddr("tcp", serverAddr)

	conn, _ := net.DialTCP("tcp", nil, tcpAddr)

	conn.SetDeadline(time.Now().Add(1 * time.Minute))

	conn.Write([]byte(httpBody))

	reader := bufio.NewReader(conn)
	reply := make([]byte, 1024)
	reader.Read(reply)
}


func TestGMT(t *testing.T) {
	timestamp := GMT()
	if "GMT" != timestamp[len(timestamp) -3:len(timestamp)] {
		log.Print("GMT() is invalid")
	} else {
		log.Print("GMT() is valid")
	}
}

func TestNewHttp(t *testing.T) {
	go httpClient(plaintext)
	httpServer()
}


func TestParseHttp(t *testing.T) {
	// empty http request
	msg := make([]byte, 0)
	msg = ParseHttpRequest(msg)
	if len(msg) == 0 {
		log.Print("empty http request test success")
	} else {
		log.Print("empty http request test failed")
	}

	// incomplete http request
	msg = ParseHttpRequest(incompleteHttpBody)
	if len(msg) == 0 {
		log.Print("incomplete http request test success")
	} else {
		log.Print("incomplete http request test failed")
	}

	// complete http request
	msg = NewHttpRequest([]byte(plaintext))
	msg = ParseHttpRequest(msg)
	if string(msg) == plaintext {
		log.Print("complete http request test success")
	} else {
		log.Print("complete http request test failed")
	}
}
