#pragma once

#include "Context.h"
#include "ContextStore.h"
#include "LauncherSettings.h"
#include "SessionManager.h"

#include <QMainWindow>
#include <QPointer>
#include <QSystemTrayIcon>

class QCloseEvent;
class QListWidget;
class QListWidgetItem;
class QLabel;
class QFrame;
class QStackedWidget;
class QAction;
class QActionGroup;
class BasicModeWidget;

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
    void rebuildUi();

    void onFileOpenProject();
    void onRefreshAppList();
    void onViewBasic();
    void onViewAdvanced();
    void onViewIconGrid();
    void onViewCompactList();
    void onBasicLaunch(const QString &workflowId);
    void onBasicBackHome();
    void onBasicOpenRecent(const QString &absPath);
    void onBasicContinueLast();
    void onSetupMcp();
    void onDismissThirdPartyDisclaimer();
    void onRestoreThirdPartyDisclaimer();

private:
    void setupMenuBar();
    void setupAdvancedPage(QWidget *page);
    void applyUiMode();
    void clearContextList();
    void rebuildAdvancedContextList();
    void updateBasicPage();
    bool hasContext(const Context &c) const;
    Context *findContext(const QString &workdir, const QString &workflow, const QString &workflowFile);
    Context *currentContext();
    QListWidgetItem *currentItem();
    void setupTray();
    void setupDisclaimerBar();
    void applyContextMenu(QListWidgetItem *item, const QPoint &globalPos);

    ContextStore m_store;
    SessionManager m_sessions;
    LauncherSettings m_settings;

    QStackedWidget *m_stack = nullptr;
    BasicModeWidget *m_basicWidget = nullptr;
    QWidget *m_advancedPage = nullptr;

    QListWidget *m_list = nullptr;
    QLabel *m_hint = nullptr;
    QFrame *m_emptyState = nullptr;
    QSystemTrayIcon *m_tray = nullptr;

    QAction *m_actBasic = nullptr;
    QAction *m_actAdvanced = nullptr;
    QAction *m_actIcons = nullptr;
    QAction *m_actList = nullptr;

    QWidget *m_disclaimerContainer = nullptr;
};
