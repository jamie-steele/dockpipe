#include "WorkflowCatalog.h"

#include "DockpipeChoices.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QProcess>
#include <algorithm>

namespace {

QStringList sortedUnique(QStringList values)
{
    values.removeDuplicates();
    std::sort(values.begin(), values.end());
    return values;
}

} // namespace

WorkflowCatalogData WorkflowCatalog::discoverCatalog(const QString &hintWorkdir)
{
    WorkflowCatalogData out;
    QString program = DockpipeChoices::preferredDockpipeBinary(hintWorkdir);
    if (program.trimmed().isEmpty())
        program = QStringLiteral("dockpipe");

    QStringList args{
        QStringLiteral("catalog"),
        QStringLiteral("list"),
        QStringLiteral("--format"),
        QStringLiteral("json"),
    };
    if (!hintWorkdir.trimmed().isEmpty())
        args << QStringLiteral("--workdir") << hintWorkdir;

    QProcess proc;
    proc.start(program, args);
    if (!proc.waitForFinished(8000) || proc.exitCode() != 0)
        return out;

    const QJsonDocument doc = QJsonDocument::fromJson(proc.readAllStandardOutput());
    if (!doc.isObject())
        return out;

    const QJsonObject root = doc.object();
    const QJsonArray workflows = root.value(QStringLiteral("workflows")).toArray();
    for (const QJsonValue &value : workflows) {
        if (!value.isObject())
            continue;
        const QJsonObject o = value.toObject();
        WorkflowMeta meta;
        meta.workflowId = o.value(QStringLiteral("workflow_id")).toString().trimmed();
        meta.displayName = o.value(QStringLiteral("display_name")).toString().trimmed();
        meta.description = o.value(QStringLiteral("description")).toString().trimmed();
        meta.category = o.value(QStringLiteral("category")).toString().trimmed();
        meta.configPath = o.value(QStringLiteral("config_path")).toString().trimmed();
        if (meta.displayName.isEmpty())
            meta.displayName = meta.workflowId;
        if (!meta.workflowId.isEmpty())
            out.workflows.append(meta);
    }

    for (const QJsonValue &value : root.value(QStringLiteral("resolvers")).toArray())
        out.resolvers.append(value.toString().trimmed());
    for (const QJsonValue &value : root.value(QStringLiteral("strategies")).toArray())
        out.strategies.append(value.toString().trimmed());
    for (const QJsonValue &value : root.value(QStringLiteral("runtimes")).toArray())
        out.runtimes.append(value.toString().trimmed());

    std::sort(out.workflows.begin(), out.workflows.end(), [](const WorkflowMeta &a, const WorkflowMeta &b) {
        return a.workflowId.localeAwareCompare(b.workflowId) < 0;
    });
    out.resolvers = sortedUnique(out.resolvers);
    out.strategies = sortedUnique(out.strategies);
    out.runtimes = sortedUnique(out.runtimes);
    return out;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAll(const QString &repoRoot, const QString &hintWorkdir)
{
    Q_UNUSED(repoRoot);
    return discoverCatalog(hintWorkdir).workflows;
}

QVector<WorkflowMeta> WorkflowCatalog::discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir)
{
    Q_UNUSED(repoRoot);
    const QVector<WorkflowMeta> all = discoverCatalog(hintWorkdir).workflows;
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
