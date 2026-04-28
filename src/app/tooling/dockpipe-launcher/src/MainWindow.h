#pragma once

#include "Context.h"
#include "ContextStore.h"
#include "LauncherSettings.h"
#include "SessionManager.h"
#include "WorkflowCatalog.h"

#include <QMainWindow>
#include <QPointer>
#include <QSystemTrayIcon>

class QCloseEvent;
class QListWidget;
class QListWidgetItem;
class QLabel;
class QLineEdit;
class QPlainTextEdit;
class QFrame;
class QStackedWidget;
class QAction;
class QActionGroup;
class BasicModeWidget;
class QTabWidget;
class DockerObservabilityWidget;
class QTimer;

class MainWindow : public QMainWindow {
    Q_OBJECT
public:
    explicit MainWindow(QWidget *parent = nullptr);

protected:
    void closeEvent(QCloseEvent *event) override;

private slots:
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
    void onOpenSettings();
    void onManagePackages();
    void onBasicLaunch(const QString &workflowId);
    void onBasicBackHome();
    void onBasicOpenRecent(const QString &absPath);
    void onBasicContinueLast();
    void activateHome();
    void onAbout();
    void onDismissThirdPartyDisclaimer();
    void onRestoreThirdPartyDisclaimer();
    void onAdvancedSearchChanged(const QString &text);
    void onAdvancedSelectionChanged();
    void onSessionOutput(const QString &contextId, const QString &text);

private:
    void setupMenuBar();
    void setupAdvancedPage(QWidget *page);
    void applyUiMode();
    void clearContextList();
    void rebuildAdvancedContextList();
    void applyAdvancedContextFilter();
    void updateBasicPage();
    bool hasContext(const Context &c) const;
    Context *findContext(const QString &workdir, const QString &workflow, const QString &workflowFile);
    Context *findStoredContextForDisplay(const Context &display);
    Context *ensureStoredContextForDisplay(const Context &display);
    Context *currentAdvancedDisplayContext();
    Context *currentContext();
    QListWidgetItem *currentItem();
    void setupTray();
    void setupDisclaimerBar();
    void applyContextMenu(QListWidgetItem *item, const QPoint &globalPos);
    void refreshInlineConsole();
    void appendInlineConsole(const QString &text);
    QString currentContextCommandLine() const;

    ContextStore m_store;
    SessionManager m_sessions;
    LauncherSettings m_settings;

    QStackedWidget *m_stack = nullptr;
    BasicModeWidget *m_basicWidget = nullptr;
    QWidget *m_advancedPage = nullptr;
    QTabWidget *m_advancedTabs = nullptr;
    DockerObservabilityWidget *m_advancedDocker = nullptr;

    QListWidget *m_list = nullptr;
    QLabel *m_hint = nullptr;
    QLineEdit *m_search = nullptr;
    QTimer *m_advancedSearchTimer = nullptr;
    QLabel *m_consoleTitle = nullptr;
    QLabel *m_consoleMeta = nullptr;
    QPlainTextEdit *m_console = nullptr;
    QFrame *m_emptyState = nullptr;
    QLabel *m_emptyTitle = nullptr;
    QLabel *m_emptyBody = nullptr;
    QSystemTrayIcon *m_tray = nullptr;

    QAction *m_actBasic = nullptr;
    QAction *m_actAdvanced = nullptr;
    QAction *m_actIcons = nullptr;
    QAction *m_actList = nullptr;

    QWidget *m_disclaimerContainer = nullptr;
    QString m_consoleContextId;
    QVector<WorkflowMeta> m_basicApps;
    QVector<Context> m_advancedSourceContexts;
    QVector<Context> m_advancedContexts;
    QString m_basicLaunchingContextId;
    QString m_basicLaunchingWorkflowId;
};
