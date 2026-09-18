package testkit

import (
	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit/chaos"
)

// WithChaos wraps provided actions with chaos fault injection.
//
// This is the only function in package testkit that references
// testkit/chaos. Keeping it in its own file makes the dependency
// boundary explicit: any reader of the package can see at a glance
// that chaos is opt-in, not part of the core Suite.
func WithChaos(actions []action.AnyAction, cfg chaos.Config) []action.AnyAction {
	hook := chaos.GlobalHook(cfg)
	for _, act := range actions {
		act.AddAnyHook(hook)
	}
	return actions
}
