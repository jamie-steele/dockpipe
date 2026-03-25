#pragma once

#include <QString>
#include <QVector>

/// One workflow discovered on disk (same resolution as dockpipe) with optional `category:` from YAML.
struct WorkflowMeta {
    /// Folder / workflow name passed to `dockpipe --workflow`.
    QString workflowId;
    QString displayName;
    QString description;
    QString category;
    QString configPath;
};

class WorkflowCatalog {
public:
    /// All workflows with metadata (any category).
    static QVector<WorkflowMeta> discoverAll(const QString &repoRoot);

    /// Only workflows with `category: app` (case-insensitive).
    static QVector<WorkflowMeta> discoverAppWorkflows(const QString &repoRoot);
};
