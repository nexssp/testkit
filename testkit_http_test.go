package testkit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xctx"
	"github.com/nexssp/kernel/xerr"
	"github.com/nexssp/testkit"
	"github.com/nexssp/transport/thttp"
)

type ComplexOrderDTO struct {
	ID       string `json:"id" path:"id"`
	Customer struct {
		Name string `json:"name"`
		Tier string `json:"tier"`
	} `json:"customer"`
	Items []struct {
		SKU string  `json:"sku"`
		Qty int     `json:"qty"`
		Val float64 `json:"val"`
	} `json:"items"`
}

func TestHTTP_CompleteFluentDSL(t *testing.T) {
	t.Parallel()

	// 1. Setup Action with deep nested JSON
	getOrder := action.New("order.get_complex", func(_ context.Context, req ComplexOrderDTO) (ComplexOrderDTO, error) {
		res := ComplexOrderDTO{
			ID: req.ID,
		}
		res.Customer.Name = "Acme Corp"
		res.Customer.Tier = "Enterprise"
		res.Items = []struct {
			SKU string  `json:"sku"`
			Qty int     `json:"qty"`
			Val float64 `json:"val"`
		}{
			{SKU: "SERVER-RACK", Qty: 2, Val: 4500.00},
			{SKU: "CABLE-OPTIC", Qty: 10, Val: 35.50},
		}
		return res, nil
	}).Route(thttp.GET("/api/v1/orders/{id}")).Build()

	suite := testkit.New(t, getOrder)

	// 2. Test Path Param + Nested Dot-Path Traversal + Array Indexing
	suite.GET("/api/v1/orders/ORD-999").
		Do().
		ExpectOK().
		ExpectHeader("Content-Type", "application/json").
		HasField("id", "ORD-999").
		HasField("customer.name", "Acme Corp").
		HasField("customer.tier", "Enterprise").
		HasField("items.0.sku", "SERVER-RACK").
		HasField("items.0.qty", 2).
		HasField("items.1.sku", "CABLE-OPTIC").
		ExpectArrayLen("items", 2).
		ContainsString("Acme Corp")
}

func TestHTTP_QueryParamsAndFormURLEncoded(t *testing.T) {
	t.Parallel()

	type SearchReq struct {
		Query string `query:"q"`
		Page  int    `query:"page"`
		Sort  string `query:"sort"`
	}

	searchAct := action.New("search.query", func(_ context.Context, req SearchReq) (map[string]any, error) {
		return map[string]any{
			"q":    req.Query,
			"page": req.Page,
			"sort": req.Sort,
		}, nil
	}).Route(thttp.GET("/search")).Build()

	suite := testkit.New(t, searchAct)

	// Test WithQuery & WithQueries
	suite.GET("/search").
		WithQuery("q", "golang").
		WithQueries(map[string]string{"page": "3", "sort": "desc"}).
		Do().
		ExpectOK().
		HasField("q", "golang").
		HasField("page", 3).
		HasField("sort", "desc")
}

func TestHTTP_MultipartFileUpload(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("document")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		content, _ := io.ReadAll(file)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"filename": header.Filename,
			"bytes":    len(content),
			"size":     123456,
		})
	})

	suite := testkit.NewWithHandler(t, mux)

	suite.POST("/upload").
		WithMultipartFile("document", "contract.pdf", []byte("123456")).
		Do().
		ExpectCreated().
		HasField("filename", "contract.pdf").
		HasField("bytes", 6).
		HasField("size", 123456)
}

func TestHTTP_CookiesAndHeadersInheritance(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/check", func(w http.ResponseWriter, r *http.Request) {
		sessCookie, err := r.Cookie("session_id")
		if err != nil || sessCookie.Value != "sess-active" {
			http.Error(w, `{"error":"missing cookie"}`, http.StatusUnauthorized)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer secret-token" {
			http.Error(w, `{"error":"missing auth"}`, http.StatusForbidden)
			return
		}

		tenant := r.Header.Get(testkit.HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "authorized",
			"tenant": tenant,
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	suite := testkit.NewWithHandler(t, mux)

	// 1. Without credentials -> 401
	suite.GET("/auth/check").Do().ExpectUnauthorized()

	// 2. Set Global Cookies & Headers
	suite.WithCookie("session_id", "sess-active").
		WithGlobalBearerToken("secret-token")

	suite.GET("/auth/check").
		WithTenant("tenant-corp-eu").
		Do().
		ExpectOK().
		HasField("status", "authorized").
		HasField("tenant", "tenant-corp-eu")

	// 3. Reset Headers
	suite.ResetHeaders()
	suite.GET("/auth/check").Do().ExpectUnauthorized()
}

func TestHTTP_ErrorResponsesAndStatusExpectations(t *testing.T) {
	t.Parallel()

	failingAct := action.New("failing.action", func(_ context.Context, req struct{ Reason string }) (struct{}, error) {
		switch req.Reason {
		case "bad_input":
			return struct{}{}, xerr.BadRequest("invalid field values")
		case "forbidden":
			return struct{}{}, xerr.Forbidden("insufficient permissions")
		case "not_found":
			return struct{}{}, xerr.NotFound("entity does not exist")
		default:
			return struct{}{}, xerr.Internal("unexpected system crash")
		}
	}).Route(thttp.POST("/fail")).Build()

	suite := testkit.New(t, failingAct)

	// Test 400 Bad Request
	suite.POST("/fail", map[string]string{"Reason": "bad_input"}).
		Do().
		ExpectBadRequest().
		ExpectErrorKind(xerr.KindBadRequest, "invalid field values")

	// Test 403 Forbidden
	suite.POST("/fail", map[string]string{"Reason": "forbidden"}).
		Do().
		ExpectForbidden().
		ExpectErrorKind(xerr.KindForbidden, "insufficient permissions")

	// Test 404 Not Found
	suite.POST("/fail", map[string]string{"Reason": "not_found"}).
		Do().
		ExpectNotFound().
		ExpectErrorKind(xerr.KindNotFound, "entity does not exist")
}

func TestHTTP_ContextBridgeTenantAndUser(t *testing.T) {
	t.Parallel()

	checkContextAct := action.New("context.check", func(ctx context.Context, _ struct{}) (map[string]string, error) {
		return map[string]string{
			"tenant_id": xctx.TenantIDFrom(ctx),
			"user_id":   xctx.UserIDFrom(ctx),
		}, nil
	}).Route(thttp.GET("/context")).Build()

	suite := testkit.New(t, checkContextAct)

	suite.GET("/context").
		WithTenant("acme-tenant").
		WithBearerToken("mock-user-token").
		Do().
		ExpectOK().
		HasField("tenant_id", "acme-tenant").
		HasField("user_id", "test_user") // Injected automatically by contextBridgeMiddleware
}

func TestSuite_E2EMode(t *testing.T) {
	t.Parallel()

	// 1. Simulate an external live staging service on a remote port
	liveRemoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret-staging-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"environment":"staging","status":"healthy"}`))
	}))
	t.Cleanup(liveRemoteServer.Close)

	// 2. Initialize testkit in E2E mode against the remote URL
	suite := testkit.NewE2E(t, liveRemoteServer.URL)

	// 3. Execute assertions over real network calls
	suite.GET("/status").
		WithHeader("X-Api-Key", "secret-staging-key").
		Do().
		ExpectOK().
		HasField("environment", "staging").
		HasField("status", "healthy")
}
