#pragma once

#include <QString>

/// Pipeon UI preferences (Basic vs Advanced, project folder, view mode). Stored in `launcher.json` under the app config dir.
class LauncherSettings {
public:
    QString uiMode;     ///< `"basic"` (default) or `"advanced"`
    QString basicView;  ///< `"icons"` or `"list"`
    QString projectFolder;
    /// When true, the status-bar third-party disclaimer is hidden until restored from Help.
    bool thirdPartyDisclaimerDismissed = false;

    bool load();
    bool save() const;

    bool isAdvanced() const { return uiMode == QStringLiteral("advanced"); }
    bool isBasicIcons() const { return basicView != QStringLiteral("list"); }

    static QString filePath();
};
