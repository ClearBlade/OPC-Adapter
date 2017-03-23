package main

import (
	"fmt"
	"net"
	"bufio"
)

func main() {

	connectionString := "127.0.0.1:8999"
	connectionType := "tcp"
	conn, err := net.Dial(connectionType , connectionString)
	if err!=nil {
		fmt.Println("Error : ", err.Error())
	}

	fmt.Fprintf(conn, "Write Error.Boolean")
	message, err := bufio.NewReader(conn).ReadString('\n')

	fmt.Println("Got reading : " , message)

	conn.Close()
}