#pragma once

#include "Context.h"

#include <QVector>

/// Builds launcher contexts from a workdir + resolved dockpipe repo layout (same rules as dockpipe workflow lookup).
class ContextDiscovery {
public:
    /// If a dockpipe repo is found: one context per workflow under `shipyard/workflows/` and `src/templates/` or `templates/` (see DockpipeChoices).
    /// Otherwise a single context with workflow `vscode`.
    static QVector<Context> contextsForWorkdir(const QString &workdir);
};
