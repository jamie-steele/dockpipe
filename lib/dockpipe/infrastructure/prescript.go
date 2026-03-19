package infrastructure

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// SourceHostScript runs a bash script with set -a (export all) and returns the resulting environment as a map.
func SourceHostScript(scriptPath string, env []string) (map[string]string, error) {
	cmd := exec.Command("bash", "-c", `set -euo pipefail; set -a; source "$1"; env -0`, "dockpipe-prescript", scriptPath)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pre-script %s: %w\n%s", scriptPath, err, out)
	}
	return parseEnv0(out), nil
}

func parseEnv0(data []byte) map[string]string {
	m := make(map[string]string)
	for _, chunk := range bytes.Split(data, []byte{0}) {
		if len(chunk) == 0 {
			continue
		}
		line := string(chunk)
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}
