#include "WorkflowCatalog.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
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

bool parseWorkflowFile(const QString &configPath, WorkflowMeta *out)
{
    QFile f(configPath);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;

    out->configPath = configPath;
    out->workflowId = QFileInfo(configPath).absoluteDir().dirName();
    out->displayName = out->workflowId;
    out->description.clear();
    out->category.clear();

    QTextStream ts(&f);
    QStringList lines;
    while (!ts.atEnd())
        lines.append(ts.readLine());

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

} // namespace

QVector<WorkflowMeta> WorkflowCatalog::discoverAll(const QString &repoRoot)
{
    QStringList paths;
    collectConfigPaths(repoRoot, paths);
    QVector<WorkflowMeta> out;
    for (const QString &p : paths) {
        WorkflowMeta m;
        if (parseWorkflowFile(p, &m))
            out.append(m);
    }
    return out;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAppWorkflows(const QString &repoRoot)
{
    const QVector<WorkflowMeta> all = discoverAll(repoRoot);
    QVector<WorkflowMeta> out;
    for (const WorkflowMeta &m : all) {
        if (m.category.compare(QStringLiteral("app"), Qt::CaseInsensitive) == 0)
            out.append(m);
    }
    return out;
}
