#pragma once

#include <QString>
#include <QStringList>

/// Discovers dockpipe workflow / resolver / strategy / runtime names from a repo checkout
/// (same layout as dockpipe's workflow_dirs). Falls back to static lists when no repo is found.
class DockpipeChoices {
public:
    /// Walk upward from workdir (or DOCKPIPE_REPO_ROOT) to find a dockpipe repo root.
    static QString findRepoRoot(const QString &hintWorkdir);

    /// Path to `cursor-dev` resolver's `cursor-prep.sh` when `hintWorkdir` is inside a dockpipe checkout; empty if not found.
    static QString cursorPrepScriptPath(const QString &hintWorkdir);

    /// Workflow names from `workflows/<name>/config.yml`, nested `packages/**/<name>/`, `src/core/workflows/**/<name>/`, optional `DOCKPIPE_EXTRA_WORKFLOW_ROOTS`, or legacy `templates/<name>/` (excluding `core`).
    /// No static fallbacks — empty if repoRoot is invalid or nothing is found.
    static QStringList listWorkflowNamesFromRepo(const QString &repoRoot);

    void scan(const QString &repoRoot);

    QStringList workflowNames;
    QStringList workflowConfigPaths;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};
