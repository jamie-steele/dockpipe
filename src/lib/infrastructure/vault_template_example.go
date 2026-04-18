package infrastructure

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"dockpipe"
)

// VaultTemplateExampleEmbeddedPath is the bundled core asset path for the vault env template example
// (see dockpipe init → project root .env.vault.template.example).
const VaultTemplateExampleEmbeddedPath = EmbeddedTemplatesPrefix + "/assets/env.vault.template.example"

// WriteVaultTemplateExampleIfMissing writes projectDir/.env.vault.template.example when absent.
// Prefers templates/core/assets/env.vault.template.example after dockpipe init merge; falls back to the embedded copy.
func WriteVaultTemplateExampleIfMissing(projectDir string) error {
	dst := filepath.Join(projectDir, ".env.vault.template.example")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	src := filepath.Join(projectDir, "templates", "core", "assets", "env.vault.template.example")
	if b, err := os.ReadFile(src); err == nil {
		return os.WriteFile(dst, b, 0o644)
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := fs.ReadFile(dockpipe.BundledFS, VaultTemplateExampleEmbeddedPath)
	if err != nil {
		return fmt.Errorf("bundled vault template example: %w", err)
	}
	return os.WriteFile(dst, b, 0o644)
}
