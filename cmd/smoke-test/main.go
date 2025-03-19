package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

// The following challenge can be found here: https://protohackers.com/problem/0

// View leaderboard

// Deep inside Initrode Global's enterprise management framework lies a component that writes data to a server and expects to read the same data back. (Think of it as a kind of distributed system delay-line memory). We need you to write the server to echo the data back.

// Accept TCP connections.

// Whenever you receive data from a client, send it back unmodified.

// Make sure you don't mangle binary data, and that you can handle at least 5 simultaneous clients.

// Once the client has finished sending data to you it shuts down its sending side. Once you've reached end-of-file on your receiving side, and sent back all the data you've received, close the socket so that the client knows you've finished. (This point trips up a lot of proxy software, such as ngrok; if you're using a proxy and you can't work out why you're failing the check, try hosting your server in the cloud instead).

// Your program will implement the TCP Echo Service from RFC 862.a

type Server struct {
	protocol     string
	port         int
	log          *log.Logger
	cancellation context.Context
}

func NewServer(protocol string, port int, ctx context.Context) *Server {
	return &Server{
		protocol:     protocol,
		port:         port,
		log:          log.New(os.Stdout, "Server: ", log.Ldate|log.Ltime),
		cancellation: ctx,
	}
}

func (s *Server) Network() string {
	return s.protocol
}

func (s *Server) ListenerAddr() string {
	return fmt.Sprintf("%s:%d", "127.0.0.1", s.port)
}

func (s *Server) Listen() {
	listener, err := net.Listen(s.Network(), s.ListenerAddr())
	defer func() {
		err = listener.Close()
		if err != nil {
			s.log.Print("Failed to close listener")
		}
	}()
	if err != nil {
		s.log.Printf("Failed to bind to port %s using protocol %s", s.ListenerAddr(), s.Network())
	}

	s.log.Printf("Actively listening on port %s", s.ListenerAddr())

	for {
		select {
		case <-s.cancellation.Done():
			s.log.Printf("Received shutdown signal, terminating listener at %s", listener.Addr())
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				s.log.Printf(
					"Error occured while trying to accept a connection, failed with %s",
					err.Error(),
				)
			}

			s.log.Printf("Received connection from %s, forwarding to handler", conn.RemoteAddr())
			go s.handleConnections(conn)
		}
	}
}

func (s *Server) handleConnections(conn net.Conn) {
	reader := bufio.NewReaderSize(conn, 1024)
	defer func() {
		err := conn.Close()
		if err != nil {
			s.log.Printf("Failed to close the connection from %s", conn.RemoteAddr())
		}
	}()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-s.cancellation.Done():
			s.log.Printf(
				"Received shutdown signal, terminating read from connection: %s",
				conn.RemoteAddr(),
			)
			return
		default:
			length, err := reader.Read(buffer)
			if err != nil {
				if err != io.EOF {
					s.log.Printf("Failed to read packet from %s", conn.RemoteAddr())
				}

				return
			}

			s.log.Printf("Successfully read message (%d bytes) from %s", length, conn.RemoteAddr())
			length, err = conn.Write(buffer[:length])
			if err != nil {
				s.log.Printf("Failed to write packet (%d bytes) to %s", length, conn.RemoteAddr())
			}

			s.log.Printf("Successfully wrote message (%d bytes) to %s", length, conn.RemoteAddr())
		}
	}
}

type Client struct {
	log          log.Logger
	cancellation context.Context
	disconnect   chan uint8
	name         string
	conn         net.Conn
}

func NewClient(name string, ctx context.Context) *Client {
	return &Client{
		log:          *log.New(os.Stdout, fmt.Sprintf("Client (%s): ", name), log.Ldate|log.Ltime),
		cancellation: ctx,
		name:         name,
		conn:         nil,
		disconnect:   make(chan uint8, 1),
	}
}

func (c *Client) gracefulShutdown(conn net.Conn) {
	c.log.Printf(
		"Established monitoring for external shutdown signal or internal disconnect signal",
	)
	for {
		select {
		case <-c.cancellation.Done():
			c.log.Printf("Received an external shutdown signal, shutting down connection")
			err := conn.Close()
			if err != nil {
				c.log.Printf("Failed to close connection to server %s", conn.RemoteAddr())
			}
			c.log.Printf("Successfully terminated connection")
			return
		case <-c.disconnect:
			c.log.Printf("Attempting to disconnect, shutting down connection")
			err := conn.Close()
			if err != nil {
				c.log.Printf("Failed to close connection to server %s", conn.RemoteAddr())
			}
			c.log.Printf("Successfully terminated connection")
			return
		default:
			continue
		}
	}
}

func (c *Client) Connect(protocol string, addr string) {
	conn, err := net.Dial(protocol, addr)
	go c.gracefulShutdown(conn)

	if err != nil {
		c.log.Printf("Failed to establish connection to server at: %s", addr)
	}

	c.conn = conn
}

func (c *Client) Disconnect() {
	c.log.Printf("Client has requested a disconnect, propagating signal")
	c.disconnect <- 1
}

func (c *Client) Write(payload []byte) {
	if c.conn == nil {
		c.log.Printf("Cannot write data to server, no connection has been established")
	}

	n, err := c.conn.Write(payload)
	if err != nil {
		c.log.Printf("Failed to write payload to server at: %s", c.conn.RemoteAddr())
	}
	c.log.Printf(
		"Successfully wrote payload payload (%d bytes) to server at: %s",
		n,
		c.conn.RemoteAddr(),
	)
}

func (c *Client) Read() {
	if c.conn == nil {
		log.Printf("Cannot read data from server, no connection has been established")
	}
}

func main() {
}
