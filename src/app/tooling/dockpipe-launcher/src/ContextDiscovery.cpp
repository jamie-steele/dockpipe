#include "ContextDiscovery.h"
#include "DockpipeChoices.h"
#include "GitHelper.h"
#include "WorkflowCatalog.h"

#include <QDir>
#include <QFileInfo>

QVector<Context> ContextDiscovery::contextsForWorkdir(const QString &workdir)
{
    const QString clean = QDir::cleanPath(workdir);

    QString baseLabel = QFileInfo(clean).fileName();
    const QString gitRoot = GitHelper::repoRoot(clean);
    if (!gitRoot.isEmpty())
        baseLabel = QFileInfo(gitRoot).fileName() + QStringLiteral(" — ") + QFileInfo(clean).fileName();

    const QString repoRoot = DockpipeChoices::findRepoRoot(clean);
    const QVector<WorkflowMeta> workflows = WorkflowCatalog::discoverAll(repoRoot, clean);

    QVector<Context> out;
    if (workflows.isEmpty()) {
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = baseLabel;
        c.workflow = QStringLiteral("vscode");
        out.append(c);
        return out;
    }

    for (const WorkflowMeta &wf : workflows) {
        if (wf.workflowId.trimmed().isEmpty())
            continue;
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = baseLabel + QStringLiteral(" — ") + wf.workflowId;
        c.workflow = wf.workflowId;
        out.append(c);
    }
    return out;
}
