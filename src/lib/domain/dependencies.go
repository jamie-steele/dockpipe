package domain

import (
	"fmt"
	"strings"
)

type DependencySpec struct {
	Host []HostDependency `yaml:"host,omitempty"`
}

type HostDependency struct {
	ID          string                    `yaml:"id,omitempty"`
	Command     string                    `yaml:"command,omitempty"`
	Description string                    `yaml:"description,omitempty"`
	Required    *bool                     `yaml:"required,omitempty"`
	Install     HostDependencyInstallHint `yaml:"install,omitempty"`
}

type HostDependencyInstallHint struct {
	Windows string `yaml:"windows,omitempty"`
	MacOS   string `yaml:"macos,omitempty"`
	Linux   string `yaml:"linux,omitempty"`
	Deb     string `yaml:"deb,omitempty"`
}

func ValidateDependencySpec(fieldPrefix string, deps DependencySpec) error {
	seen := map[string]struct{}{}
	for i, dep := range deps.Host {
		prefix := fmt.Sprintf("%s.host[%d]", fieldPrefix, i)
		id := strings.TrimSpace(dep.ID)
		command := strings.TrimSpace(dep.Command)
		if id == "" && command == "" {
			return fmt.Errorf("%s requires id or command", prefix)
		}
		if id != "" && strings.ContainsAny(id, " \t\r\n") {
			return fmt.Errorf("%s.id must not contain whitespace", prefix)
		}
		if command != "" && strings.ContainsAny(command, `/\`) {
			return fmt.Errorf("%s.command must be an executable name, not a path", prefix)
		}
		key := firstNonEmptyString(command, id)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%s duplicates dependency %q", prefix, key)
		}
		seen[key] = struct{}{}
		if err := validateDependencyInstallHint(prefix+".install", dep.Install); err != nil {
			return err
		}
	}
	return nil
}

func validateDependencyInstallHint(fieldPrefix string, hint HostDependencyInstallHint) error {
	for k, v := range map[string]string{
		"windows": hint.Windows,
		"macos":   hint.MacOS,
		"linux":   hint.Linux,
		"deb":     hint.Deb,
	} {
		if strings.ContainsAny(v, "\x00\r\n") {
			return fmt.Errorf("%s.%s must be a single shell command or short instruction", fieldPrefix, k)
		}
	}
	return nil
}

func ValidatePlatformList(fieldPrefix string, platforms []string) error {
	seen := map[string]struct{}{}
	for i, platform := range platforms {
		platform = strings.TrimSpace(strings.ToLower(platform))
		switch platform {
		case "windows", "macos", "linux", "deb":
		default:
			return fmt.Errorf("%s[%d] must be one of windows, macos, linux, deb", fieldPrefix, i)
		}
		if _, ok := seen[platform]; ok {
			return fmt.Errorf("%s[%d] duplicates platform %q", fieldPrefix, i, platform)
		}
		seen[platform] = struct{}{}
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
