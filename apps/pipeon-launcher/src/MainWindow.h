#pragma once

#include "Context.h"
#include "ContextStore.h"
#include "SessionManager.h"

#include <QMainWindow>
#include <QPointer>
#include <QSystemTrayIcon>

class QCloseEvent;
class QListWidget;
class QListWidgetItem;
class QLabel;
class QFrame;

class MainWindow : public QMainWindow {
    Q_OBJECT
public:
    explicit MainWindow(QWidget *parent = nullptr);

protected:
    void closeEvent(QCloseEvent *event) override;

private slots:
    void onAddFolder();
    void onRefreshWorktrees();
    void onRemoveContext();
    void onEditContext();
    void onLaunch();
    void onRelaunch();
    void onStop();
    void onStopAllForRepo();
    void onOpenLogs();
    void onOpenFolder();
    void onTrayActivate(QSystemTrayIcon::ActivationReason reason);
    void onSessionChanged();
    void rebuildContextList();

private:
    void clearContextList();
    bool hasContext(const Context &c) const;
    Context *currentContext();
    QListWidgetItem *currentItem();
    void setupTray();
    void applyContextMenu(QListWidgetItem *item, const QPoint &globalPos);

    ContextStore m_store;
    SessionManager m_sessions;

    QListWidget *m_list = nullptr;
    QLabel *m_hint = nullptr;
    QFrame *m_emptyState = nullptr;
    QSystemTrayIcon *m_tray = nullptr;
};
