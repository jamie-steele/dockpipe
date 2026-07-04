package infrastructure

import (
	"os"
)

// ParseEnvFile reads KEY=VAL lines (dotenv-style). Skips comments and blanks.
func ParseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseEnvReader(f)
}
