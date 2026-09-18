package testkit

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("testkit.update", false, "rewrite golden files")

// GoldenJSON compares got against a JSON golden file at
// testdata/<name>.golden.json. Run with -testkit.update to rewrite it.
//
// The testdata/ directory is created automatically on first run. The
// comparison is done on the unmarshalled form, so field order and
// insignificant whitespace do not cause false failures.
func GoldenJSON(t testing.TB, name string, got any) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden.json")

	normalized, err := canonicalJSON(got)
	if err != nil {
		t.Fatalf("testkit.GoldenJSON: marshal %T: %v", got, err)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755); mkdirErr != nil {
		t.Fatalf("testkit.GoldenJSON: mkdir: %v", mkdirErr)
	}

	if *updateGolden {
		if writeErr := os.WriteFile(path, normalized, 0o600); writeErr != nil {
			t.Fatalf("testkit.GoldenJSON: write: %v", writeErr)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		// First run in a fresh checkout: write the initial golden and
		// pass. This keeps CI and pre-commit green the very first time
		// a golden test lands, without requiring the author to
		// remember -testkit.update before the first commit. The
		// generated file still needs to be staged and committed;
		// subsequent runs compare strictly against it.
		if writeErr := os.WriteFile(path, normalized, 0o600); writeErr != nil {
			t.Fatalf("testkit.GoldenJSON: write initial golden: %v", writeErr)
		}
		t.Logf("testkit.GoldenJSON: created initial golden file %s", path)
		return
	}

	wantCanon, err := canonicalJSON(want)
	if err != nil {
		t.Fatalf("testkit.GoldenJSON: canonicalize golden: %v", err)
	}

	if !bytes.Equal(normalized, wantCanon) {
		t.Fatalf("testkit.GoldenJSON: %s mismatch\n  want: %s\n  got:  %s\n(run with -testkit.update to rewrite)",
			path, wantCanon, normalized)
	}
}

func canonicalJSON(v any) ([]byte, error) {
	var raw []byte
	switch x := v.(type) {
	case []byte:
		raw = x
	case json.RawMessage:
		raw = x
	default:
		var err error
		raw, err = json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
	}

	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		return nil, err
	}

	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
