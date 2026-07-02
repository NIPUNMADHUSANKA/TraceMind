package store

import (
	"sync"
	"time"
)

var (
	payloadAllowListMu sync.RWMutex
	payloadAllowList   map[string]bool
)

// ConfigurePayloadAllowList sets the payload keys that are retained when saving signals.
// If no keys are configured, payload values are stored as-is.
func ConfigurePayloadAllowList(keys []string) {
	payloadAllowListMu.Lock()
	defer payloadAllowListMu.Unlock()
	if len(keys) == 0 {
		payloadAllowList = nil
		return
	}
	next := make(map[string]bool, len(keys))
	for _, k := range keys {
		next[k] = true
	}
	payloadAllowList = next
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
func StartRetentionEnforcer(s PostgresStore, window time.Duration, stop <-chan struct{}) {
	startRetentionEnforcerWithInterval(s, window, time.Hour, stop)
}

func startRetentionEnforcerWithInterval(s PostgresStore, window time.Duration, interval time.Duration, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		cleanup := func() {
			cutoff := time.Now().UTC().Add(-window)
			s.DeleteSignalsOlderThan(cutoff)
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
