#include "ContextDiscovery.h"
#include "DockpipeChoices.h"
#include "GitHelper.h"

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
    const QStringList wfNames = DockpipeChoices::listWorkflowNamesFromRepo(repoRoot);

    QVector<Context> out;
    if (wfNames.isEmpty()) {
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = baseLabel;
        c.workflow = QStringLiteral("vscode");
        out.append(c);
        return out;
    }

    for (const QString &wf : wfNames) {
        Context c = Context::createNew();
        c.workdir = clean;
        c.label = baseLabel + QStringLiteral(" — ") + wf;
        c.workflow = wf;
        out.append(c);
    }
    return out;
}
