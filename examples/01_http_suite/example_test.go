// Package httpsuite demonstrates the fluent HTTP test suite.
//
// The test below is intentionally what you would write at the top of a
// real handler's test file. There is no manual httptest.NewServer, no
// manual request building, no manual JSON decoding.
package httpsuite

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nexssp/testkit"
)

func TestFluentHTTPSuite(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "usr_42",
			"name":    in.Name,
			"tenant":  r.Header.Get(testkit.HeaderTenantID),
			"created": true,
		})
	})

	suite := testkit.NewWithHandler(t, mux)

	suite.POST("/api/v1/users", map[string]string{"name": "Ada"}).
		WithTenant("acme").
		Do().
		ExpectCreated().
		ExpectHeader("Content-Type", "application/json").
		HasField("id", "usr_42").
		HasField("name", "Ada").
		HasField("tenant", "acme").
		HasField("created", true)
}
