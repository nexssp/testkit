package testkit_test

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
	"github.com/nexssp/transport/thttp"
)

type ValidPayload struct {
	Name string `json:"name"`
}

func TestContract_ValidActionsPass(t *testing.T) {
	t.Parallel()

	act1 := action.New("user.create", func(_ context.Context, req ValidPayload) (string, error) {
		return "ok", nil
	}).Route(thttp.POST("/users")).Build()

	act2 := action.New("user.get", func(_ context.Context, req ValidPayload) (string, error) {
		return "ok", nil
	}).Route(thttp.GET("/users")).Build()

	// Should pass without failing subtests
	testkit.AssertContracts(t, []action.AnyAction{act1, act2})
}
