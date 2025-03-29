package server

import (
	"context"
	"log"
	"net"
)

type ConnectionHandler interface {
	HandleConnection(ctx context.Context, log *log.Logger, conn net.Conn)
}
