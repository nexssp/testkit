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

				payload := smokePayload(act)

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

// smokePayload prefers the action's declared Example. Only when the
// author did not provide one does it guess from the request struct.
func smokePayload(act action.AnyAction) any {
	if ex := act.Describe().Example; ex != nil {
		return ex
	}
	if typed, ok := act.(action.TypedPayload); ok {
		return generateMinimalPayload(typed.ReqPayload())
	}
	return struct{}{}
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
	for f := range rt.Fields() {
		jsonTag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if jsonTag == "" || jsonTag == "-" {
			jsonTag = strings.ToLower(f.Name)
		}

		// The remaining reflect.Kind values (structs, slices, maps,
		// channels, functions, interfaces, complex numbers, unsafe
		// pointers, arrays) are deliberately omitted: synthesizing a
		// wrong shape is worse than omitting the field, and a caller
		// who wants a specific shape should set act.Meta.Example.
		//
		//nolint:exhaustive // unsupported kinds are intentionally omitted
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
		default:
			// See comment above.
		}
	}
	return fields
}
