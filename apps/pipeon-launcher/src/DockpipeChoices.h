#pragma once

#include <QString>
#include <QStringList>

/// Discovers dockpipe workflow / resolver / strategy / runtime names from a repo checkout
/// (same layout as dockpipe's workflow_dirs). Falls back to static lists when no repo is found.
class DockpipeChoices {
public:
    /// Walk upward from workdir (or DOCKPIPE_REPO_ROOT) to find a dockpipe repo root.
    static QString findRepoRoot(const QString &hintWorkdir);

    /// Workflow names from `dockpipe/workflows/<name>/config.yml` and `templates/<name>/config.yml` (excluding `core`).
    /// No static fallbacks — empty if repoRoot is invalid or nothing is found.
    static QStringList listWorkflowNamesFromRepo(const QString &repoRoot);

    void scan(const QString &repoRoot);

    QStringList workflowNames;
    QStringList workflowConfigPaths;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};
