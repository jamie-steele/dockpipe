#pragma once

#include "Context.h"

#include <QVector>

/// Builds launcher contexts from a workdir + resolved dockpipe repo layout (same rules as dockpipe workflow lookup).
class ContextDiscovery {
public:
    /// If a dockpipe repo is found: one context per workflow discovered by WorkflowCatalog, including
    /// compiled workflow tarballs in the package store for the chosen workdir. Otherwise a single
    /// fallback context with workflow `vscode`.
    static QVector<Context> contextsForWorkdir(const QString &workdir);
};
