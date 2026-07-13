package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/stretchr/testify/require"
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
	require.Len(t, groups, 3)
}

func TestProcessJob_CreatesIncidentForHighSeverityGroup(t *testing.T) {
	t.Parallel()

	s, cleanup := newWorkerTestPostgresStore(t)
	t.Cleanup(cleanup)
	base := time.Now().UTC()
	job := ingestionJobForTest([]models.Signal{
		{ID: "h1", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base, Severity: 5},
		{ID: "h2", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base.Add(5 * time.Second), Severity: 4},
	})

	require.NoError(t, processJob(job, s))

	incidents := s.ListIncidents()
	var found *models.Incident
	for i := range incidents {
		inc := &incidents[i]
		if contains(inc.SignalIDs, "h1") && contains(inc.SignalIDs, "h2") {
			found = inc
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, 5, found.Severity)
	require.Equal(t, []string{"svc-a"}, found.ImpactedServices)
	require.Equal(t, []string{"prod"}, found.Environments)
}

func TestProcessJob_MergesIntoExistingIncident_WhenRelated(t *testing.T) {
	t.Parallel()

	s, cleanup := newWorkerTestPostgresStore(t)
	t.Cleanup(cleanup)
	base := time.Now().UTC()
	s.SaveIncident(models.Incident{
		ID:               "inc-existing",
		Title:            "Auto-generated incident",
		Status:           "open",
		Severity:         4,
		SignalIDs:        []string{"prev"},
		ImpactedServices: []string{"svc-a"},
		Environments:     []string{"prod"},
		UpdatedAt:        base,
	})

	job := ingestionJobForTest([]models.Signal{
		{ID: "n1", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base.Add(10 * time.Second), Severity: 5},
	})

	require.NoError(t, processJob(job, s))

	inc, ok := s.GetIncident("inc-existing")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"prev", "n1"}, inc.SignalIDs)
	require.Equal(t, 5, inc.Severity)
}

func TestProcessJob_AttachesAnalysisToIncident(t *testing.T) {
	t.Parallel()

	s, cleanup := newWorkerTestPostgresStore(t)
	t.Cleanup(cleanup)

	base := time.Now().UTC()
	job := ingestionJobForTest([]models.Signal{
		{ID: "a-db-1", EventType: "database", Source: "svc-a", Env: "prod", Timestamp: base, Severity: 5, Message: "too many connections"},
		{ID: "a-health-1", EventType: "health", Source: "svc-a", Env: "prod", Timestamp: base.Add(5 * time.Second), Severity: 4, Message: "service timeout"},
	})

	require.NoError(t, processJob(job, s))

	incidents := s.ListIncidents()
	var found *models.Incident
	for i := range incidents {
		inc := &incidents[i]
		if contains(inc.SignalIDs, "a-db-1") && contains(inc.SignalIDs, "a-health-1") {
			found = inc
			break
		}
	}

	require.NotNil(t, found)
	require.NotEmpty(t, found.AnalysisSummary)
	require.Contains(t, strings.ToLower(found.AnalysisSummary), "database")
	require.NotEmpty(t, found.Recommendations)
}

func TestWorker_NacksDeliveryOnProcessingError(t *testing.T) {
	t.Parallel()

	originalProcessDelivery := processDelivery
	t.Cleanup(func() {
		processDelivery = originalProcessDelivery
	})

	nackCalled := make(chan nackCall, 1)
	q := &fakeDeliveryQueue{
		deliveries: []queue.Delivery{{
			Job:     queue.IngestionJob{IngestionID: "ing-1"},
			Receipt: "receipt-1",
			Attempt: 1,
		}},
		nackCalled: nackCalled,
	}

	processDelivery = func(queue.IngestionJob, store.PostgresStore) error {
		return errors.New("processing failed")
	}

	stopCh := make(chan struct{})
	StartWorker(q, store.PostgresStore{}, stopCh)

	select {
	case call := <-nackCalled:
		require.Equal(t, "receipt-1", call.receipt)
		require.Equal(t, "processing failed", call.reason)
		close(stopCh)
	case <-time.After(time.Second):
		t.Fatal("expected delivery to be nacked")
	}
}

func TestProcessDelivery_ReturnsErrorWhenStoreUnavailable(t *testing.T) {
	t.Parallel()

	job := ingestionJobForTest([]models.Signal{
		{ID: "h1", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: time.Now().UTC(), Severity: 5},
	})

	err := processDelivery(job, store.PostgresStore{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "postgres connection is not initialized")
}

func ingestionJobForTest(signals []models.Signal) queueIngestionJobAlias {
	return queueIngestionJobAlias{Signals: signals}
}

type queueIngestionJobAlias = queue.IngestionJob

type nackCall struct {
	receipt string
	reason  string
}

type fakeDeliveryQueue struct {
	mu         sync.Mutex
	deliveries []queue.Delivery
	nackCalled chan<- nackCall
	acked      []string
	nacked     []nackCall
}

func (f *fakeDeliveryQueue) Dequeue(context.Context) (queue.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.deliveries) == 0 {
		return queue.Delivery{}, queue.ErrQueueEmpty
	}
	delivery := f.deliveries[0]
	f.deliveries = f.deliveries[1:]
	return delivery, nil
}

func (f *fakeDeliveryQueue) Ack(receipt string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.acked = append(f.acked, receipt)
	return nil
}

func (f *fakeDeliveryQueue) Nack(receipt, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	call := nackCall{receipt: receipt, reason: reason}
	f.nacked = append(f.nacked, call)
	if f.nackCalled != nil {
		f.nackCalled <- call
	}
	return nil
}
