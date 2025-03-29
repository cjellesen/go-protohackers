package worker

import (
	"context"
	"log"
)

type Job interface {
	Execute(ctx context.Context, logger *log.Logger) error
}
