package domain

import "strings"

// runtime.type (DOCKPIPE_RUNTIME_TYPE): three behavior classifications only — not Docker vs EC2, not tool names.
// See docs/architecture-model.md (normative).
const (
	RuntimeKindExecution = "execution" // non-interactive command/test execution
	RuntimeKindIDE       = "ide"       // interactive development environment
	RuntimeKindAgent     = "agent"     // autonomous task execution
)

// ValidRuntimeKinds lists accepted DOCKPIPE_RUNTIME_TYPE values.
var ValidRuntimeKinds = []string{
	RuntimeKindExecution,
	RuntimeKindIDE,
	RuntimeKindAgent,
}

// IsValidRuntimeKind reports whether s is one of the three runtime.type values (after trim).
func IsValidRuntimeKind(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, k := range ValidRuntimeKinds {
		if s == k {
			return true
		}
	}
	return false
}
