package worker

import (
	"context"
	"errors"
	"testing"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
)

func TestGroupBySourceAndWindow_SplitsBySourceEnvAndGap(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	signals := []models.Signal{
		{ID: "a1", Source: "svc-a", Env: "prod", Timestamp: base, Severity: 2},
		{ID: "a2", Source: "svc-a", Env: "prod", Timestamp: base.Add(10 * time.Second), Severity: 2},
		{ID: "a3", Source: "svc-a", Env: "prod", Timestamp: base.Add(2 * time.Minute), Severity: 4},
		{ID: "b1", Source: "svc-b", Env: "prod", Timestamp: base.Add(5 * time.Second), Severity: 3},
	}

	groups := groupBySourceAndWindow(signals, 30*time.Second)
	assert.Len(t, groups, 3)
}

func TestProcessJob_ReturnsNilError(t *testing.T) {
	assert.NoError(t, processJob(queue.IngestionJob{}, store.PostgresStore{}))
}

type mockQueue struct {
	msg        *types.Message
	err        error
	ackCalled  bool
	nackCalled bool
	ackReceipt string
}

func (m *mockQueue) Dequeue(ctx context.Context) (*types.Message, error) {
	if m.msg == nil && m.err == nil {
		return nil, queue.ErrQueueEmpty
	}
	msg := m.msg
	m.msg = nil
	return msg, m.err
}

func (m *mockQueue) Ack(receipt string, ctx context.Context) error {
	m.ackCalled = true
	m.ackReceipt = receipt
	return nil
}

func (m *mockQueue) Nack(receipt string, ctx context.Context) error {
	m.nackCalled = true
	return nil
}

func TestStartWorker_NilReceiptHandleAndMessageIdDoesNotPanic(t *testing.T) {
	origProcessDelivery := processDelivery
	defer func() { processDelivery = origProcessDelivery }()
	processDelivery = func(job queue.IngestionJob, st store.PostgresStore) error {
		return nil
	}

	mq := &mockQueue{
		msg: &types.Message{
			Body:          aws.String(`{"Signals":[]}`),
			ReceiptHandle: nil,
			MessageId:     nil,
		},
	}
	stopch := make(chan struct{})

	// Start worker in background
	StartWorker(mq, store.PostgresStore{}, stopch)

	// Wait briefly for execution
	time.Sleep(50 * time.Millisecond)
	close(stopch)

	assert.True(t, mq.ackCalled)
	assert.Equal(t, "", mq.ackReceipt)
}

func TestStartWorker_NackWithNilReceiptHandleDoesNotPanic(t *testing.T) {
	origProcessDelivery := processDelivery
	defer func() { processDelivery = origProcessDelivery }()
	processDelivery = func(job queue.IngestionJob, st store.PostgresStore) error {
		return errors.New("process error")
	}

	mq := &mockQueue{
		msg: &types.Message{
			Body:          aws.String(`{"Signals":[]}`),
			ReceiptHandle: nil,
			MessageId:     nil,
		},
	}
	stopch := make(chan struct{})

	StartWorker(mq, store.PostgresStore{}, stopch)

	time.Sleep(50 * time.Millisecond)
	close(stopch)

	assert.True(t, mq.nackCalled)
}
