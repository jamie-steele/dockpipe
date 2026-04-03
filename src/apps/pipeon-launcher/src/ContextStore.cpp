#include "ContextStore.h"

#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QStandardPaths>

QString ContextStore::configDir()
{
    QString base = QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation);
    if (base.isEmpty())
        base = QDir::homePath() + QStringLiteral("/.config/pipeon");
    QDir().mkpath(base);
    return base;
}

QString ContextStore::contextsPath()
{
    return configDir() + QStringLiteral("/contexts.json");
}

QString ContextStore::statePath()
{
    return configDir() + QStringLiteral("/state.json");
}

QString ContextStore::logsDir()
{
    QString d = configDir() + QStringLiteral("/logs");
    QDir().mkpath(d);
    return d;
}

bool ContextStore::load()
{
    contexts.clear();
    QFile f(contextsPath());
    if (!f.exists())
        return true;
    if (!f.open(QIODevice::ReadOnly))
        return false;
    const QJsonDocument doc = QJsonDocument::fromJson(f.readAll());
    f.close();
    if (!doc.isObject())
        return false;
    const QJsonArray arr = doc.object().value(QStringLiteral("contexts")).toArray();
    for (const QJsonValue &v : arr) {
        if (v.isObject())
            contexts.append(Context::fromJson(v.toObject()));
    }
    return true;
}

bool ContextStore::save() const
{
    QJsonArray arr;
    for (const Context &c : contexts)
        arr.append(c.toJson());
    QJsonObject root;
    root.insert(QStringLiteral("contexts"), arr);
    root.insert(QStringLiteral("version"), 1);

    QFile f(contextsPath());
    if (!f.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;
    f.write(QJsonDocument(root).toJson(QJsonDocument::Indented));
    f.close();
    return true;
}
