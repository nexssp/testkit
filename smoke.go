package testkit

import (
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/transport/thttp"
)

func RunSmokeTests(t *testing.T, actions []action.AnyAction) {
	t.Helper()
	suite := New(t, actions)
	pathParamRegex := regexp.MustCompile(`\{[^}]+\}`)

	for _, act := range actions {
		meta := act.Describe()

		for _, bind := range act.GetBindings() {
			var method, rawPath string
			switch r := bind.(type) {
			case thttp.HTTPRoute:
				method, rawPath = r.Method, r.Path
			case *thttp.HTTPRoute:
				if r != nil {
					method, rawPath = r.Method, r.Path
				}
			default:
				continue
			}

			if method == "" || rawPath == "" {
				continue
			}

			t.Run(meta.Name+"_"+method, func(t *testing.T) {
				path := pathParamRegex.ReplaceAllString(rawPath, "test-id")

				var payload any = struct{}{}
				if typed, ok := act.(action.TypedPayload); ok {
					payload = generateMinimalPayload(typed.ReqPayload())
				}

				req := suite.Request(method, path)
				if method != http.MethodGet && method != http.MethodDelete && method != http.MethodHead {
					req.WithJSON(payload)
				}

				res := req.Do()
				if res.Status() >= 500 {
					t.Fatalf("🚨 Smoke test failed for %s [%s %s]\nStatus: %d\nBody: %s",
						meta.Name, method, path, res.Status(), res.BodyString())
				}
			})
		}
	}
}

func generateMinimalPayload(t any) map[string]any {
	if t == nil {
		return nil
	}
	rt := reflect.TypeOf(t)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
	}

	fields := make(map[string]any)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		jsonTag := strings.Split(f.Tag.Get("json"), ",")[0]
		if jsonTag == "" || jsonTag == "-" {
			jsonTag = strings.ToLower(f.Name)
		}

		switch f.Type.Kind() {
		case reflect.String:
			if strings.Contains(f.Tag.Get("validate"), "email") {
				fields[jsonTag] = "test@example.com"
			} else {
				fields[jsonTag] = "test_value"
			}
		case reflect.Int, reflect.Int64:
			fields[jsonTag] = 1
		case reflect.Float64:
			fields[jsonTag] = 1.0
		case reflect.Bool:
			fields[jsonTag] = true
		}
	}
	return fields
}
