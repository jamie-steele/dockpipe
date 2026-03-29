#include "DockpipeChoices.h"

#include <QDir>
#include <QDirIterator>
#include <QFileInfo>
#include <QProcessEnvironment>
#include <algorithm>

namespace {

QString coreCategoriesRoot(const QString &repoRoot)
{
    const QDir r(repoRoot);
    const QString srcCore = r.filePath(QStringLiteral("src/core"));
    if (QFileInfo(srcCore + QStringLiteral("/runtimes")).isDir())
        return srcCore;
    return r.filePath(QStringLiteral("templates/core"));
}

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
                              QStringLiteral("cursor-dev"), QStringLiteral("code-server")};
    c.strategies = QStringList{QStringLiteral("commit"), QStringLiteral("worktree")};
    c.runtimes = QStringList{QStringLiteral("dockerimage"), QStringLiteral("dockerfile"), QStringLiteral("package")};
}

// Recursively collect workflow dirs that contain config.yml (.staging/packages may be nested by namespace).
void collectStagingWorkflowConfigs(const QDir &root, QStringList &names, QStringList *paths)
{
    if (!root.exists())
        return;
    for (const QFileInfo &fi : root.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name)) {
        const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
        if (QFileInfo::exists(cfg)) {
            names.append(fi.fileName());
            if (paths)
                paths->append(QDir::cleanPath(cfg));
        } else {
            collectNestedWorkflowConfigs(QDir(fi.filePath()), names, paths);
        }
    }
}

QStringList extraWorkflowRootDirs(const QString &repoRoot)
{
    QStringList out;
    const QString raw = QProcessEnvironment::systemEnvironment().value(QStringLiteral("DOCKPIPE_EXTRA_WORKFLOW_ROOTS"));
    if (raw.isEmpty())
        return out;
    for (const QString &part : raw.split(QLatin1Char(':'), Qt::SkipEmptyParts)) {
        const QString p = QDir::cleanPath(QDir(repoRoot).filePath(part.trimmed()));
        if (QFileInfo(p).isDir())
            out.append(p);
    }
    return out;
}

QString findCursorPrepUnderPackages(const QString &repoRoot)
{
    const QDir pkg(QDir(repoRoot).filePath(QStringLiteral("packages")));
    if (!pkg.exists())
        return {};
    QDirIterator it(pkg.path(), QStringList{QStringLiteral("cursor-prep.sh")}, QDir::Files,
                    QDirIterator::Subdirectories);
    while (it.hasNext()) {
        it.next();
        return QDir::cleanPath(it.filePath());
    }
    return {};
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

QString DockpipeChoices::cursorPrepScriptPath(const QString &hintWorkdir)
{
    const QString root = findRepoRoot(hintWorkdir);
    if (root.isEmpty())
        return {};
    const QString envPrep =
        QProcessEnvironment::systemEnvironment().value(QStringLiteral("DOCKPIPE_CURSOR_PREP_SCRIPT")).trimmed();
    if (!envPrep.isEmpty() && QFileInfo(envPrep).exists())
        return QDir::cleanPath(envPrep);
    const QString srcCore =
        QDir::cleanPath(root + QStringLiteral("/src/core/resolvers/cursor-dev/assets/scripts/cursor-prep.sh"));
    if (QFileInfo::exists(srcCore))
        return srcCore;
    const QString legacy =
        QDir::cleanPath(root + QStringLiteral("/templates/core/resolvers/cursor-dev/assets/scripts/cursor-prep.sh"));
    if (QFileInfo::exists(legacy))
        return legacy;
    const QString underPkg = findCursorPrepUnderPackages(root);
    if (!underPkg.isEmpty())
        return underPkg;
    return {};
}

QStringList DockpipeChoices::listWorkflowNamesFromRepo(const QString &repoRoot)
{
    QStringList names;
    if (repoRoot.isEmpty())
        return names;

    const QDir root(repoRoot);

    {
        const QDir wf(root.filePath(QStringLiteral("workflows")));
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
        const QDir pkg(root.filePath(QStringLiteral("packages")));
        if (pkg.exists())
            collectNestedWorkflowConfigs(pkg, names, nullptr);
    }
    for (const QString &extra : extraWorkflowRootDirs(repoRoot)) {
        collectNestedWorkflowConfigs(QDir(extra), names, nullptr);
    }

    {
        const QString bundledWf = root.filePath(QStringLiteral("src/core/workflows"));
        if (QFileInfo(bundledWf).isDir())
            collectNestedWorkflowConfigs(QDir(bundledWf), names, nullptr);
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
    const QString cc = coreCategoriesRoot(repoRoot);

    {
        const QDir wf(root.filePath(QStringLiteral("workflows")));
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
        const QDir pkg(root.filePath(QStringLiteral("packages")));
        if (pkg.exists())
            collectNestedWorkflowConfigs(pkg, workflowNames, &workflowConfigPaths);
    }
    for (const QString &extra : extraWorkflowRootDirs(repoRoot)) {
        collectNestedWorkflowConfigs(QDir(extra), workflowNames, &workflowConfigPaths);
    }

    {
        const QString bundledWf = root.filePath(QStringLiteral("src/core/workflows"));
        if (QFileInfo(bundledWf).isDir())
            collectNestedWorkflowConfigs(QDir(bundledWf), workflowNames, &workflowConfigPaths);
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
        const QDir res(cc + QStringLiteral("/resolvers"));
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
        const QDir strat(cc + QStringLiteral("/strategies"));
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
        const QDir rt(cc + QStringLiteral("/runtimes"));
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
            runtimes = QStringList{QStringLiteral("cli"), QStringLiteral("powershell"), QStringLiteral("cmd"), QStringLiteral("dockerimage"), QStringLiteral("dockerfile"), QStringLiteral("package")};
    }

    sortUnique(workflowNames);
    sortUnique(workflowConfigPaths);
    sortUnique(resolvers);
    sortUnique(strategies);
    sortUnique(runtimes);
}
