// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package kctl

import (
	"reflect"
	"testing"
)

func testNamespaces() map[string]map[string]string {
	return map[string]map[string]string{
		"ns-one":   {"team-label": "team-a"},
		"ns-two":   {"team-label": "team-b"},
		"ns-three": {"team-label": "team-a"},
		"ns-four":  {}, // no label at all
	}
}

func TestDistinctLabelValues(t *testing.T) {
	got := DistinctLabelValues(testNamespaces(), "team-label")
	want := []string{"team-a", "team-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDistinctLabelValues_UnknownKey(t *testing.T) {
	got := DistinctLabelValues(testNamespaces(), "does-not-exist")
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestNamespacesForLabelValue(t *testing.T) {
	got := NamespacesForLabelValue(testNamespaces(), "team-label", "team-a")
	want := []string{"ns-one", "ns-three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNamespacesForLabelValue_NoMatch(t *testing.T) {
	got := NamespacesForLabelValue(testNamespaces(), "team-label", "team-z")
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}
