package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
)

type TcpServer struct {
	protocol string
	port     uint
	log      *log.Logger
	isActive bool
	lsn      net.Listener
	handler  ConnectionHandler
}

func NewServer(protocol string, port uint, handler ConnectionHandler) *TcpServer {
	return &TcpServer{
		protocol: protocol,
		port:     port,
		log:      log.New(os.Stdout, "Server: ", log.Ldate|log.Ltime),
		isActive: false,
		handler:  handler,
	}
}

func (s *TcpServer) Listen(ctx context.Context) error {
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

	s.lsn = listener

	go s.listenerLoop(ctx)
	s.isActive = true

	s.log.Printf("Actively listening on port %s", s.ListenerAddr())
	return nil
}

func (s *TcpServer) listenerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.log.Printf("No longer acception connections due to shutdown signal: '%s'", ctx.Err())
			return
		default:
			s.log.Print("Listening for connections")
			conn, err := s.lsn.Accept()
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
			go s.handler.HandleConnection(ctx, s.log, conn)
		}
	}
}

func (s *TcpServer) Shutdown(ctx context.Context) {
	s.log.Printf("Actively monitoring for shutdown signals")
	<-ctx.Done()

	s.log.Printf("Received an external shutdown signal: '%s'", ctx.Err())
	err := s.lsn.Close()
	if err != nil {
		s.log.Print("Failed to close listener")
		return
	}
	s.log.Print("Successfully terminated server")
	s.isActive = false
}

func (s *TcpServer) IsActive() bool {
	return s.isActive
}

func (s *TcpServer) Network() string {
	return s.protocol
}

func (s *TcpServer) ListenerAddr() string {
	return fmt.Sprintf("%s:%d", "127.0.0.1", s.port)
}
