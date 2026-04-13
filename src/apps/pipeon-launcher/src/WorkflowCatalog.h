#pragma once

#include <QString>
#include <QStringList>
#include <QVector>

/// One workflow entry returned by DockPipe's launcher/tooling catalog contract.
struct WorkflowMeta {
    /// Folder / workflow name passed to `dockpipe --workflow`.
    QString workflowId;
    QString displayName;
    QString description;
    QString category;
    QString configPath;
};

struct WorkflowCatalogData {
    QVector<WorkflowMeta> workflows;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};

class WorkflowCatalog {
public:
    /// Launcher-facing catalog provided by `dockpipe catalog list --format json`.
    static WorkflowCatalogData discoverCatalog(const QString &hintWorkdir = QString());

    /// All workflows with metadata (any category).
    static QVector<WorkflowMeta> discoverAll(const QString &repoRoot, const QString &hintWorkdir = QString());

    /// Only workflows with `category: app` (case-insensitive).
    static QVector<WorkflowMeta> discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir = QString());
};
