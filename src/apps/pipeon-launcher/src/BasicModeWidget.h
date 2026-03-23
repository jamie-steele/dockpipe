#pragma once

#include "WorkflowCatalog.h"

#include <QHash>
#include <QWidget>

class QLabel;
class QListWidget;
class QPushButton;

/// App-launcher style UI: project folder + grid or list of `category: app` workflows.
class BasicModeWidget : public QWidget {
    Q_OBJECT
public:
    explicit BasicModeWidget(QWidget *parent = nullptr);

    void setProjectFolder(const QString &absPath);
    void setApps(const QVector<WorkflowMeta> &apps);
    void setRunningByWorkflow(const QHash<QString, bool> &running);
    void setViewIconMode(bool icons);

signals:
    void openProjectRequested();
    void refreshAppsRequested();
    void launchRequested(const QString &workflowId);
    void viewModeChanged(bool iconMode);

private slots:
    void onBrowse();
    void onRefresh();

private:
    void applyViewMode();
    void rebuildItemTexts();

    QLabel *m_projectLabel = nullptr;
    QPushButton *m_browse = nullptr;
    QPushButton *m_refresh = nullptr;
    QListWidget *m_list = nullptr;
    QVector<WorkflowMeta> m_apps;
    QHash<QString, bool> m_running;
    bool m_iconMode = true;
};
