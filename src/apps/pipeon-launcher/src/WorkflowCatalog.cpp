#include "WorkflowCatalog.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QProcess>
#include <QProcessEnvironment>
#include <QTextStream>
#include <algorithm>

namespace {

void appendNestedWorkflowConfigPaths(const QDir &root, QStringList &out)
{
    if (!root.exists())
        return;
    for (const QFileInfo &fi : root.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name)) {
        const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
        if (QFileInfo::exists(cfg)) {
            out.append(QDir::cleanPath(cfg));
        } else {
            appendNestedWorkflowConfigPaths(QDir(fi.filePath()), out);
        }
    }
}

QStringList extraWorkflowRootDirsCatalog(const QString &repoRoot)
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

// Lean category root: dockpipe source src/core, else downstream templates/core.
QString coreCategoriesRoot(const QString &repoRoot)
{
    const QDir r(repoRoot);
    const QString srcCore = r.filePath(QStringLiteral("src/core"));
    if (QFileInfo(srcCore + QStringLiteral("/runtimes")).isDir())
        return srcCore;
    return r.filePath(QStringLiteral("templates/core"));
}

QString packageStoreRoot(const QString &hintWorkdir)
{
    if (hintWorkdir.isEmpty())
        return {};
    const QString env = QProcessEnvironment::systemEnvironment().value(QStringLiteral("DOCKPIPE_PACKAGES_ROOT")).trimmed();
    if (!env.isEmpty()) {
        if (QDir::isAbsolutePath(env))
            return QDir::cleanPath(env);
        return QDir::cleanPath(QDir(hintWorkdir).filePath(env));
    }
    return QDir::cleanPath(QDir(hintWorkdir).filePath(QStringLiteral("bin/.dockpipe/internal/packages")));
}

QString globalPackagesRoot()
{
    const QProcessEnvironment env = QProcessEnvironment::systemEnvironment();
    const QString overrideRoot = env.value(QStringLiteral("DOCKPIPE_GLOBAL_ROOT")).trimmed();
    if (!overrideRoot.isEmpty())
        return QDir::cleanPath(overrideRoot + QStringLiteral("/packages"));
#ifdef Q_OS_WIN
    QString base = env.value(QStringLiteral("LOCALAPPDATA")).trimmed();
    if (base.isEmpty())
        base = QDir::homePath() + QStringLiteral("/AppData/Local");
    return QDir::cleanPath(QDir(base).filePath(QStringLiteral("dockpipe/packages")));
#elif defined(Q_OS_MACOS)
    return QDir::cleanPath(QDir::home().filePath(QStringLiteral("Library/Application Support/dockpipe/packages")));
#else
    QString base = env.value(QStringLiteral("XDG_DATA_HOME")).trimmed();
    if (base.isEmpty())
        base = QDir::home().filePath(QStringLiteral(".local/share"));
    return QDir::cleanPath(QDir(base).filePath(QStringLiteral("dockpipe/packages")));
#endif
}

void collectConfigPaths(const QString &repoRoot, QStringList &out)
{
    if (repoRoot.isEmpty())
        return;

    const QDir root(repoRoot);
    const QString cc = coreCategoriesRoot(repoRoot);

    {
        const QDir wf(root.filePath(QStringLiteral("workflows")));
        if (wf.exists()) {
            for (const QFileInfo &fi : wf.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name)) {
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    out.append(QDir::cleanPath(cfg));
            }
        }
    }

    {
        const QDir pkg(root.filePath(QStringLiteral("packages")));
        if (pkg.exists())
            appendNestedWorkflowConfigPaths(pkg, out);
    }
    for (const QString &extra : extraWorkflowRootDirsCatalog(repoRoot)) {
        appendNestedWorkflowConfigPaths(QDir(extra), out);
    }

    {
        const QString bundledWf = root.filePath(QStringLiteral("src/core/workflows"));
        if (QFileInfo(bundledWf).isDir())
            appendNestedWorkflowConfigPaths(QDir(bundledWf), out);
    }

    {
        const QString tpl = root.filePath(QStringLiteral("templates"));
        const QDir tpld(tpl);
        if (tpld.exists()) {
            for (const QFileInfo &fi : tpld.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name)) {
                if (fi.fileName() == QStringLiteral("core"))
                    continue;
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    out.append(QDir::cleanPath(cfg));
            }
        }
    }

    {
        const QDir res(cc + QStringLiteral("/resolvers"));
        if (res.exists()) {
            for (const QFileInfo &fi : res.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name)) {
                const QString cfg = fi.filePath() + QStringLiteral("/config.yml");
                if (QFileInfo::exists(cfg))
                    out.append(QDir::cleanPath(cfg));
            }
        }
    }

    std::sort(out.begin(), out.end());
    out.erase(std::unique(out.begin(), out.end()), out.end());
}

QString stripInlineComment(QString line)
{
    const int hash = line.indexOf(QLatin1Char('#'));
    if (hash >= 0)
        line = line.left(hash);
    return line;
}

QString unquote(QString s)
{
    if ((s.startsWith(QLatin1Char('"')) && s.endsWith(QLatin1Char('"')))
        || (s.startsWith(QLatin1Char('\'')) && s.endsWith(QLatin1Char('\''))))
        s = s.mid(1, s.size() - 2);
    return s;
}

bool parseWorkflowText(const QString &configText, const QString &configPath, const QString &workflowId, WorkflowMeta *out)
{
    out->configPath = configPath;
    out->workflowId = workflowId.trimmed();
    if (out->workflowId.isEmpty())
        out->workflowId = QFileInfo(configPath).absoluteDir().dirName();
    out->displayName = out->workflowId;
    out->description.clear();
    out->category.clear();

    const QStringList lines = configText.split(QLatin1Char('\n'));
    for (int i = 0; i < lines.size(); ++i) {
        QString line = stripInlineComment(lines[i]);
        const QString trimmed = line.trimmed();
        if (trimmed.isEmpty())
            continue;

        auto scalarAfterKey = [&](const QString &key) -> QString {
            if (!trimmed.startsWith(key + QLatin1Char(':')))
                return QString();
            return line.mid(line.indexOf(QLatin1Char(':')) + 1).trimmed();
        };

        if (const QString v = scalarAfterKey(QStringLiteral("category")); !v.isEmpty()) {
            out->category = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(QStringLiteral("name")); !v.isEmpty()) {
            out->displayName = unquote(v);
            continue;
        }
        if (trimmed.startsWith(QStringLiteral("description:"))) {
            QString rest = line.mid(line.indexOf(QLatin1Char(':')) + 1).trimmed();
            if (rest == QStringLiteral(">-") || rest == QStringLiteral(">") || rest == QStringLiteral("|")
                || rest.isEmpty()) {
                QStringList acc;
                int j = i + 1;
                for (; j < lines.size(); ++j) {
                    const QString L = lines[j];
                    if (L.trimmed().isEmpty() && !acc.isEmpty())
                        break;
                    if (!L.startsWith(QLatin1Char(' ')) && !L.startsWith(QLatin1Char('\t'))) {
                        if (L.trimmed().isEmpty())
                            continue;
                        const QString t2 = stripInlineComment(L).trimmed();
                        if (!t2.isEmpty() && !t2.startsWith(QLatin1Char('#'))
                            && t2.contains(QLatin1Char(':')) && !t2.startsWith(QLatin1Char('-')))
                            break;
                    }
                    acc.append(L.trimmed());
                }
                out->description = acc.join(QLatin1Char(' '));
                i = j - 1;
            } else {
                out->description = unquote(rest);
            }
            continue;
        }
    }

    return true;
}

bool parseWorkflowFile(const QString &configPath, WorkflowMeta *out)
{
    QFile f(configPath);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;
    return parseWorkflowText(QString::fromUtf8(f.readAll()), configPath, QFileInfo(configPath).absoluteDir().dirName(), out);
}

QStringList listTarballMembers(const QString &tarPath)
{
    QProcess proc;
    proc.start(QStringLiteral("tar"), {QStringLiteral("-tzf"), tarPath});
    if (!proc.waitForFinished(5000) || proc.exitCode() != 0)
        return {};
    return QString::fromUtf8(proc.readAllStandardOutput()).split(QLatin1Char('\n'), Qt::SkipEmptyParts);
}

QString workflowConfigMemberPath(const QStringList &members, QString *workflowNameOut = nullptr)
{
    for (QString member : members) {
        member = QDir::cleanPath(member);
        const QString prefix = QStringLiteral("workflows/");
        if (!member.startsWith(prefix) || !member.endsWith(QStringLiteral("/config.yml")))
            continue;
        const QString rel = member.mid(prefix.size());
        const int slash = rel.indexOf(QLatin1Char('/'));
        if (slash <= 0)
            continue;
        if (workflowNameOut)
            *workflowNameOut = rel.left(slash);
        return member;
    }
    return {};
}

QString readTarballMemberText(const QString &tarPath, const QString &memberPath)
{
    QProcess proc;
    proc.start(QStringLiteral("tar"), {QStringLiteral("-xOzf"), tarPath, memberPath});
    if (!proc.waitForFinished(5000) || proc.exitCode() != 0)
        return {};
    return QString::fromUtf8(proc.readAllStandardOutput());
}

void appendTarballWorkflowMeta(const QDir &root, QVector<WorkflowMeta> &out, QStringList &seenIds)
{
    if (!root.exists())
        return;
    const auto tars =
        root.entryInfoList(QStringList{QStringLiteral("dockpipe-workflow-*.tar.gz")}, QDir::Files, QDir::Name);
    for (const QFileInfo &fi : tars) {
        const QStringList members = listTarballMembers(fi.filePath());
        QString workflowId;
        const QString memberPath = workflowConfigMemberPath(members, &workflowId);
        if (memberPath.isEmpty() || seenIds.contains(workflowId))
            continue;
        WorkflowMeta m;
        const QString text = readTarballMemberText(fi.filePath(), memberPath);
        if (text.isEmpty())
            continue;
        if (parseWorkflowText(text, QStringLiteral("tar://") + fi.filePath() + QStringLiteral("##") + memberPath,
                              workflowId, &m)) {
            out.append(m);
            seenIds.append(workflowId);
        }
    }
}

} // namespace

QVector<WorkflowMeta> WorkflowCatalog::discoverAll(const QString &repoRoot, const QString &hintWorkdir)
{
    QStringList paths;
    collectConfigPaths(repoRoot, paths);
    QVector<WorkflowMeta> out;
    QStringList seenIds;
    for (const QString &p : paths) {
        WorkflowMeta m;
        if (parseWorkflowFile(p, &m) && !seenIds.contains(m.workflowId)) {
            out.append(m);
            seenIds.append(m.workflowId);
        }
    }

    if (!hintWorkdir.isEmpty())
        appendTarballWorkflowMeta(QDir(packageStoreRoot(hintWorkdir) + QStringLiteral("/workflows")), out, seenIds);
    appendTarballWorkflowMeta(QDir(globalPackagesRoot() + QStringLiteral("/workflows")), out, seenIds);

    std::sort(out.begin(), out.end(), [](const WorkflowMeta &a, const WorkflowMeta &b) {
        return a.workflowId.localeAwareCompare(b.workflowId) < 0;
    });
    return out;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir)
{
    const QVector<WorkflowMeta> all = discoverAll(repoRoot, hintWorkdir);
    QVector<WorkflowMeta> out;
    QStringList seenDisplayNames;
    for (const WorkflowMeta &m : all) {
        if (m.category.compare(QStringLiteral("app"), Qt::CaseInsensitive) != 0)
            continue;
        const QString displayKey = m.displayName.trimmed().toLower();
        if (!displayKey.isEmpty() && seenDisplayNames.contains(displayKey))
            continue;
        out.append(m);
        if (!displayKey.isEmpty())
            seenDisplayNames.append(displayKey);
    }
    return out;
}
