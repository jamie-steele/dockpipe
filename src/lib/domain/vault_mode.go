package domain

import (
	"fmt"
	"strings"
)

// EffectiveVaultString returns the vault mode for op inject. Workflow YAML wins when `vault:` is set;
// otherwise secrets.vault from dockpipe.config.json applies when present.
func EffectiveVaultString(wf *Workflow, cfg *DockpipeProjectConfig) string {
	if wf != nil {
		v := strings.TrimSpace(wf.Vault)
		if v != "" {
			return v
		}
	}
	if cfg != nil && cfg.Secrets.Vault != nil {
		return strings.TrimSpace(*cfg.Secrets.Vault)
	}
	return ""
}

// ValidateVaultModeString checks a vault backend token (workflow vault: or secrets.vault).
func ValidateVaultModeString(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "op", "1password", "none", "off", "false", "no", "0":
		return nil
	default:
		return fmt.Errorf("vault %q is not supported (see docs/vault.md)", v)
	}
}

// ValidateDockpipeProjectConfig checks optional fields after JSON decode.
func ValidateDockpipeProjectConfig(c *DockpipeProjectConfig) error {
	if c == nil {
		return nil
	}
	if c.Secrets.Vault != nil {
		if err := ValidateVaultModeString(*c.Secrets.Vault); err != nil {
			return fmt.Errorf("secrets.vault: %w", err)
		}
	}
	return nil
}
