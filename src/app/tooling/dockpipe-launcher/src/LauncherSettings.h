#pragma once

#include <QString>
#include <QStringList>

/// DockPipe Launcher UI preferences (Basic vs Advanced, project folder, view mode). Stored in `launcher.json` under the app config dir.
class LauncherSettings {
public:
    static constexpr int kMaxRecentProjects = 20;

    QString uiMode;     ///< `"basic"` (default) or `"advanced"`
    QString basicView;  ///< `"icons"` or `"list"`
    QString projectFolder;
    QString repoRootOverride;
    QString globalRootOverride;
    QStringList extraWorkflowRoots;
    QStringList packageRemotes;
    /// Most recently opened project folders (newest first), capped at `kMaxRecentProjects`.
    QStringList recentProjectFolders;
    /// When true, the status-bar third-party disclaimer is hidden until restored from Help.
    bool thirdPartyDisclaimerDismissed = false;

    bool load();
    bool save() const;

    /// Dedupes, moves `path` to the front, and trims to `kMaxRecentProjects`.
    void addRecentProject(const QString &path);

    bool isAdvanced() const { return uiMode == QStringLiteral("advanced"); }
    bool isBasicIcons() const { return basicView != QStringLiteral("list"); }

    static LauncherSettings current();
    static QString filePath();
};
