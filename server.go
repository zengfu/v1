package main

import (
	"fmt"
	// "github.com/jinzhu/gorm"
	// _ "github.com/jinzhu/gorm/dialects/mysql"
	//"github.com/gomqtt/packet"
	"github.com/zengfu/v1/broker"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("socket create failed")
	}
	fmt.Println("start")
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}

		client := broker.NewClient(conn)
		go client.Process()
	}
}
