package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
	"tracemind/internal/analysis"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const correlationWindow = time.Minute

type deliveryQueue interface {
	Dequeue(ctx context.Context) (*types.Message, error)
	Ack(string, context.Context) error
	Nack(string, context.Context) error
}

var processDelivery = func(job queue.IngestionJob, st store.PostgresStore) error {
	return processJob(job, st)
}

var incidentAnalyzer = analysis.NewRuleEngine()

func StartWorker(q deliveryQueue, st store.PostgresStore, stopch <-chan struct{}) {
	go func() {
		for {
			select {
			case <-stopch:
				log.Println("worker: stopping")
				return
			default:
			}

			deliveryInput, err := q.Dequeue(context.Background())
			if err != nil {
				if errors.Is(err, queue.ErrQueueEmpty) {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				log.Printf("worker: dequeue failed: %v", err)
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if deliveryInput == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			receiptHandle := aws.ToString(deliveryInput.ReceiptHandle)
			messageID := aws.ToString(deliveryInput.MessageId)

			body := aws.ToString(deliveryInput.Body)
			var delivery queue.IngestionJob

			err = json.Unmarshal([]byte(body), &delivery)
			if err != nil {
				log.Printf("failed to unmarshal SQS message: %v", err)
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if err := processDelivery(delivery, st); err != nil {
				if nackErr := q.Nack(receiptHandle, context.Background()); nackErr != nil {
					log.Printf("worker: nack failed for receipt %s: %v", messageID, nackErr)
				}
				continue
			}

			if err := q.Ack(receiptHandle, context.Background()); err != nil {
				log.Printf("worker: ack failed for receipt %s: %v", messageID, err)
			}
		}
	}()
}

func processJob(job queue.IngestionJob, st store.PostgresStore) error {
	groups := groupBySourceAndWindow(job.Signals, correlationWindow)
	for _, g := range groups {
		// Signals are already persisted by the ingest handler; only correlate incidents here.
		if groupHasHighSeverity(g) {
			if err := upsertIncidentForGroup(g, st, correlationWindow); err != nil {
				return fmt.Errorf("upsert incident: %w", err)
			}
			continue
		}
		if err := mergeGroupIntoRelatedIncident(g, st, correlationWindow); err != nil {
			return fmt.Errorf("merge incident: %w", err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

type signalGroup struct {
	Source  string
	Env     string
	Signals []models.Signal
	Start   time.Time
	End     time.Time
}

func groupBySourceAndWindow(signals []models.Signal, window time.Duration) []signalGroup {
	if len(signals) == 0 {
		return nil
	}
	buckets := make(map[string][]models.Signal)
	for _, s := range signals {
		key := s.Source + "|" + s.Env
		buckets[key] = append(buckets[key], s)
	}

	groups := make([]signalGroup, 0)
	for key, list := range buckets {
		sort.Slice(list, func(i, j int) bool {
			return signalTime(list[i]).Before(signalTime(list[j]))
		})

		parts := strings.SplitN(key, "|", 2)
		source := parts[0]
		env := ""
		if len(parts) == 2 {
			env = parts[1]
		} else {
			env = os.Getenv("APP_ENV")
		}

		current := signalGroup{Source: source, Env: env, Signals: []models.Signal{list[0]}, Start: signalTime(list[0]), End: signalTime(list[0])}
		for i := 1; i < len(list); i++ {
			ts := signalTime(list[i])
			if ts.Sub(current.End) > window {
				groups = append(groups, current)
				current = signalGroup{Source: source, Env: env, Signals: []models.Signal{list[i]}, Start: ts, End: ts}
				continue
			}
			current.Signals = append(current.Signals, list[i])
			current.End = ts
		}
		groups = append(groups, current)
	}
	return groups
}

func signalTime(s models.Signal) time.Time {
	if s.Timestamp.IsZero() {
		return time.Now().UTC()
	}
	return s.Timestamp
}

func groupHasHighSeverity(g signalGroup) bool {
	for _, s := range g.Signals {
		if s.Severity >= 4 {
			return true
		}
	}
	return false
}

/*
Check can we optimize this function
*/
func upsertIncidentForGroup(g signalGroup, st store.PostgresStore, window time.Duration) error {
	if inc, ok := findRelatedOpenIncident(st.ListIncidents(), g.Source, g.Env, g.End, window); ok {
		inc.SignalIDs = appendUniqueSignalIDs(inc.SignalIDs, signalIDs(g.Signals))
		inc.Severity = maxSeverity(inc.Severity, maxGroupSeverity(g))
		inc.UpdatedAt = time.Now().UTC()
		if err := attachAnalysis(&inc, g.Signals, st); err != nil {
			return err
		}
		return st.SaveIncident(inc)
	}
	inc := models.Incident{
		Title:            "Auto-generated incident",
		Status:           "new",
		Severity:         maxGroupSeverity(g),
		SignalIDs:        signalIDs(g.Signals),
		ImpactedServices: []string{g.Source},
		Environments:     []string{g.Env},
	}
	if err := attachAnalysis(&inc, g.Signals, st); err != nil {
		return err
	}
	return st.SaveIncident(inc)
}

func mergeGroupIntoRelatedIncident(g signalGroup, st store.PostgresStore, window time.Duration) error {
	inc, ok := findRelatedOpenIncident(st.ListIncidents(), g.Source, g.Env, g.End, window)
	if !ok {
		return nil
	}
	inc.SignalIDs = appendUniqueSignalIDs(inc.SignalIDs, signalIDs(g.Signals))
	inc.UpdatedAt = time.Now().UTC()
	if err := attachAnalysis(&inc, g.Signals, st); err != nil {
		return err
	}
	return st.SaveIncident(inc)
}

func attachAnalysis(incident *models.Incident, evidence []models.Signal, st store.PostgresStore) error {
	if incident == nil {
		return nil
	}
	incident.Status = "in-progress"
	if incident.ID != "" {
		if err := st.UpdateIncidentStatus(incident.ID, incident.Status); err != nil {
			return fmt.Errorf("set incident in-progress: %w", err)
		}
	}
	result, err := incidentAnalyzer.Analyze(*incident, evidence, st)
	if err != nil {
		return fmt.Errorf("analyze incident: %w", err)
	}
	incident.AnalysisSummary = strings.Join(result.Hypotheses, "; ")
	incident.Recommendations = append(incident.Recommendations, result.Recommendations...)
	incident.Status = "resolved"
	if incident.ID != "" {
		if err := st.UpdateIncidentStatus(incident.ID, incident.Status); err != nil {
			return fmt.Errorf("set incident resolved: %w", err)
		}
	}
	return nil
}

func findRelatedOpenIncident(incidents []models.Incident, source, env string, ts time.Time, window time.Duration) (models.Incident, bool) {
	for _, inc := range incidents {
		if strings.ToLower(inc.Status) == "resolved" || strings.ToLower(inc.Status) == "closed" || strings.ToLower(inc.Status) == "in-progress" {
			continue
		}
		if !contains(inc.ImpactedServices, source) || !contains(inc.Environments, env) {
			continue
		}
		if inc.UpdatedAt.IsZero() || ts.Sub(inc.UpdatedAt) > window {
			continue
		}
		return inc, true
	}
	return models.Incident{}, false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func signalIDs(signals []models.Signal) []string {
	ids := make([]string, 0, len(signals))
	for _, s := range signals {
		ids = append(ids, s.ID)
	}
	return ids
}

func appendUniqueSignalIDs(base []string, extra []string) []string {
	seen := make(map[string]bool, len(base))
	for _, id := range base {
		seen[id] = true
	}
	for _, id := range extra {
		if seen[id] {
			continue
		}
		base = append(base, id)
		seen[id] = true
	}
	return base
}

func maxGroupSeverity(g signalGroup) int {
	max := 0
	for _, s := range g.Signals {
		if s.Severity > max {
			max = s.Severity
		}
	}
	return max
}

func maxSeverity(a, b int) int {
	if a > b {
		return a
	}
	return b
}
