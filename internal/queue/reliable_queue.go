package queue

import (
	"context"
	"errors"
	"sync"
	"time"
	"tracemind/internal/models"

	"github.com/google/uuid"
)

var (
	ErrQueueEmpty      = errors.New("queue is empty")
	ErrReceiptNotFound = errors.New("receipt not found")
)

type IngestionJob struct {
	IngestionID string
	Signals     []models.Signal
}

/*
Purpose: runtime behavior settings for retry logic.
MaxAttempts: maximum total delivery attempts before dead-lettering.
VisibilityTimeout: how long a dequeued item stays invisible before it can be retried.
*/
type QueueConfig struct {
	MaxAttempts       int
	VisibilityTimeout time.Duration
}

/*
Purpose: snapshot of queue.
Depth: total items not fully completed in the Queue.
RetryCount: total retries triggered by nack or timeout expiration.
DeadLetterCount: number of items dropped after exceeding attempts.
LastProcessedTimestamp: last successful ack or nack processing time.
*/
type QueueStats struct {
	Depth                  int
	RetryCount             int
	DeadLetterCount        int
	LastProcessedTimestamp time.Time
}

/*
Purpose: payload returned to consumers
Job: the actual ingestion job to process.
Receipt: unique token required for Ack or Nack.
Attempt: current delivery attempt number.
*/
type Delivery struct {
	Job     IngestionJob
	Receipt string
	Attempt int
}

/*
Purpose: in-memory queue engine with visibility-timeout + retry semantics.
mu: mutex for thread-safe access.
config: QueueConfig.
ready: FIFO list of items available for dequeue.
inFlight: map from receipt to currently leased item.
retryCount: counter for all retry events.
deadLetter: counter for items that exceeded max attempts.
lastProcessed: timestamp updated on Ack/Nack.
*/
type ReliableQueue struct {
	mu            sync.Mutex
	config        QueueConfig
	ready         []queueItem
	inFlight      map[string]inFlightItem
	retryCount    int
	deadLetter    int
	lastProcessed time.Time
}

/*
Purpose: lightweight internal representation of items waiting in ready.
*/
type queueItem struct {
	job     IngestionJob
	attempt int
}

/*
Purpose: internal leased item state while worker is processing.
*/
type inFlightItem struct {
	job       IngestionJob
	attempt   int
	visibleAt time.Time
}

func NewReliableQueue(cfg QueueConfig) *ReliableQueue {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.VisibilityTimeout <= 0 {
		cfg.VisibilityTimeout = 30 * time.Second
	}

	return &ReliableQueue{
		config:   cfg,
		ready:    make([]queueItem, 0),
		inFlight: make(map[string]inFlightItem),
	}
}

func (q *ReliableQueue) Enqueue(job IngestionJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.ready = append(q.ready, queueItem{job: job, attempt: 1})
	return nil
}

func (q *ReliableQueue) Dequeue(ctx context.Context) (Delivery, error) {
	if err := ctx.Err(); err != nil {
		return Delivery{}, err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	q.sweepExpired(time.Now().UTC())
	if len(q.ready) == 0 {
		return Delivery{}, ErrQueueEmpty
	}

	item := q.ready[0]
	q.ready = q.ready[1:]

	receipt := uuid.NewString()
	q.inFlight[receipt] = inFlightItem{
		job:       item.job,
		attempt:   item.attempt,
		visibleAt: time.Now().UTC().Add(q.config.VisibilityTimeout),
	}

	return Delivery{Job: item.job, Receipt: receipt, Attempt: item.attempt}, nil
}

func (q *ReliableQueue) Ack(receipt string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.inFlight[receipt]; !ok {
		return ErrReceiptNotFound
	}
	delete(q.inFlight, receipt)
	q.lastProcessed = time.Now().UTC()
	return nil
}

func (q *ReliableQueue) Nack(receipt, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	item, ok := q.inFlight[receipt]
	if !ok {
		return ErrReceiptNotFound
	}
	delete(q.inFlight, receipt)
	q.retryCount++

	q.requeueOrDeadLetter(item)
	q.lastProcessed = time.Now().UTC()
	_ = reason
	return nil
}

func (q *ReliableQueue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.sweepExpired(time.Now().UTC())

	return QueueStats{
		Depth:                  len(q.ready) + len(q.inFlight),
		RetryCount:             q.retryCount,
		DeadLetterCount:        q.deadLetter,
		LastProcessedTimestamp: q.lastProcessed,
	}
}

func (q *ReliableQueue) sweepExpired(now time.Time) {
	for receipt, item := range q.inFlight {
		if now.Before(item.visibleAt) {
			continue
		}

		delete(q.inFlight, receipt)
		q.retryCount++
		q.requeueOrDeadLetter(item)
	}
}

func (q *ReliableQueue) requeueOrDeadLetter(item inFlightItem) {
	nextAttempt := item.attempt + 1
	if nextAttempt > q.config.MaxAttempts {
		q.deadLetter++
		return
	}

	q.ready = append(q.ready, queueItem{job: item.job, attempt: nextAttempt})
}
