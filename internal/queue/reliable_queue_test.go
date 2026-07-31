package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReliableQueue_Enqueue(t *testing.T) {
	t.Parallel()

	q := NewReliableQueue(QueueConfig{MaxAttempts: 2, VisibilityTimeout: 20 * time.Millisecond})

	Job := q.Enqueue(IngestionJob{
		IngestionID: "ing-100",
		Signals:     nil,
	})
	assert.NoError(t, Job)

}
func TestReliableQueue_MovesToDLQAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	q := NewReliableQueue(QueueConfig{
		MaxAttempts:       2,
		VisibilityTimeout: 20 * time.Microsecond,
	})

	Job := q.Enqueue(IngestionJob{
		IngestionID: "ing-1",
	})
	assert.NoError(t, Job)

	d1, err := q.Dequeue(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, q.Nack(d1.Receipt, "failure-1"))

	d2, err := q.Dequeue(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, q.Nack(d2.Receipt, "failure-2"))

	stats := q.Stats()
	assert.Equal(t, 1, stats.DeadLetterCount)
	assert.Equal(t, 2, stats.RetryCount)
}

func TestReliableQueue_RequeuesAfterVisibilityTimeout(t *testing.T) {
	t.Parallel()

	q := NewReliableQueue(QueueConfig{MaxAttempts: 3, VisibilityTimeout: 15 * time.Millisecond})
	assert.NoError(t, q.Enqueue(IngestionJob{IngestionID: "ing-2"}))

	d1, err := q.Dequeue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, d1.Attempt)

	time.Sleep(20 * time.Millisecond)
	d2, err := q.Dequeue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, d1.Job.IngestionID, d2.Job.IngestionID)
	assert.Equal(t, 2, d2.Attempt)

}
