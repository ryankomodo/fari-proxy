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
	httpBody := NewHttp([]byte(plaintext))
	tcpAddr, _ := net.ResolveTCPAddr("tcp", serverAddr)

	conn, _ := net.DialTCP("tcp", nil, tcpAddr)

	conn.SetDeadline(time.Now().Add(1 * time.Minute))

	conn.Write([]byte(httpBody))

	reader := bufio.NewReader(conn)
	reply := make([]byte, 1024)
	reader.Read(reply)
}

func TestNewHttp(t *testing.T) {
	go httpClient(plaintext)
	httpServer()
}
