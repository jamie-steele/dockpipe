// Package composegen writes docker-compose snippets for local postgres+pgvector and optional Ollama.
package composegen

import (
	"fmt"
	"os"
)

// Options tune generated compose.
type Options struct {
	PostgresPassword string
	PostgresDB       string
	PostgresUser     string
	PostgresPort     string
	OllamaPort       string
	IncludeOllama    bool
}

// DefaultOptions returns sensible dev defaults.
func DefaultOptions() Options {
	return Options{
		PostgresPassword: "dorkpipe",
		PostgresDB:       "dorkpipe",
		PostgresUser:     "dorkpipe",
		PostgresPort:     "15432",
		OllamaPort:       "11434",
		IncludeOllama:    true,
	}
}

// Render returns docker-compose YAML (v3, no version key — compose spec v2).
func Render(o Options) string {
	if o.PostgresPassword == "" {
		o.PostgresPassword = "dorkpipe"
	}
	if o.PostgresDB == "" {
		o.PostgresDB = "dorkpipe"
	}
	if o.PostgresUser == "" {
		o.PostgresUser = "dorkpipe"
	}
	if o.PostgresPort == "" {
		o.PostgresPort = "15432"
	}
	if o.OllamaPort == "" {
		o.OllamaPort = "11434"
	}
	out := fmt.Sprintf(`services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: %s
      POSTGRES_PASSWORD: %s
      POSTGRES_DB: %s
    ports:
      - "%s:5432"
    volumes:
      - dorkpipe_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U %s -d %s"]
      interval: 5s
      timeout: 5s
      retries: 5
`, o.PostgresUser, o.PostgresPassword, o.PostgresDB, o.PostgresPort, o.PostgresUser, o.PostgresDB)
	if o.IncludeOllama {
		out += fmt.Sprintf(`  ollama:
    image: ollama/ollama:latest
    ports:
      - "%s:11434"
    volumes:
      - dorkpipe_ollama:/root/.ollama
`, o.OllamaPort)
	}
	out += `
volumes:
  dorkpipe_pgdata:
`
	if o.IncludeOllama {
		out += "  dorkpipe_ollama:\n"
	}
	return out
}

// WriteFile writes compose YAML to path.
func WriteFile(path string, o Options) error {
	return os.WriteFile(path, []byte(Render(o)), 0o644)
}
