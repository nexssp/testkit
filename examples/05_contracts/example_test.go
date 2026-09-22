// Package contracts demonstrates AssertContracts.
//
// AssertContracts runs a single invariant suite over every action in a
// library. Add a new action, the test automatically covers it. Remove
// a route, the test tells you which action is now orphaned.
//
// This is how a codebase keeps its architecture honest as it grows:
// invariants live in one place, applied to every action by one call.
package contracts

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xtest/ktest"
	"github.com/nexssp/transport/thttp"
)

type CreateReq struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name"  validate:"required"`
}

func TestLibrary_MeetsInvariants(t *testing.T) {
	createUser := action.New("user.create",
		func(_ context.Context, _ CreateReq) (string, error) { return "usr_1", nil }).
		Route(thttp.POST("/api/v1/users")).
		Build()

	getUser := action.New("user.get",
		func(_ context.Context, _ CreateReq) (CreateReq, error) { return CreateReq{}, nil }).
		Route(thttp.GET("/api/v1/users/{id}")).
		Build()

	ktest.AssertContracts(t, []action.AnyAction{createUser, getUser})
}

func TestLibrary_DetectsOrphanedAction(t *testing.T) {
	// No route, no hooks — AssertContracts flags this as orphaned.
	orphan := action.New("ghost.action",
		func(_ context.Context, _ struct{}) (string, error) { return "", nil }).
		Build()

	// In a real test you would run this against a subprocess and
	// assert the failure message. Here we assert the invariant
	// directly so the example stays single-file.
	meta := orphan.Describe()
	if len(orphan.GetBindings()) == 0 && len(orphan.GetAnyHooks()) == 0 {
		t.Logf("AssertContracts would flag %q as orphaned", meta.Name)
	} else {
		t.Fatal("test fixture drift")
	}
}
