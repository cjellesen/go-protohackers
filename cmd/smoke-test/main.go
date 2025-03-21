package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type Server struct {
	protocol string
	port     uint
	log      *log.Logger
	isActive bool
	listener net.Listener
}

func NewServer(protocol string, port uint) *Server {
	return &Server{
		protocol: protocol,
		port:     port,
		log:      log.New(os.Stdout, "Server: ", log.Ldate|log.Ltime),
		isActive: false,
	}
}

func (s *Server) Network() string {
	return s.protocol
}

func (s *Server) ListenerAddr() string {
	return fmt.Sprintf("%s:%d", "127.0.0.1", s.port)
}

func (s *Server) Listen(ctx context.Context) error {
	var l net.ListenConfig
	listener, err := l.Listen(ctx, s.Network(), s.ListenerAddr())
	if err != nil {
		s.log.Printf(
			"Failed to bind to port %s using protocol %s, failed with error: '%s'",
			s.ListenerAddr(),
			s.Network(),
			err.Error(),
		)
		return err
	}

	s.listener = listener

	// go s.Shutdown(ctx, listener)
	go s.listenerLoop(ctx)
	s.isActive = true

	s.log.Printf("Actively listening on port %s", s.ListenerAddr())
	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	s.log.Printf("Actively monitoring for shutdown signals")
	<-ctx.Done()

	s.log.Printf("Received an external shutdown signal: '%s'", ctx.Err())
	err := s.listener.Close()
	if err != nil {
		s.log.Print("Failed to close listener")
		return
	}
	s.log.Print("Successfully terminated server")
	s.isActive = false
}

func (s *Server) listenerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.log.Printf("No longer acception connections due to shutdown signal: '%s'", ctx.Err())
			return
		default:
			s.log.Print("Listening for connections")
			conn, err := s.listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					s.log.Printf("Listener loop terminated due to shutdown signal: %s", ctx.Err())
					return
				}
				s.log.Printf(
					"Error occured while trying to accept a connection, failed with %s - Continuing accept loop",
					err.Error(),
				)
				continue
			}

			s.log.Printf("Received connection from %s, forwarding to handler", conn.RemoteAddr())

			go s.handleConnections(ctx, conn)
		}
	}
}

func (s *Server) handleConnections(ctx context.Context, conn net.Conn) {
	reader := bufio.NewReaderSize(conn, 1024)
	defer func() {
		err := conn.Close()
		if err != nil {
			s.log.Printf(
				"Failed to close the connection from %s, failed with error: '%s'",
				conn.RemoteAddr(),
				err.Error(),
			)
		}
		s.log.Printf("Successfully closed the connection to client: %s", conn.RemoteAddr())
	}()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			s.log.Printf("Received CONNECTION signal due to: '%s'", ctx.Err())
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
	log        log.Logger
	disconnect chan uint8
	name       string
	conn       net.Conn
}

func NewClient(name string) *Client {
	return &Client{
		log:        *log.New(os.Stdout, fmt.Sprintf("Client (%s): ", name), log.Ldate|log.Ltime),
		name:       name,
		conn:       nil,
		disconnect: make(chan uint8, 1),
	}
}

func (c *Client) Connect(ctx context.Context, protocol string, addr string) error {
	if c.conn != nil {
		str := fmt.Sprintf(
			"Cannot connect to multiple servers - There is already an existing connection to %s",
			c.conn.RemoteAddr(),
		)
		c.log.Print(str)
		return errors.New(str)
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, protocol, addr)
	go c.gracefulShutdown(ctx, conn)

	if err != nil {
		c.log.Printf("Failed to establish connection to server at: %s", addr)
	}

	c.conn = conn
	return nil
}

func (c *Client) Disconnect() {
	c.log.Printf("Client has requested a disconnect, propagating signal")
	c.disconnect <- 1
}

func (c *Client) Write(payload []byte) error {
	if c.conn == nil {
		return errors.New("Cannot write data to server, no connection has been established")
	}

	c.log.Printf(
		"Attempting to write payload (%d bytes) to server at: %s",
		len(payload),
		c.conn.RemoteAddr(),
	)
	n, err := c.conn.Write(payload)
	if err != nil {
		str := fmt.Sprintf(
			"Failed to write payload to server at: %s, failed with error: %s",
			c.conn.RemoteAddr(),
			err.Error(),
		)
		c.log.Print(str)
		return errors.New(str)
	}
	c.log.Printf(
		"Successfully wrote payload payload (%d bytes) to server at: %s",
		n,
		c.conn.RemoteAddr(),
	)

	return nil
}

func (c *Client) Read() ([]byte, error) {
	if c.conn == nil {
		return nil, errors.New("Cannot write data to server, no connection has been established")
	}

	reader := bufio.NewReaderSize(c.conn, 1024)
	defer func() {
		err := c.conn.Close()
		if err != nil {
			c.log.Printf(
				"Failed to close the connection from %s, failed with error: '%s'",
				c.conn.RemoteAddr(),
				err.Error(),
			)
			c.log.Printf(
				"Successfully closed the connection to the server at: %s",
				c.conn.RemoteAddr(),
			)
		}
	}()

	buffer := make([]byte, 1024)
	length, err := reader.Read(buffer)
	if err != nil {
		if err != io.EOF {
			c.log.Printf("Failed to read packet from %s", c.conn.RemoteAddr())
		}

		return nil, err
	}

	c.log.Printf("Successfully read message (%d bytes) from %s", length, c.conn.RemoteAddr())

	return buffer[:length], nil
}

func (c *Client) gracefulShutdown(ctx context.Context, conn net.Conn) {
	c.log.Printf(
		"Established monitoring for external shutdown signal or internal disconnect signal",
	)
	for {
		select {
		case <-ctx.Done():
			c.log.Printf("Received an external shutdown signal: '%s'", ctx.Err())
			c.log.Printf("Attempting to close connection to server: %s", conn.RemoteAddr())
			err := conn.Close()
			if err != nil {
				c.log.Printf("Failed to close connection to server: %s", conn.RemoteAddr())
				return
			}
			c.log.Printf("Successfully terminated connection to server: %s", conn.RemoteAddr())
			c.conn = nil
			return
		case <-c.disconnect:
			c.log.Printf(
				"Attempting to disconnect, shutting down connection to server: %s",
				conn.RemoteAddr(),
			)
			err := conn.Close()
			if err != nil {
				c.log.Printf("Failed to close connection to server: %s", conn.RemoteAddr())
				return
			}
			c.log.Printf("Successfully terminated connection to server: %s", conn.RemoteAddr())
			c.conn = nil
			return
		default:
			continue
		}
	}
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
	)
	defer stop()
	protocol := "tcp"
	port := rand.UintN(1000) + 8000
	server := NewServer(protocol, port)
	err := server.Listen(ctx)
	if err != nil {
		panic("Failed to start server")
	}

	for {
		if server.isActive {
			break
		}
	}

	// fmt.Printf("Server status: %t\n", server.isActive)
	// for n := range 10 {
	// 	client := NewClient(fmt.Sprintf("Client %d", n))
	// 	err := client.Connect(ctx, server.Network(), server.ListenerAddr())
	// 	if err != nil {
	// 		panic("Failed to start client")
	// 	}

	// 	err = client.Write([]byte("ECHO"))
	// 	if err != nil {
	// 		panic("Client failed to send message to server")
	// 	}

	// 	var payload []byte
	// 	payload, err = client.Read()
	// 	if err != nil {
	// 		panic("Failed to read from server")
	// 	}

	// 	fmt.Printf("Received message from server: %s \n", string(payload))
	// }

	fmt.Println("Awaiting termination")
	<-ctx.Done()
	server.Shutdown(ctx)
	fmt.Printf("Terminated (Server is active: %t)\n", server.isActive)
}
