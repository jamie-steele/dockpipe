#pragma once

#include "WorkflowCatalog.h"

#include <QHash>
#include <QWidget>

class QLabel;
class QListWidget;
class QPushButton;
class QStackedWidget;

/// IDE-style flow: home (recent projects) → workspace (apps for the selected folder).
class BasicModeWidget : public QWidget {
    Q_OBJECT
public:
    explicit BasicModeWidget(QWidget *parent = nullptr);

    void showHomePage();
    void showWorkspacePage();

    void setProjectFolder(const QString &absPath);
    void setRecentProjects(const QStringList &paths);
    void setContinueLastVisible(bool visible);

    void setApps(const QVector<WorkflowMeta> &apps);
    void setRunningByWorkflow(const QHash<QString, bool> &running);
    void setViewIconMode(bool icons);

signals:
    void openProjectRequested();
    void refreshAppsRequested();
    void launchRequested(const QString &workflowId);

    void backToHomeRequested();
    void recentProjectSelected(const QString &absPath);
    void continueLastRequested();
    void setupMcpRequested();

private slots:
    void onBrowse();
    void onRefresh();

private:
    void applyViewMode();
    void rebuildItemTexts();
    void rebuildRecentList();

    QStackedWidget *m_stack = nullptr;
    QWidget *m_homePage = nullptr;
    QWidget *m_workspacePage = nullptr;

    QListWidget *m_recentList = nullptr;
    QLabel *m_homeEmptyHint = nullptr;
    QPushButton *m_openProjectHome = nullptr;
    QPushButton *m_continueLast = nullptr;

    QPushButton *m_backHome = nullptr;
    QLabel *m_projectLabel = nullptr;
    QPushButton *m_browse = nullptr;
    QPushButton *m_refresh = nullptr;
    QPushButton *m_setupMcp = nullptr;
    QListWidget *m_list = nullptr;

    QStringList m_recentPaths;
    QVector<WorkflowMeta> m_apps;
    QHash<QString, bool> m_running;
    bool m_iconMode = true;
};
