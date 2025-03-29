package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
)

type Worker[T Job] struct {
	id           uuid.UUID
	log          *log.Logger
	isActive     bool
	processQueue chan T
	cancelSignal chan int
}

func NewWorker[T Job]() *Worker[T] {
	id := uuid.New()
	return &Worker[T]{
		id:           id,
		log:          log.New(os.Stdout, fmt.Sprintf("Worker (%s): ", id), log.Ldate|log.Ltime),
		isActive:     false,
		processQueue: make(chan T, 1),
		cancelSignal: make(chan int, 1),
	}
}

func (w *Worker[T]) Start(ctx context.Context) {
	w.log.Print("Ready for work!")
	for {
		select {
		case <-ctx.Done():
			w.log.Printf("Terminating worker, received cancellation: %s", ctx.Err())
			return
		case job := <-w.processQueue:
			w.log.Print("Found work!")
			jobCtx, cancel := context.WithCancel(ctx)
			go w.internalRun(ctx, job)
			select {
			case <-jobCtx.Done():
				w.log.Printf(
					"Terminating job execution, received external cancellation: %s",
					ctx.Err(),
				)
				cancel()
				w.isActive = false
			case <-w.cancelSignal:
				w.log.Print("Terminating job execution, job was requested cancelled")
				cancel()
				w.isActive = false
			}

		default:
			continue
		}
	}
}

func (w *Worker[T]) GetId() uuid.UUID {
	return w.id
}

func (w *Worker[T]) IsActive() bool {
	return w.isActive
}

func (w *Worker[T]) internalRun(ctx context.Context, task T) {
	err := task.Execute(ctx, w.log)
	w.isActive = true
	if err != nil {
		w.log.Printf("Executor failed to execute, failed with error: %s", err.Error())
	}

	w.isActive = false
}

func (w *Worker[T]) CancelCurrentJob() error {
	if w.IsActive() {
		return errors.New("No job is currently processing, nothing to cancel")
	}
	w.cancelSignal <- 1
	return nil
}
