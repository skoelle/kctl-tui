package kctl

import "testing"

func TestFindNextContext_TwoWayToggle(t *testing.T) {
	pairs := []ContextPair{
		{Name: "env-pair-1", Contexts: []string{"ctx-a", "ctx-b"}},
	}

	next, ok := FindNextContext("ctx-a", pairs)
	if !ok || next != "ctx-b" {
		t.Fatalf("expected ctx-b, got %q (ok=%v)", next, ok)
	}

	next, ok = FindNextContext("ctx-b", pairs)
	if !ok || next != "ctx-a" {
		t.Fatalf("expected ctx-a, got %q (ok=%v)", next, ok)
	}
}

func TestFindNextContext_Rotation(t *testing.T) {
	pairs := []ContextPair{
		{Name: "rotation", Contexts: []string{"a", "b", "c"}},
	}

	next, ok := FindNextContext("c", pairs)
	if !ok || next != "a" {
		t.Fatalf("expected wraparound to a, got %q (ok=%v)", next, ok)
	}
}

func TestFindNextContext_NotConfigured(t *testing.T) {
	pairs := []ContextPair{
		{Name: "env-pair-1", Contexts: []string{"ctx-a", "ctx-b"}},
	}

	_, ok := FindNextContext("unrelated-context", pairs)
	if ok {
		t.Fatalf("expected ok=false for a context with no configured pair")
	}
}

func TestFindNextContext_NoPairsConfigured(t *testing.T) {
	_, ok := FindNextContext("ctx-a", nil)
	if ok {
		t.Fatalf("expected ok=false when no pairs are configured")
	}
}
