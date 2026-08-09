// Copyright (c) 2026 Stefan Koelle (https://stefankoelle.de)
// Licensed under the MIT License. See LICENSE file in project root for details.

package kctl

import "testing"

func TestResolveTemplate_SinglePlaceholder(t *testing.T) {
	got := ResolveTemplate("secret-{namespace}", map[string]string{"namespace": "example-ns"})
	want := "secret-example-ns"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTemplate_MultiplePlaceholders(t *testing.T) {
	template := "arn:aws:eks:{region}:{account_id}:cluster/tf-{env}-{context}-1"
	values := map[string]string{
		"region":     "eu-central-1",
		"account_id": "123456789012",
		"env":        "beta",
		"context":    "internal",
	}
	got := ResolveTemplate(template, values)
	want := "arn:aws:eks:eu-central-1:123456789012:cluster/tf-beta-internal-1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTemplate_UnknownPlaceholderLeftAsIs(t *testing.T) {
	got := ResolveTemplate("secret-{unknown}", map[string]string{"namespace": "example-ns"})
	want := "secret-{unknown}"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveTemplate_EmptyTemplate(t *testing.T) {
	got := ResolveTemplate("", map[string]string{"namespace": "example-ns"})
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}
