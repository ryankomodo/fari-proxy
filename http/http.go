package http

import (
	"bytes"
	"strconv"
)

var httpBody = []byte("GET /blog.html HTTP/1.1\r\n" +
	"Accept:image/gif.image/jpeg,*/*\r\n" +
	"Accept-Language:zh-cn\r\n" +
	"Connection:Keep-Alive\r\n" +
	"Host:localhost\r\n" +
	"User-Agent:Mozila/4.0(compatible;MSIE5.01;Window NT5.0)\r\n" +
	"Accept-Encoding:gzip,deflate\r\n" +
	"Content-Length:")

var bodyLength = len(httpBody)
var ctrf []byte = []byte("\r\n\r\n")
var ctrfLength = len(ctrf)

func NewHttp(ciphertext []byte) []byte {
	httpBody := append(httpBody, []byte(strconv.Itoa(len(ciphertext)))...)
	httpBody = append(httpBody, ctrf...)
	httpBody = append(httpBody, ciphertext...)
	return httpBody
}

func ParseHttp(msg []byte) []byte {
	header := bytes.Split(msg, []byte("\r\n"))
	//fmt.Printf("%d\r\n", len(header))
	lengthName := bytes.Split(header[7], []byte(":"))[0]
	length := bytes.Split(header[7], []byte(":"))[1]
	if string(lengthName) == "Content-Length" {
		contentLength, _ := strconv.Atoi(string(length))
		return msg[bodyLength+len(length)+ctrfLength : bodyLength+len(length)+ctrfLength+contentLength]
	}
	// TODO check more
	return msg
}
