#include "DockpipeChoices.h"

#include <QDir>
#include <QFileInfo>
#include <QProcessEnvironment>
#include <algorithm>

namespace {

bool looksLikeRepoRoot(const QString &absPath)
{
    const QDir d(absPath);
    return QFileInfo(d.filePath(QStringLiteral("dockpipe/workflows"))).isDir()
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
                              QStringLiteral("cursor-dev"), QStringLiteral("code-server")};
    c.strategies = QStringList{QStringLiteral("commit"), QStringLiteral("worktree")};
    c.runtimes = QStringList{QStringLiteral("docker"), QStringLiteral("cli"), QStringLiteral("kube-pod")};
}

} // namespace

QString DockpipeChoices::findRepoRoot(const QString &hintWorkdir)
{
    const QString env = QProcessEnvironment::systemEnvironment().value(QStringLiteral("DOCKPIPE_REPO_ROOT"));
    if (!env.isEmpty()) {
        const QString clean = QDir::cleanPath(env);
        if (looksLikeRepoRoot(clean))
            return clean;
    }

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
    return QString();
}

QStringList DockpipeChoices::listWorkflowNamesFromRepo(const QString &repoRoot)
{
    QStringList names;
    if (repoRoot.isEmpty())
        return names;

    const QDir root(repoRoot);

    {
        const QDir wf(root.filePath(QStringLiteral("dockpipe/workflows")));
        if (wf.exists()) {
            const auto dirs = wf.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    names.append(fi.fileName());
            }
        }
    }

    {
        const QDir tpl(root.filePath(QStringLiteral("templates")));
        if (tpl.exists()) {
            const auto dirs = tpl.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                if (fi.fileName() == QStringLiteral("core"))
                    continue;
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    names.append(fi.fileName());
            }
        }
    }

    sortUnique(names);
    return names;
}

void DockpipeChoices::scan(const QString &repoRoot)
{
    workflowNames.clear();
    workflowConfigPaths.clear();
    resolvers.clear();
    strategies.clear();
    runtimes.clear();

    if (repoRoot.isEmpty()) {
        appendStaticFallbacks(*this);
        sortUnique(workflowNames);
        sortUnique(resolvers);
        sortUnique(strategies);
        sortUnique(runtimes);
        return;
    }

    const QDir root(repoRoot);

    {
        const QDir wf(root.filePath(QStringLiteral("dockpipe/workflows")));
        if (wf.exists()) {
            const auto dirs = wf.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg)) {
                    workflowNames.append(fi.fileName());
                    workflowConfigPaths.append(QDir::cleanPath(cfg));
                }
            }
        }
    }

    {
        const QDir tpl(root.filePath(QStringLiteral("templates")));
        if (tpl.exists()) {
            const auto dirs = tpl.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                if (fi.fileName() == QStringLiteral("core"))
                    continue;
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg)) {
                    workflowNames.append(fi.fileName());
                    workflowConfigPaths.append(QDir::cleanPath(cfg));
                }
            }
        }
    }

    {
        const QDir res(root.filePath(QStringLiteral("templates/core/resolvers")));
        if (res.exists()) {
            const auto dirs = res.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    resolvers.append(fi.fileName());
            }
        }
    }

    {
        const QDir strat(root.filePath(QStringLiteral("templates/core/strategies")));
        if (strat.exists()) {
            const auto files = strat.entryInfoList(QDir::Files | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : files) {
                if (fi.fileName() == QStringLiteral("README.md"))
                    continue;
                strategies.append(fi.fileName());
            }
        }
    }

    {
        const QDir rt(root.filePath(QStringLiteral("templates/core/runtimes")));
        if (rt.exists()) {
            const auto dirs = rt.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
            for (const QFileInfo &fi : dirs) {
                if (fi.fileName().endsWith(QStringLiteral(".md"), Qt::CaseInsensitive))
                    continue;
                runtimes.append(fi.fileName());
            }
        }
    }

    if (workflowNames.isEmpty() && resolvers.isEmpty()) {
        appendStaticFallbacks(*this);
    } else {
        if (workflowNames.isEmpty()) {
            workflowNames = QStringList{QStringLiteral("vscode"), QStringLiteral("init"), QStringLiteral("run"),
                                        QStringLiteral("run-apply"), QStringLiteral("run-apply-validate")};
        }
        if (resolvers.isEmpty())
            resolvers = QStringList{QStringLiteral("vscode"), QStringLiteral("claude"), QStringLiteral("codex"),
                                    QStringLiteral("cursor-dev"), QStringLiteral("code-server")};
        if (strategies.isEmpty())
            strategies = QStringList{QStringLiteral("commit"), QStringLiteral("worktree")};
        if (runtimes.isEmpty())
            runtimes = QStringList{QStringLiteral("docker"), QStringLiteral("cli"), QStringLiteral("kube-pod")};
    }

    sortUnique(workflowNames);
    sortUnique(workflowConfigPaths);
    sortUnique(resolvers);
    sortUnique(strategies);
    sortUnique(runtimes);
}
