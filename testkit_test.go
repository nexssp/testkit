// nexssp/testkit/testkit_test.go
package testkit_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xerr"
	"github.com/nexssp/testkit"
	"github.com/nexssp/transport/thttp"
)

type UserDTO struct {
	ID    string `json:"id" path:"id"`
	Email string `json:"email"`
}

// ── Test 1: Action-Mounted Suite Integration ──────────────────────────────────
func TestSuite_WithActions(t *testing.T) {
	t.Parallel()

	getUser := action.New("user.get", func(_ context.Context, req UserDTO) (UserDTO, error) {
		return UserDTO{ID: req.ID, Email: "admin@nexss.com"}, nil
	}).Route(thttp.GET("/users/{id}")).Build()

	suite := testkit.New(t, getUser)

	suite.GET("/users/usr-100").
		Do().
		ExpectOK().
		HasField("id", "usr-100").
		HasField("email", "admin@nexss.com")
}

// ── Test 2: Plain http.Handler Standalone Testing (No Actions Needed!) ────────
func TestSuite_StandaloneHttpHandler(t *testing.T) {
	t.Parallel()

	// Standard Go http.ServeMux — works identically with Chi, Gin, Echo!
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","uptime_sec":120}`))
	})

	suite := testkit.NewWithHandler(t, mux)

	suite.GET("/healthz").
		Do().
		ExpectOK().
		HasField("status", "healthy").
		HasField("uptime_sec", 120)
}

// ── Test 3: Automated Smoke Testing ──────────────────────────────────────────
func TestSmoke(t *testing.T) {
	t.Parallel()

	act1 := action.New("health", func(_ context.Context, _ struct{}) (string, error) {
		return "ok", nil
	}).Route(thttp.GET("/health")).Build()

	act2 := action.New("create.user", func(_ context.Context, req UserDTO) (UserDTO, error) {
		return req, nil
	}).Route(thttp.POST("/users")).Build()

	testkit.RunSmokeTests(t, []action.AnyAction{act1, act2})
}

// ── Test 4: Script & Recorder ────────────────────────────────────────────────
func TestScriptAndRecorder(t *testing.T) {
	t.Parallel()

	// Use xerr.Unavailable so the retry engine recognizes it as a transient failure
	scripted := testkit.Script[int, string](
		testkit.Failure[string](xerr.Unavailable("transient-error")),
		testkit.Success("recovered"),
	)

	rec := new(testkit.Recorder[int, string])
	act := action.New("retry.test", scripted).
		Retry(2, action.ConstantBackoff(0)).
		Build()

	act.AddAnyHook(action.AnyHook{
		OnRetry: func(ctx context.Context, req any, attempt int, err error, _ *action.Meta) {
			rec.OnRetry(ctx, req.(int), attempt, err) // <-- use inherited ctx
		},
	})

	res, err := act.Do(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "recovered" {
		t.Fatalf("expected 'recovered', got %q", res)
	}
	if len(rec.Retries) != 1 || rec.Retries[0].Attempt != 1 {
		t.Fatalf("expected 1 retry recorded, got %+v", rec.Retries)
	}
}
