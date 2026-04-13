#pragma once

#include <QString>
#include <QStringList>

/// Launcher-facing DockPipe choice lists sourced from the DockPipe CLI contract.
/// Falls back to static lists when no usable DockPipe catalog is available.
class DockpipeChoices {
public:
    /// Walk upward from workdir to find a dockpipe project root.
    static QString findRepoRoot(const QString &hintWorkdir);

    /// Prefer DOCKPIPE_BIN when set; otherwise fall back to plain `dockpipe`.
    static QString preferredDockpipeBinary(const QString &hintWorkdir);

    void scan(const QString &repoRoot, const QString &hintWorkdir = QString());

    QStringList workflowNames;
    QStringList workflowConfigPaths;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};
