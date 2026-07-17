package appserversupervisor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"dorkpipe.orchestrator/providersession"
)

const (
	maxSupervisorDeadline   = 5 * time.Minute
	maxInitializationValues = 32
	maxRequiredCapabilities = 16
	maxLifecycleRoots       = 16
	maxLocalPathBytes       = 1024
)

// validateSupervisorSession is deliberately stricter than the neutral
// contract. Private state, snapshots, and audit evidence must use identifiers
// that are bounded and safe to serialize locally.
func validateSupervisorSession(session providersession.SessionRef) error {
	if err := session.Validate(); err != nil || !validID(session.Provider) || !validID(session.SessionID) {
		return errors.New("bounded supervisor session identity is required")
	}
	return nil
}

func boundedLocalPath(path string) (string, bool) {
	if strings.TrimSpace(path) != path || path == "" || len(path) > maxLocalPathBytes || !filepath.IsAbs(path) {
		return "", false
	}
	clean := filepath.Clean(path)
	if clean != path {
		return "", false
	}
	return clean, true
}

func containedPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." {
		return err == nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// objectFields rejects arrays, scalars, duplicate keys, and unknown keys. It
// validates shape only; callers still decide which safe fields they project.
func objectFields(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, bool) {
	if !uniqueJSONObjectKeys(raw) {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if len(raw) == 0 || !json.Valid(raw) || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false
	}
	permitted := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		permitted[name] = struct{}{}
	}
	for name := range fields {
		if _, found := permitted[name]; !found {
			return nil, false
		}
	}
	return fields, true
}

func uniqueJSONObjectKeys(raw json.RawMessage) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

func nestedObjectFields(raw json.RawMessage, allowed ...string) bool {
	_, ok := objectFields(raw, allowed...)
	return ok
}
