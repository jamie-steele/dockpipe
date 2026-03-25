#include "LauncherSettings.h"
#include "ContextStore.h"

#include <QDir>
#include <QFile>
#include <QJsonArray>
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
    if (o.contains(QStringLiteral("recentProjectFolders")) && o.value(QStringLiteral("recentProjectFolders")).isArray()) {
        const QJsonArray a = o.value(QStringLiteral("recentProjectFolders")).toArray();
        for (const QJsonValue &v : a) {
            const QString p = v.toString();
            if (!p.isEmpty())
                recentProjectFolders.append(p);
        }
        while (recentProjectFolders.size() > kMaxRecentProjects)
            recentProjectFolders.removeLast();
    }
    if (!projectFolder.isEmpty() && recentProjectFolders.isEmpty()) {
        recentProjectFolders.prepend(QDir::cleanPath(projectFolder));
    }
    return true;
}

void LauncherSettings::addRecentProject(const QString &path)
{
    if (path.isEmpty())
        return;
    const QString c = QDir::cleanPath(path);
    recentProjectFolders.removeAll(c);
    recentProjectFolders.prepend(c);
    while (recentProjectFolders.size() > kMaxRecentProjects)
        recentProjectFolders.removeLast();
}

bool LauncherSettings::save() const
{
    QJsonObject o;
    o.insert(QStringLiteral("version"), 1);
    o.insert(QStringLiteral("uiMode"), uiMode);
    o.insert(QStringLiteral("basicView"), basicView);
    o.insert(QStringLiteral("projectFolder"), projectFolder);
    o.insert(QStringLiteral("thirdPartyDisclaimerDismissed"), thirdPartyDisclaimerDismissed);
    {
        QJsonArray a;
        for (const QString &p : recentProjectFolders)
            a.append(p);
        o.insert(QStringLiteral("recentProjectFolders"), a);
    }

    QFile f(filePath());
    if (!f.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;
    f.write(QJsonDocument(o).toJson(QJsonDocument::Indented));
    f.close();
    return true;
}
