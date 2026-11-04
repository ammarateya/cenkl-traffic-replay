package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9000", "TCP address to listen on")
	cert := flag.String("cert", "", "TLS certificate PEM file")
	key := flag.String("key", "", "TLS private key PEM file")
	readTimeout := flag.Duration("read-timeout", 30*time.Second, "maximum read time per connection")
	flag.Parse()
	if *cert == "" || *key == "" || *readTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "-cert and -key are required; -read-timeout must be positive")
		os.Exit(2)
	}
	pair, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	listener, err := tls.Listen("tcp", *listen, &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Printf("TLS discard server listening on %s\n", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		go discard(conn, *readTimeout)
	}
}

func discard(conn net.Conn, idleTimeout time.Duration) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(idleTimeout))
	n, err := io.Copy(io.Discard, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: received=%d bytes; error=%v\n", conn.RemoteAddr(), n, err)
		return
	}
	fmt.Printf("%s: received=%d bytes\n", conn.RemoteAddr(), n)
}
