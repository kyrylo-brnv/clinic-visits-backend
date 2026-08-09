package outbox

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRunWorkerUsesBoundedBatchesAndOnlyPausesAfterQueueIsDrained(t *testing.T) {
	t.Parallel()

	processor := &fakeWorkerProcessor{results: []int{3, 0}}
	waits := 0
	runWorker(
		t.Context(),
		processor,
		3,
		time.Second,
		nil,
		func(context.Context, time.Duration) bool {
			waits++
			return false
		},
	)

	if want := []int32{3, 3}; !reflect.DeepEqual(processor.batchSizes, want) {
		t.Fatalf("batch sizes = %v, want %v", processor.batchSizes, want)
	}
	if waits != 1 {
		t.Fatalf("wait calls = %d, want 1", waits)
	}
}

func TestRunWorkerLogsFailureAndWaitsBeforeRetry(t *testing.T) {
	t.Parallel()

	failure := errors.New("Elasticsearch unavailable")
	processor := &fakeWorkerProcessor{err: failure}
	var logged error
	waits := 0
	runWorker(
		t.Context(),
		processor,
		5,
		time.Second,
		func(err error) { logged = err },
		func(context.Context, time.Duration) bool {
			waits++
			return false
		},
	)

	if processor.calls != 1 {
		t.Fatalf("process calls = %d, want 1", processor.calls)
	}
	if !errors.Is(logged, failure) {
		t.Fatalf("logged error = %v, want %v", logged, failure)
	}
	if waits != 1 {
		t.Fatalf("wait calls = %d, want 1", waits)
	}
}

func TestRunWorkerStopsPromptlyWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	processingStarted := make(chan struct{})
	processor := &fakeWorkerProcessor{processingStarted: processingStarted}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runWorker(ctx, processor, 5, time.Hour, nil, waitForWorkerPoll)
	}()

	select {
	case <-processingStarted:
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestRunWorkerDoesNotLogCancellationAsRetryableFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	processor := processBatchFunc(func(context.Context, int32) (int, error) {
		cancel()
		return 0, context.Canceled
	})
	logged := false
	waited := false
	runWorker(
		ctx,
		processor,
		5,
		time.Second,
		func(error) { logged = true },
		func(context.Context, time.Duration) bool {
			waited = true
			return false
		},
	)

	if logged {
		t.Fatal("cancellation was logged as a retryable failure")
	}
	if waited {
		t.Fatal("worker waited for a retry after cancellation")
	}
}

type fakeWorkerProcessor struct {
	results           []int
	err               error
	calls             int
	batchSizes        []int32
	processingStarted chan struct{}
}

func (p *fakeWorkerProcessor) ProcessBatch(_ context.Context, batchSize int32) (int, error) {
	p.calls++
	p.batchSizes = append(p.batchSizes, batchSize)
	if p.processingStarted != nil {
		close(p.processingStarted)
		p.processingStarted = nil
	}
	if p.err != nil {
		return 0, p.err
	}
	if len(p.results) == 0 {
		return 0, nil
	}

	result := p.results[0]
	p.results = p.results[1:]
	return result, nil
}

type processBatchFunc func(context.Context, int32) (int, error)

func (f processBatchFunc) ProcessBatch(ctx context.Context, batchSize int32) (int, error) {
	return f(ctx, batchSize)
}
