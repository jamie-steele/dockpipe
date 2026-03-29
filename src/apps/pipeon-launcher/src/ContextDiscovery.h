#pragma once

#include "Context.h"

#include <QVector>

/// Builds launcher contexts from a workdir + resolved dockpipe repo layout (same rules as dockpipe workflow lookup).
class ContextDiscovery {
public:
    /// If a dockpipe repo is found: one context per workflow under `workflows/`, nested `packages/**/`, `src/core/workflows/`, optional extra roots from env, or legacy `templates/` (see DockpipeChoices).
    /// Otherwise a single context with workflow `vscode`.
    static QVector<Context> contextsForWorkdir(const QString &workdir);
};
