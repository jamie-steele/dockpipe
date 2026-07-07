package statepaths

import (
	"path/filepath"

	"dockpipe/src/lib/infrastructure"
)

const dorkpipeScope = "dorkpipe"

func PackageStateDir(workdir string) (string, error) {
	return infrastructure.PackageStateDir(workdir, dorkpipeScope)
}

func EditArtifactsDir(workdir, requestID string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "edit", requestID), nil
}

func ReasoningArtifactsDir(workdir, requestID string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "reasoning", requestID), nil
}

func MetricsPath(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "metrics.jsonl"), nil
}

func RunPath(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "run.json"), nil
}

func NodesDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "nodes"), nil
}

func AnalysisDir(workdir string) string {
	return packageStatePath(workdir, "analysis")
}

func QueuePath(workdir string) string {
	return filepath.Join(AnalysisDir(workdir), "queue.json")
}

func InsightsPath(workdir string) string {
	return filepath.Join(AnalysisDir(workdir), "insights.json")
}

func AnalysisHistoryPath(workdir string) string {
	return filepath.Join(AnalysisDir(workdir), "history.jsonl")
}

func InsightsByCategoryDir(workdir string) string {
	return filepath.Join(AnalysisDir(workdir), "by-category")
}

func PackageCIDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "ci"), nil
}

func PackageCIRawDir(workdir string) (string, error) {
	root, err := PackageCIDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "raw"), nil
}

func PackageCIAnalysisDir(workdir string) (string, error) {
	root, err := PackageCIDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "analysis"), nil
}

func PackageCIFindingsPath(workdir string) (string, error) {
	root, err := PackageCIAnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "findings.json"), nil
}

func PackageCISummaryPath(workdir string) (string, error) {
	root, err := PackageCIAnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "SUMMARY.md"), nil
}

func SelfAnalysisDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "self-analysis"), nil
}

func ProviderPoolsDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "provider-pools"), nil
}

func ProviderPoolLeasesDir(workdir string) (string, error) {
	root, err := ProviderPoolsDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "leases"), nil
}

func ProviderPoolSessionsPath(workdir string) (string, error) {
	root, err := ProviderPoolsDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions.json"), nil
}

func packageStatePath(workdir string, parts ...string) string {
	root, err := PackageStateDir(workdir)
	if err != nil {
		root = filepath.Join(workdir, infrastructure.DockpipeDirRel, "packages", dorkpipeScope)
	}
	all := append([]string{root}, parts...)
	return filepath.Join(all...)
}

func CursorPromptPath(workdir string) string {
	return packageStatePath(workdir, "handoff", "orchestrator-cursor-prompt.md")
}

func CursorRefinedPromptPath(workdir string) string {
	return packageStatePath(workdir, "handoff", "orchestrator-cursor-prompt.refined.md")
}

func PastePromptPath(workdir string) string {
	return packageStatePath(workdir, "handoff", "paste-this-prompt.txt")
}
