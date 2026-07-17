package application

import (
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"os"
	"path/filepath"
	"strings"
)

func envSliceWithScriptContext(env []string, scriptAbs string) []string {
	if strings.TrimSpace(scriptAbs) == "" {
		return env
	}
	envMap := domain.EnvSliceToMap(env)
	for k, v := range scriptContextEnv(scriptAbs) {
		if strings.TrimSpace(v) == "" {
			delete(envMap, k)
			continue
		}
		envMap[k] = v
	}
	return domain.EnvMapToSlice(envMap)
}

func scriptContextEnv(scriptAbs string) map[string]string {
	out := map[string]string{}
	scriptDir := filepath.Dir(scriptAbs)
	if abs, err := filepath.Abs(scriptDir); err == nil {
		scriptDir = abs
	}
	out["DOCKPIPE_SCRIPT_DIR"] = scriptDir
	if pkgRoot := nearestPackageRoot(scriptDir); pkgRoot != "" {
		out["DOCKPIPE_PACKAGE_ROOT"] = pkgRoot
	}
	if assetsDir := nearestAssetsDir(scriptDir); assetsDir != "" {
		out["DOCKPIPE_ASSETS_DIR"] = assetsDir
	}
	return out
}

func nearestPackageRoot(start string) string {
	for cur := start; cur != ""; {
		manifest := filepath.Join(cur, infrastructure.PackageManifestFilename)
		if st, err := os.Stat(manifest); err == nil && !st.IsDir() {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}
	return ""
}

func nearestAssetsDir(start string) string {
	for cur := start; cur != ""; {
		if filepath.Base(cur) == "assets" {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}
	return ""
}
