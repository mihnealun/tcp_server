package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

const (
	defaultServerAddress = "localhost:4040"
	defaultPathPrefix    = "."
	usageServer          = "Server address to bind to. Ex: localhost:4040"
	usagePath            = "Path used fo saving received files. No end slash necessary. Ex: /tmp"
)

func main() {
	serverAddr := defaultServerAddress
	pathPrefix := defaultPathPrefix

	cmd := flag.NewFlagSet("tcpServer", flag.ExitOnError)
	cmd.StringVar(&serverAddr, "server", defaultServerAddress, usageServer)
	cmd.StringVar(&pathPrefix, "path", defaultPathPrefix, usagePath)

	err := cmd.Parse(os.Args[1:])
	checkError(err)

	server, err := net.Listen("tcp", serverAddr)
	fmt.Printf("[server] Listening on %s \n", serverAddr)
	fmt.Printf("[server] Saving files to %s \n", pathPrefix)
	checkError(err)

	done := make(chan struct{})

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			conn, err := server.Accept()
			if err != nil {
				log.Println(err)
				return
			}

			go func(c net.Conn) {
				defer func() {
					_ = c.Close()
				}()
				handleRequest(c, pathPrefix)
			}(conn)
		}
	}()

	<-done
	_ = server.Close()
}

func checkError(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func handleRequest(conn net.Conn, path string) {
	// read filename length
	fileLen := make([]byte, 4)
	fnLengthReader := io.LimitReader(conn, 4)

	for {
		_, err := fnLengthReader.Read(fileLen)
		if err != nil {
			break
		}
	}

	// filename size
	ln := binary.LittleEndian.Uint16(fileLen)

	// read filename based on the length received above
	fnFileNameReader := io.LimitReader(conn, int64(ln))

	bufName := make([]byte, ln)
	for {
		_, err := fnFileNameReader.Read(bufName)
		if err != nil {
			break
		}
	}

	fName := string(bufName)
	fullPath := fmt.Sprintf("%s/%s", path, fName)

	// create new file
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(fullPath), 0700)
	}
	fo, err := os.Create(fullPath)
	checkError(err)
	defer func(fo *os.File) {
		err := fo.Close()
		if err != nil {
			checkError(err)
		}
	}(fo)

	// write file data
	_, err = io.Copy(fo, conn)
	checkError(err)

	_, _ = fmt.Fprintf(os.Stdout, "[server] File received: %s \n", fmt.Sprintf("%s/%s", path, fName))
}
