package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	"github.com/fari-proxy/client/util"
)

func main() {
	var conf string
	var config map[string]string
	flag.StringVar(&conf, "c", ".client.json", "client config")
	flag.Parse()

	bytes, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Fatalf("Reading %s failed.", conf)
	}

	if err := json.Unmarshal(bytes, &config); err != nil {
		log.Fatalf("Parsing %s failed.", conf)
	}
	clientImpl := client.NewClient(config["remote_addr"], config["listen_addr"], config["password"])
	clientImpl.Listen()
}
