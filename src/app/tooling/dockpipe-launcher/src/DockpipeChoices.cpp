#include "DockpipeChoices.h"

#include "LauncherSettings.h"
#include "WorkflowCatalog.h"

#include <QDir>
#include <QFileInfo>
#include <QProcessEnvironment>
#include <algorithm>

namespace {

bool looksLikeRepoRoot(const QString &absPath)
{
    const QDir d(absPath);
    return QFileInfo(d.filePath(QStringLiteral("workflows"))).isDir()
        || QFileInfo(d.filePath(QStringLiteral("dockpipe.config.json"))).isFile()
        || QFileInfo(d.filePath(QStringLiteral("packages"))).isDir()
        || QFileInfo(d.filePath(QStringLiteral("src/core/runtimes"))).isDir()
        || QFileInfo(d.filePath(QStringLiteral("templates/core"))).isDir();
}

void sortUnique(QStringList &list)
{
    list.removeDuplicates();
    std::sort(list.begin(), list.end());
}

void appendStaticFallbacks(DockpipeChoices &c)
{
    c.workflowNames = QStringList{QStringLiteral("vscode"), QStringLiteral("init"), QStringLiteral("run"),
                                  QStringLiteral("run-apply"), QStringLiteral("run-apply-validate")};
    c.resolvers = QStringList{QStringLiteral("vscode"), QStringLiteral("claude"), QStringLiteral("codex"),
                              QStringLiteral("cursor-dev")};
    c.strategies = QStringList{QStringLiteral("commit"), QStringLiteral("worktree")};
    c.runtimes = QStringList{QStringLiteral("dockerimage"), QStringLiteral("dockerfile"), QStringLiteral("package")};
}

} // namespace

QString DockpipeChoices::findRepoRoot(const QString &hintWorkdir)
{
    if (!hintWorkdir.isEmpty()) {
        QDir d(QFileInfo(hintWorkdir).absoluteFilePath());
        if (!d.exists())
            d = QFileInfo(hintWorkdir).absoluteDir();
        for (int i = 0; i < 32; ++i) {
            const QString p = d.absolutePath();
            if (looksLikeRepoRoot(p))
                return p;
            if (!d.cdUp())
                break;
        }
    }
    const LauncherSettings settings = LauncherSettings::current();
    if (!settings.repoRootOverride.trimmed().isEmpty()) {
        const QString clean = QDir::cleanPath(settings.repoRootOverride.trimmed());
        if (looksLikeRepoRoot(clean))
            return clean;
    }
    return QString();
}

QString DockpipeChoices::preferredDockpipeBinary(const QString &hintWorkdir)
{
    Q_UNUSED(hintWorkdir);
    const QString configured = QProcessEnvironment::systemEnvironment().value(QStringLiteral("DOCKPIPE_BIN")).trimmed();
    if (!configured.isEmpty())
        return configured;
    return QStringLiteral("dockpipe");
}

void DockpipeChoices::scan(const QString &repoRoot, const QString &hintWorkdir)
{
    Q_UNUSED(repoRoot);
    workflowNames.clear();
    workflowConfigPaths.clear();
    resolvers.clear();
    strategies.clear();
    runtimes.clear();

    const WorkflowCatalogData catalog = WorkflowCatalog::discoverCatalog(hintWorkdir);
    for (const WorkflowMeta &wf : catalog.workflows) {
        const QString id = wf.workflowId.trimmed();
        if (!id.isEmpty())
            workflowNames.append(id);
        const QString cfg = wf.configPath.trimmed();
        if (!cfg.isEmpty() && !cfg.startsWith(QStringLiteral("tar://")))
            workflowConfigPaths.append(QDir::cleanPath(cfg));
    }
    resolvers = catalog.resolvers;
    strategies = catalog.strategies;
    runtimes = catalog.runtimes;

    if (workflowNames.isEmpty() && resolvers.isEmpty()) {
        appendStaticFallbacks(*this);
    } else {
        if (workflowNames.isEmpty()) {
            workflowNames = QStringList{QStringLiteral("vscode"), QStringLiteral("init"), QStringLiteral("run"),
                                        QStringLiteral("run-apply"), QStringLiteral("run-apply-validate")};
        }
        if (resolvers.isEmpty())
            resolvers = QStringList{QStringLiteral("vscode"), QStringLiteral("claude"), QStringLiteral("codex"),
                                    QStringLiteral("cursor-dev")};
        if (strategies.isEmpty())
            strategies = QStringList{QStringLiteral("commit"), QStringLiteral("worktree")};
        if (runtimes.isEmpty()) {
            runtimes = QStringList{QStringLiteral("cli"), QStringLiteral("powershell"), QStringLiteral("cmd"),
                                   QStringLiteral("dockerimage"), QStringLiteral("dockerfile"),
                                   QStringLiteral("package")};
        }
    }

    sortUnique(workflowNames);
    sortUnique(workflowConfigPaths);
    sortUnique(resolvers);
    sortUnique(strategies);
    sortUnique(runtimes);
}
