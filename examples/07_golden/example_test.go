// Package golden demonstrates GoldenJSON.
//
// Run first time:          go test ./... -run TestGolden -v
// The test fails, telling you to create the file.
//
// Create / rewrite:        go test ./... -run TestGolden -testkit.update
// Then review the diff in git.
//
// Normal run:              go test ./...
// Compares against the committed file.
//
// The comparison is done on the unmarshalled form, so reordering keys
// or changing whitespace does not cause false failures. Only a real
// change in the data fails the test — and the failure shows both sides.
package golden

import (
	"testing"

	"github.com/nexssp/testkit"
)

type APIResponse struct {
	Tenant    string   `json:"tenant"`
	UserCount int      `json:"user_count"`
	Features  []string `json:"features"`
}

func TestGolden_SnapshotAPIResponse(t *testing.T) {
	got := APIResponse{
		Tenant:    "acme-corp",
		UserCount: 42,
		Features:  []string{"sso", "audit", "webhooks"},
	}

	testkit.GoldenJSON(t, "api_response", got)
}
