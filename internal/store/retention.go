package store

import (
	"log"
	"sync"
	"time"
)

var (
	payloadAllowListMu sync.RWMutex
	payloadAllowList   map[string]bool
)

// ConfigurePayloadAllowList loads the payload allow-list for the given environment from the store.
// If no keys are configured (or no config exists), payload values are stored as-is.
func ConfigurePayloadAllowList(s PostgresStore, env string) {
	allow, _, err := s.GetPayloadFilterConfig(env)
	payloadAllowListMu.Lock()
	defer payloadAllowListMu.Unlock()
	if err != nil {
		log.Printf("store: load payload filter config failed: %v", err)
		payloadAllowList = nil
		return
	}
	if len(allow) == 0 {
		payloadAllowList = nil
		return
	}

	payloadAllowList = make(map[string]bool, len(allow))

	for _, k := range allow {
		payloadAllowList[k] = true
	}
}

func payloadAllowListSnapshot() map[string]bool {
	payloadAllowListMu.RLock()
	defer payloadAllowListMu.RUnlock()
	if len(payloadAllowList) == 0 {
		return nil
	}
	copyMap := make(map[string]bool, len(payloadAllowList))
	for k, v := range payloadAllowList {
		copyMap[k] = v
	}
	return copyMap
}

// RedactPayloadByAllowList keeps only allow-listed keys.
// If allowList is empty, payload is returned unchanged.
func RedactPayloadByAllowList(payload map[string]interface{}, allowList map[string]bool) map[string]interface{} {
	if payload == nil {
		return nil
	}
	if len(allowList) == 0 {
		copyPayload := make(map[string]interface{}, len(payload))
		for k, v := range payload {
			copyPayload[k] = v
		}
		return copyPayload
	}
	redacted := make(map[string]interface{}, len(allowList))
	for k, v := range payload {
		if allowList[k] {
			redacted[k] = v
		}
	}
	return redacted
}

// StartRetentionEnforcer periodically deletes expired signals.
func StartRetentionEnforcer(s PostgresStore, t string, window time.Duration, stop <-chan struct{}) {
	startRetentionEnforcerWithInterval(s, t, window, time.Hour, stop)
}

func StartProfileRetentionEnforcers(s PostgresStore, env string, stop <-chan struct{}) {
	profile := RetentionProfileForEnvironment(env)
	StartRetentionEnforcer(s, "signals", profile.RawWindow, stop)
	StartRetentionEnforcer(s, "incidents", profile.NormalizedWindow, stop)
}

func startRetentionEnforcerWithInterval(s PostgresStore, t string, window time.Duration, interval time.Duration, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		cleanup := func() {
			cutoff := time.Now().UTC().Add(-window)
			switch t {
			case "signals":
				s.DeleteSignalsOlderThan(cutoff)
			case "incidents":
				s.DeleteIncidentsOlderThan(cutoff)
			}
		}
		cleanup()
		for {
			select {
			case <-ticker.C:
				cleanup()
			case <-stop:
				return
			}
		}
	}()
}
