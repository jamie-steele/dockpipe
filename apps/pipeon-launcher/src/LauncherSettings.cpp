#include "LauncherSettings.h"
#include "ContextStore.h"

#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>

QString LauncherSettings::filePath()
{
    return ContextStore::configDir() + QStringLiteral("/launcher.json");
}

bool LauncherSettings::load()
{
    uiMode = QStringLiteral("basic");
    basicView = QStringLiteral("icons");
    projectFolder.clear();

    QFile f(filePath());
    if (!f.exists() || !f.open(QIODevice::ReadOnly))
        return true;
    const QJsonDocument doc = QJsonDocument::fromJson(f.readAll());
    f.close();
    if (!doc.isObject())
        return true;
    const QJsonObject o = doc.object();
    const QString um = o.value(QStringLiteral("uiMode")).toString();
    if (um == QStringLiteral("basic") || um == QStringLiteral("advanced"))
        uiMode = um;
    const QString bv = o.value(QStringLiteral("basicView")).toString();
    if (bv == QStringLiteral("icons") || bv == QStringLiteral("list"))
        basicView = bv;
    projectFolder = o.value(QStringLiteral("projectFolder")).toString();
    if (o.contains(QStringLiteral("thirdPartyDisclaimerDismissed")))
        thirdPartyDisclaimerDismissed = o.value(QStringLiteral("thirdPartyDisclaimerDismissed")).toBool();
    return true;
}

bool LauncherSettings::save() const
{
    QJsonObject o;
    o.insert(QStringLiteral("version"), 1);
    o.insert(QStringLiteral("uiMode"), uiMode);
    o.insert(QStringLiteral("basicView"), basicView);
    o.insert(QStringLiteral("projectFolder"), projectFolder);
    o.insert(QStringLiteral("thirdPartyDisclaimerDismissed"), thirdPartyDisclaimerDismissed);

    QFile f(filePath());
    if (!f.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;
    f.write(QJsonDocument(o).toJson(QJsonDocument::Indented));
    f.close();
    return true;
}
