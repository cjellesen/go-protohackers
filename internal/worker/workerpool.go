package worker

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
)

type WorkerPool[T Job] struct {
	poolSize      int
	pool          chan Worker[T]
	killSignal    chan int
	workerKillMap map[uuid.UUID]*Worker[T]
	log           *log.Logger
}

func NewWorkerPool[T Job](nWorkers int) *WorkerPool[T] {
	return &WorkerPool[T]{
		poolSize:      nWorkers,
		pool:          make(chan Worker[T], nWorkers),
		killSignal:    make(chan int, 1),
		workerKillMap: make(map[uuid.UUID]*Worker[T]),
		log:           log.New(os.Stdout, "WorkerPool: ", log.Ldate|log.Ltime),
	}
}

func (wp *WorkerPool[T]) SpinUpWorkers(ctx context.Context) {
	for range wp.poolSize {
		go wp.startWorker(ctx)
	}
}

func (wp *WorkerPool[T]) startWorker(ctx context.Context) {
	worker := NewWorker[T]()
	workerCtx, cancel := context.WithCancel(ctx)
	worker.Start(workerCtx)
	wp.workerKillMap[worker.GetId()] = worker
	select {
	case <-ctx.Done():
		cancel()
	case <-workerCtx.Done():
		cancel()
	case <-wp.killSignal:
		cancel()
	}
}

func (wp *WorkerPool[T]) TerminateWorkerPool() {
	wp.killSignal <- 1
}

func (wp *WorkerPool[T]) TerminateWorker(workerId uuid.UUID) error {
	worker, exists := wp.workerKillMap[workerId]
	if !exists {
		return fmt.Errorf("No worker exists with Id: %s", workerId)
	}

	err := worker.CancelCurrentJob()
	if err != nil {
		return err
	}

	return nil
}
