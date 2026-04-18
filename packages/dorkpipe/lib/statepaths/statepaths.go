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
	return filepath.Join(workdir, "bin", ".dockpipe", "analysis")
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

func CIRawDir(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "ci-raw")
}

func CIAnalysisDir(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "ci-analysis")
}

func CIFindingsPath(workdir string) string {
	return filepath.Join(CIAnalysisDir(workdir), "findings.json")
}

func CISummaryPath(workdir string) string {
	return filepath.Join(CIAnalysisDir(workdir), "SUMMARY.md")
}

func SelfAnalysisDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "self-analysis"), nil
}

func CursorPromptPath(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "orchestrator-cursor-prompt.md")
}

func CursorRefinedPromptPath(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "orchestrator-cursor-prompt.refined.md")
}

func PastePromptPath(workdir string) string {
	return filepath.Join(workdir, "bin", ".dockpipe", "paste-this-prompt.txt")
}
