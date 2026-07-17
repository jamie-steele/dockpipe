#pragma once

#include "LauncherSettings.h"

#include <QDialog>

class QLineEdit;
class QPlainTextEdit;

class SettingsDialog : public QDialog {
    Q_OBJECT
public:
    explicit SettingsDialog(const LauncherSettings &settings, QWidget *parent = nullptr);

    LauncherSettings updatedSettings() const;

private slots:
    void browseRepoRoot();
    void browseGlobalRoot();

private:
    static QStringList splitLines(const QString &text);
    static QString joinLines(const QStringList &values);

    QLineEdit *m_repoRootOverride = nullptr;
    QLineEdit *m_globalRootOverride = nullptr;
    QPlainTextEdit *m_extraWorkflowRoots = nullptr;
    QPlainTextEdit *m_packageRemotes = nullptr;
    LauncherSettings m_settings;
};
