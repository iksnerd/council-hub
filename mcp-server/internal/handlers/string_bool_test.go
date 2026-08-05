package handlers

import (
	"encoding/json"
	"testing"
)

func TestStringBoolUnmarshalAcceptsStringAndBoolean(t *testing.T) {
	var fromString ListRoomsInput
	if err := json.Unmarshal([]byte(`{"cluster_wide":"true"}`), &fromString); err != nil {
		t.Fatalf("string cluster_wide should decode: %v", err)
	}
	if fromString.ClusterWide != "true" {
		t.Fatalf("expected string true, got %q", fromString.ClusterWide)
	}

	var fromBool ListRoomsInput
	if err := json.Unmarshal([]byte(`{"cluster_wide":true}`), &fromBool); err != nil {
		t.Fatalf("boolean cluster_wide should decode: %v", err)
	}
	if fromBool.ClusterWide != "true" {
		t.Fatalf("expected boolean true to coerce to string true, got %q", fromBool.ClusterWide)
	}
}
