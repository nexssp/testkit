package testkit

import (
	"reflect"
	"testing"

	"github.com/nexssp/kernel/action"
)

// AssertContracts verifies architectural invariants across all registered actions.
// Catches duplicate action names, orphaned actions without routes/hooks, and malformed DTOs.
func AssertContracts(t *testing.T, actions []action.AnyAction) {
	t.Helper()

	names := make(map[string]bool)

	for _, act := range actions {
		meta := act.Describe()

		t.Run(meta.Name, func(t *testing.T) {
			if names[meta.Name] {
				t.Errorf("Duplicate action name detected: %q", meta.Name)
			}
			names[meta.Name] = true

			if len(act.GetBindings()) == 0 && len(act.GetAnyHooks()) == 0 {
				t.Errorf("Action %q is orphaned (no route bindings and no hooks attached)", meta.Name)
			}

			if typed, ok := act.(action.TypedPayload); ok {
				req := typed.ReqPayload()
				if req != nil {
					rt := reflect.TypeOf(req)
					for rt.Kind() == reflect.Pointer {
						rt = rt.Elem()
					}
					if rt.Kind() != reflect.Struct && rt.Kind() != reflect.Map {
						t.Errorf("Action %q payload must be struct or map, got %v", meta.Name, rt.Kind())
					}
				}
			}
		})
	}
}
