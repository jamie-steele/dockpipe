package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Namespace identifies the author or org for a workflow or resolver package. Optional in YAML;
// when set it must be a single DNS-like label (lowercase, hyphens) and must not be a reserved word.
const maxNamespaceLen = 63

var namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// reservedNamespaces are blocked for user-facing namespace (engine, store, and ambiguous terms).
var reservedNamespaces = map[string]struct{}{
	"dockpipe": {}, "dorkpipe": {}, "pipeon": {},
	"system": {}, "internal": {}, "reserved": {}, "public": {}, "local": {}, "global": {},
	"core": {}, "default": {}, "engine": {}, "platform": {}, "admin": {}, "root": {},
	"api": {}, "cli": {}, "meta": {}, "vendor": {}, "test": {}, "staging": {},
	"bundle": {}, "bundles": {}, "workflow": {}, "workflows": {}, "resolver": {}, "resolvers": {},
	"null": {}, "true": {}, "false": {},
	"package": {}, "packages": {}, "store": {},
	"template": {}, "templates": {}, "assets": {}, "runtimes": {}, "strategies": {},
}

// IsReservedNamespace reports whether s matches a reserved namespace (case-insensitive).
func IsReservedNamespace(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	_, ok := reservedNamespaces[s]
	return ok
}

// ValidateNamespace checks an optional namespace string from workflow YAML, package.yml, or resolver.yaml.
// Empty or whitespace-only is allowed (field omitted). Non-empty values must match namespacePattern
// and must not be reserved.
func ValidateNamespace(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > maxNamespaceLen {
		return fmt.Errorf("namespace %q exceeds max length %d", s, maxNamespaceLen)
	}
	if !namespacePattern.MatchString(s) {
		return fmt.Errorf("namespace %q must match %s (lowercase label, e.g. acme, my-team)", s, namespacePattern.String())
	}
	if IsReservedNamespace(s) {
		return fmt.Errorf("namespace %q is reserved — choose another label (not engine/store names like dockpipe or core)", s)
	}
	return nil
}
