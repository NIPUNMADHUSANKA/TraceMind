package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReliableQueue_MovesToDLQAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	q := NewReliableQueue(QueueConfig{MaxAttempts: 2, VisibilityTimeout: 20 * time.Millisecond})
	require.NoError(t, q.Enqueue(IngestionJob{IngestionID: "ing-1"}))

	d1, err := q.Dequeue(context.Background())
	require.NoError(t, err)
	require.NoError(t, q.Nack(d1.Receipt, "failure-1"))

	d2, err := q.Dequeue(context.Background())
	require.NoError(t, err)
	require.NoError(t, q.Nack(d2.Receipt, "failure-2"))

	stats := q.Stats()
	require.Equal(t, 1, stats.DeadLetterCount)
	require.Equal(t, 2, stats.RetryCount)
}

func TestReliableQueue_RequeuesAfterVisibilityTimeout(t *testing.T) {
	t.Parallel()

	q := NewReliableQueue(QueueConfig{MaxAttempts: 3, VisibilityTimeout: 15 * time.Millisecond})
	require.NoError(t, q.Enqueue(IngestionJob{IngestionID: "ing-2"}))

	d1, err := q.Dequeue(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, d1.Attempt)

	time.Sleep(20 * time.Millisecond)
	d2, err := q.Dequeue(context.Background())
	require.NoError(t, err)
	require.Equal(t, d1.Job.IngestionID, d2.Job.IngestionID)
	require.Equal(t, 2, d2.Attempt)
}
