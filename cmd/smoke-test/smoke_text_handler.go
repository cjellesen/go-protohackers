package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
)

type ConnectionHandler struct{}

func NewConnectionHandler() ConnectionHandler {
	return ConnectionHandler{}
}

func (s ConnectionHandler) HandleConnection(ctx context.Context, log *log.Logger, conn net.Conn) {
	reader := bufio.NewReaderSize(conn, 1024)
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Printf(
				"Failed to close the connection from %s, failed with error: '%s'",
				conn.RemoteAddr(),
				err.Error(),
			)
		}
		log.Printf("Successfully closed the connection to client: %s", conn.RemoteAddr())
	}()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Received CONNECTION signal due to: '%s'", ctx.Err())
			return
		default:
			length, err := reader.Read(buffer)
			if err != nil {
				if err != io.EOF {
					log.Printf("Failed to read packet from %s", conn.RemoteAddr())
				}

				return
			}

			log.Printf("Successfully read message (%d bytes) from %s", length, conn.RemoteAddr())

			length, err = conn.Write(buffer[:length])
			if err != nil {
				log.Printf("Failed to write packet (%d bytes) to %s", length, conn.RemoteAddr())
			}

			log.Printf("Successfully wrote message (%d bytes) to %s", length, conn.RemoteAddr())
		}
	}
}
