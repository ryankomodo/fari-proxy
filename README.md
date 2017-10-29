# fari-proxy

[![Build Status](https://travis-ci.org/Leviathan1995/fari-proxy.svg?branch=master)](https://travis-ci.org/Leviathan1995/fari-proxy)

一个自由上网的代理工具, 将传输的数据加密包裹在HTTP报文, 伪装成简单的明文HTTP流量, 规避其他代理因为加密特征可能被嗅探的风险。

## 特点:

* 数据包使用`aes-cfb`加密
* 使用HTTP协议伪装数据包, 后续会支持自定义HTTP报文
* 对本地网络软件而言, 仍然是使用的SOCKS5代理, 与浏览器等软件无缝兼容
* 使用Supervisor后台运行管理
* 提供二进制可执行文件跨平台运行

## 使用方法:
请在[Release](https://github.com/Leviathan1995/fari-proxy/releases)页面下载合适的二进制可执行文件
* #### 在本地启动 `client`
	
		./client -c .client.json
	
* #### 在自由上网的服务器启动`server`
	
		./server -c .server.json
* #### 在本地开启SOCKS5的代理, 例如浏览器的SOCKS5插件

## 配置文件

* `.client.json`

		{
  			"remote_addr" : "127.0.0.1:20010",   远程服务器监听地址
  			"listen_addr" : "127.0.0.1:20011",   本地SOCKS5监听地址
  			"password" : "uzon57jd0v869t7w"
		}

* `.server.json`

		{
  			"listen_addr" : "127.0.0.1:20010",   远程服务器监听地址
 			 "password" : "uzon57jd0v869t7w"
		}
		
## TODO

* 优化代码

* 支持多种后台运行的方法

* 支持自定义HTTP报文



 