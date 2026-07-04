package worker

import (
	"log"
	"sort"
	"strings"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"
)

const correlationWindow = time.Minute

func StartWorker(q chan queue.IngestionJob, store store.PostgresStore, stopch <-chan struct{}) {
	go func() {
		for {
			select {
			case job, ok := <-q:
				if !ok {
					log.Println("worker: queue closed")
					return
				}
				processJob(job, store)
			case <-stopch:
				log.Println("worker: stopping")
				return

			}
		}
	}()
}

func processJob(job queue.IngestionJob, store store.PostgresStore) {
	groups := groupBySourceAndWindow(job.Signals, correlationWindow)
	for _, g := range groups {
		// Signals are already persisted by the ingest handler; only correlate incidents here.
		if groupHasHighSeverity(g) {
			upsertIncidentForGroup(g, store, correlationWindow)
			continue
		}
		mergeGroupIntoRelatedIncident(g, store, correlationWindow)
	}
	time.Sleep(100 * time.Millisecond)
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

func upsertIncidentForGroup(g signalGroup, st store.PostgresStore, window time.Duration) {
	if inc, ok := findRelatedOpenIncident(st.ListIncidents(), g.Source, g.Env, g.End, window); ok {
		inc.SignalIDs = appendUniqueSignalIDs(inc.SignalIDs, signalIDs(g.Signals))
		inc.Severity = maxSeverity(inc.Severity, maxGroupSeverity(g))
		inc.UpdatedAt = time.Now().UTC()
		st.SaveIncident(inc)
		return
	}
	st.SaveIncident(models.Incident{
		Title:            "Auto-generated incident",
		Status:           "new",
		Severity:         maxGroupSeverity(g),
		SignalIDs:        signalIDs(g.Signals),
		ImpactedServices: []string{g.Source},
		Environments:     []string{g.Env},
	})
}

func mergeGroupIntoRelatedIncident(g signalGroup, st store.PostgresStore, window time.Duration) {
	inc, ok := findRelatedOpenIncident(st.ListIncidents(), g.Source, g.Env, g.End, window)
	if !ok {
		return
	}
	inc.SignalIDs = appendUniqueSignalIDs(inc.SignalIDs, signalIDs(g.Signals))
	inc.UpdatedAt = time.Now().UTC()
	st.SaveIncident(inc)
}

func findRelatedOpenIncident(incidents []models.Incident, source, env string, ts time.Time, window time.Duration) (models.Incident, bool) {
	for _, inc := range incidents {
		if inc.Status == "resolved" || inc.Status == "closed" {
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
