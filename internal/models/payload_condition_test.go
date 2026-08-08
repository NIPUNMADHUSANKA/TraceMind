package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPayloadConditionUnmarshalRejectsUnknownField(t *testing.T) {
	var cond PayloadCondition
	err := json.Unmarshal([]byte(`{"path":"db.host","operator":"exists"}`), &cond)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unknown field") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestPayloadConditionUnmarshalAcceptsKnownFields(t *testing.T) {
	var cond PayloadCondition
	err := json.Unmarshal([]byte(`{"field":"db.host","operator":"exists","value":true}`), &cond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cond.Field != "db.host" {
		t.Fatalf("expected field db.host, got %s", cond.Field)
	}
	if cond.Operator != "exists" {
		t.Fatalf("expected operator exists, got %s", cond.Operator)
	}
}
