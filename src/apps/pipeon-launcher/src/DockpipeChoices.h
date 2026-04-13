#pragma once

#include <QString>
#include <QStringList>

/// Launcher-facing DockPipe choice lists sourced from the DockPipe CLI contract.
/// Falls back to static lists when no usable DockPipe catalog is available.
class DockpipeChoices {
public:
    /// Walk upward from workdir (or DOCKPIPE_REPO_ROOT) to find a dockpipe repo root.
    static QString findRepoRoot(const QString &hintWorkdir);

    /// Prefer the repo checkout binary when available; otherwise fall back to plain `dockpipe`.
    static QString preferredDockpipeBinary(const QString &hintWorkdir);

    void scan(const QString &repoRoot, const QString &hintWorkdir = QString());

    QStringList workflowNames;
    QStringList workflowConfigPaths;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};
