package application

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// mergeResolverAuthEnvFromHost copies env vars listed in the isolation profile's DOCKPIPE_*_ENV
// hint from src (host + workflow env) into dst (docker -e) when dst does not already set a value.
func mergeResolverAuthEnvFromHost(dst, src map[string]string, ra *domain.ResolverAssignments) {
	if ra == nil {
		return
	}
	mergeEnvHintKeys(dst, src, ra.EnvHint)
	if key := strings.TrimSpace(ra.AuthDirEnv); key != "" {
		if containerDir := strings.TrimSpace(ra.ContainerAuthDir); containerDir != "" && strings.TrimSpace(dst[key]) == "" {
			dst[key] = containerDir
		}
	}
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

func resolverAuthMountSpecs(ra *domain.ResolverAssignments, envMap map[string]string) []string {
	if ra == nil {
		return nil
	}
	mode := strings.TrimSpace(ra.AuthMountMode)
	if mode == "" {
		mode = "rw"
	}
	var mounts []string
	if hostDir := resolverHostPath(strings.TrimSpace(ra.AuthDirEnv), strings.TrimSpace(ra.AuthDir), envMap); hostDir != "" {
		if containerDir := strings.TrimSpace(ra.ContainerAuthDir); containerDir != "" {
			if st, err := os.Stat(hostDir); err == nil && st.IsDir() {
				mounts = append(mounts, hostDir+":"+containerDir+":"+mode)
			}
		}
	}
	if hostFile := resolverHostPath(strings.TrimSpace(ra.ConfigFileEnv), strings.TrimSpace(ra.ConfigFile), envMap); hostFile != "" {
		if containerFile := strings.TrimSpace(ra.ContainerConfigFile); containerFile != "" {
			if st, err := os.Stat(hostFile); err == nil && !st.IsDir() {
				mounts = append(mounts, hostFile+":"+containerFile+":"+mode)
			}
		}
	}
	return mounts
}

func resolverHostPath(envKey, fallback string, envMap map[string]string) string {
	if envKey != "" {
		if v := strings.TrimSpace(envMap[envKey]); v != "" {
			return cleanResolverHostPath(v)
		}
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return cleanResolverHostPath(v)
		}
	}
	if fallback == "" {
		return ""
	}
	return cleanResolverHostPath(fallback)
}

func cleanResolverHostPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "~") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if value == "~" {
				value = home
			} else if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
				value = filepath.Join(home, value[2:])
			}
		}
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Clean(filepath.Join(home, value))
	}
	return filepath.Clean(value)
}
