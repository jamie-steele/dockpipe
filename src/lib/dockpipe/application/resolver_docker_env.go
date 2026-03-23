package application

import (
	"strings"

	"dockpipe/lib/dockpipe/domain"
)

// mergeResolverAuthEnvFromHost copies env vars listed in the isolation profile's DOCKPIPE_*_ENV
// hint from src (host + workflow env) into dst (docker -e) when dst does not already set a value.
func mergeResolverAuthEnvFromHost(dst, src map[string]string, ra *domain.ResolverAssignments) {
	if ra == nil {
		return
	}
	mergeEnvHintKeys(dst, src, ra.EnvHint)
}

func mergeEnvHintKeys(dst, src map[string]string, envHint string) {
	for _, k := range domain.EnvVarNamesFromHint(envHint) {
		if strings.TrimSpace(dst[k]) != "" {
			continue
		}
		if v := strings.TrimSpace(src[k]); v != "" {
			dst[k] = v
		}
	}
}
