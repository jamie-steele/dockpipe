#include "Context.h"

#include <QJsonArray>
#include <QJsonObject>

Context Context::createNew()
{
    Context c;
    c.id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    return c;
}

Context Context::fromJson(const QJsonObject &o)
{
    Context c;
    c.id = o.value(QStringLiteral("id")).toString();
    c.label = o.value(QStringLiteral("label")).toString();
    c.workdir = o.value(QStringLiteral("workdir")).toString();
    c.workflow = o.value(QStringLiteral("workflow")).toString();
    c.workflowFile = o.value(QStringLiteral("workflowFile")).toString();
    c.resolver = o.value(QStringLiteral("resolver")).toString();
    c.strategy = o.value(QStringLiteral("strategy")).toString();
    c.runtime = o.value(QStringLiteral("runtime")).toString();
    c.dockpipeBinary = o.value(QStringLiteral("dockpipeBinary")).toString();
    c.envFile = o.value(QStringLiteral("envFile")).toString();
    if (const QJsonArray extra = o.value(QStringLiteral("extraDockpipeEnv")).toArray(); !extra.isEmpty()) {
        for (const QJsonValue &v : extra)
            c.extraDockpipeEnv.append(v.toString());
    }
    if (c.id.isEmpty())
        c.id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    return c;
}

QJsonObject Context::toJson() const
{
    QJsonObject o;
    o.insert(QStringLiteral("id"), id);
    o.insert(QStringLiteral("label"), label);
    o.insert(QStringLiteral("workdir"), workdir);
    o.insert(QStringLiteral("workflow"), workflow);
    o.insert(QStringLiteral("workflowFile"), workflowFile);
    o.insert(QStringLiteral("resolver"), resolver);
    o.insert(QStringLiteral("strategy"), strategy);
    o.insert(QStringLiteral("runtime"), runtime);
    o.insert(QStringLiteral("dockpipeBinary"), dockpipeBinary);
    o.insert(QStringLiteral("envFile"), envFile);
    if (!extraDockpipeEnv.isEmpty()) {
        QJsonArray arr;
        for (const QString &s : extraDockpipeEnv)
            arr.append(s);
        o.insert(QStringLiteral("extraDockpipeEnv"), arr);
    }
    return o;
}
