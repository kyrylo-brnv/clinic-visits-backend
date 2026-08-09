package outbox

import (
	"context"
	"time"
)

const (
	workerBatchSize    int32 = 100
	workerPollInterval       = time.Second
)

type batchProcessor interface {
	ProcessBatch(context.Context, int32) (int, error)
}

type waitForNextAttempt func(context.Context, time.Duration) bool

// RunWorker processes pending events until ctx is canceled. Full batches are
// drained immediately; empty queues and retryable failures pause before the
// next attempt.
func RunWorker(ctx context.Context, processor *Processor, logError func(error)) {
	runWorker(
		ctx,
		processor,
		workerBatchSize,
		workerPollInterval,
		logError,
		waitForWorkerPoll,
	)
}

func runWorker(
	ctx context.Context,
	processor batchProcessor,
	batchSize int32,
	pollInterval time.Duration,
	logError func(error),
	wait waitForNextAttempt,
) {
	for ctx.Err() == nil {
		processed, err := processor.ProcessBatch(ctx, batchSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if logError != nil {
				logError(err)
			}
			if !wait(ctx, pollInterval) {
				return
			}
			continue
		}

		if processed < int(batchSize) && !wait(ctx, pollInterval) {
			return
		}
	}
}

func waitForWorkerPoll(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
