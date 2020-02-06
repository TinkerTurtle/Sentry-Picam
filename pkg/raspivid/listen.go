package raspivid

import (
	"log"
	"net"
	"os"
)

// listen starts listening on a socket and blocks until a connection is returned
func listen(network string, sockAddr string) net.Conn {
	if network == "unix" {
		os.Remove(sockAddr)
	}

	l, err := net.Listen(network, sockAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Wait for a connection.
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}

	return conn
}
