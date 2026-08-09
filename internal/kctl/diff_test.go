package kctl

import "testing"

func TestDiffSecretValues_AllMatch(t *testing.T) {
	left := map[string]string{"a": "1", "b": "2"}
	right := map[string]string{"a": "1", "b": "2"}

	entries := DiffSecretValues(left, right)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if AnyMismatch(entries) {
		t.Fatalf("expected no mismatch, got %v", entries)
	}
}

func TestDiffSecretValues_Mismatch(t *testing.T) {
	left := map[string]string{"a": "1", "b": "2"}
	right := map[string]string{"a": "1", "b": "different"}

	entries := DiffSecretValues(left, right)
	if !AnyMismatch(entries) {
		t.Fatalf("expected a mismatch, got %v", entries)
	}

	var bEntry *SecretDiffEntry
	for i := range entries {
		if entries[i].Key == "b" {
			bEntry = &entries[i]
		}
	}
	if bEntry == nil || bEntry.Match {
		t.Fatalf("expected key 'b' to be a mismatch, got %v", bEntry)
	}
}

func TestDiffSecretValues_KeyOnlyOnOneSide(t *testing.T) {
	left := map[string]string{"a": "1", "only-left": "x"}
	right := map[string]string{"a": "1", "only-right": "y"}

	entries := DiffSecretValues(left, right)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (union of keys), got %d: %v", len(entries), entries)
	}
	if !AnyMismatch(entries) {
		t.Fatalf("expected mismatch due to keys only present on one side")
	}
}

func TestDiffSecretValues_EmptyMaps(t *testing.T) {
	entries := DiffSecretValues(nil, nil)
	if len(entries) != 0 {
		t.Fatalf("expected no entries for empty maps, got %v", entries)
	}
	if AnyMismatch(entries) {
		t.Fatalf("expected no mismatch for empty maps")
	}
}

func TestIsBinary_Plaintext(t *testing.T) {
	if IsBinary("hello world") {
		t.Fatal("expected plaintext to not be binary")
	}
	if IsBinary("line1\nline2\ttab") {
		t.Fatal("expected newline/tab to not be binary")
	}
}

func TestIsBinary_BinaryData(t *testing.T) {
	if !IsBinary("hello\x00world") {
		t.Fatal("expected null byte to be binary")
	}
	if !IsBinary("key=\xff\xfe") {
		t.Fatal("expected non-ASCII bytes to be binary")
	}
}

func TestDiffSecretValues_BinaryDetection(t *testing.T) {
	left := map[string]string{"ok": "text", "bin": "data\x00here"}
	right := map[string]string{"ok": "text", "bin": "data\x00here"}

	entries := DiffSecretValues(left, right)
	for _, e := range entries {
		if e.Key == "bin" {
			if !e.LeftBin || !e.RightBin {
				t.Fatalf("expected binary flags set for key 'bin', got LeftBin=%v RightBin=%v", e.LeftBin, e.RightBin)
			}
			if !e.Match {
				t.Fatalf("expected binary values to match")
			}
		}
		if e.Key == "ok" {
			if e.LeftBin || e.RightBin {
				t.Fatalf("expected binary flags unset for key 'ok'")
			}
		}
	}
}
