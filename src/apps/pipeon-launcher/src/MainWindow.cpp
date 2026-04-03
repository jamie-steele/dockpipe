#include "MainWindow.h"

#include "BasicModeWidget.h"
#include "ContextDiscovery.h"
#include "ContextRowWidget.h"
#include "DockpipeChoices.h"
#include "EditContextDialog.h"
#include "GitHelper.h"
#include "WorkflowCatalog.h"

#include <QActionGroup>
#include <QApplication>
#include <QCloseEvent>
#include <QDesktopServices>
#include <QDir>
#include <QDialog>
#include <QFileDialog>
#include <QFileInfo>
#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QMenu>
#include <QMenuBar>
#include <QMessageBox>
#include <QProcess>
#include <QProcessEnvironment>
#include <QPushButton>
#include <QScrollBar>
#include <QSplitter>
#include <QStackedWidget>
#include <QStatusBar>
#include <QTimer>
#include <QUrl>
#include <QVBoxLayout>
#include <QFontDatabase>
#include <QRegularExpression>

namespace {

QString statusLabel(SessionManager &sm, const QString &id, bool *runningOut, bool *failedOut)
{
    *runningOut = sm.isRunning(id);
    if (*runningOut) {
        *failedOut = false;
        return QObject::tr("Running");
    }
    const SessionInfo si = sm.info(id);
    if (si.status == SessionStatus::Failed) {
        *failedOut = true;
        return QObject::tr("Failed");
    }
    *failedOut = false;
    return QObject::tr("Stopped");
}

bool contextMatchesFilter(const Context &c, const QString &filter)
{
    const QString needle = filter.trimmed().toCaseFolded();
    if (needle.isEmpty())
        return true;

    const QString haystack = QStringList{
                                 c.label,
                                 c.workdir,
                                 c.workflow,
                                 c.workflowFile,
                                 c.resolver,
                                 c.strategy,
                                 c.runtime,
                                 c.dockpipeBinary,
                                 c.envFile,
                                 c.id,
                             }
                                 .join(QLatin1Char('\n'))
                                 .toCaseFolded();
    return haystack.contains(needle);
}

QString shellQuote(QString s)
{
    if (s.isEmpty())
        return QStringLiteral("''");
    s.replace(QLatin1Char('\''), QStringLiteral("'\"'\"'"));
    if (s.contains(QRegularExpression(QStringLiteral("[\\s\"'`$&|;<>()\\[\\]{}*!?\\\\]"))))
        return QStringLiteral("'") + s + QStringLiteral("'");
    return s;
}

} // namespace

MainWindow::MainWindow(QWidget *parent) : QMainWindow(parent), m_sessions(this)
{
    setWindowTitle(tr("Pipeon"));
    resize(800, 520);

    m_settings.load();

    setupMenuBar();

    auto *central = new QWidget(this);
    auto *outer = new QVBoxLayout(central);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(0);

    m_stack = new QStackedWidget;
    m_basicWidget = new BasicModeWidget(this);
    m_advancedPage = new QWidget;
    setupAdvancedPage(m_advancedPage);

    m_stack->addWidget(m_basicWidget);
    m_stack->addWidget(m_advancedPage);
    outer->addWidget(m_stack);

    setCentralWidget(central);

    setupDisclaimerBar();

    connect(m_basicWidget, &BasicModeWidget::openProjectRequested, this, &MainWindow::onFileOpenProject);
    connect(m_basicWidget, &BasicModeWidget::refreshAppsRequested, this, &MainWindow::onRefreshAppList);
    connect(m_basicWidget, &BasicModeWidget::launchRequested, this, &MainWindow::onBasicLaunch);
    connect(m_basicWidget, &BasicModeWidget::recentProjectSelected, this, &MainWindow::onBasicOpenRecent);
    connect(m_basicWidget, &BasicModeWidget::continueLastRequested, this, &MainWindow::onBasicContinueLast);
    connect(m_basicWidget, &BasicModeWidget::backToHomeRequested, this, &MainWindow::onBasicBackHome);
    connect(m_basicWidget, &BasicModeWidget::setupMcpRequested, this, &MainWindow::onSetupMcp);

    m_store.load();
    rebuildUi();

    connect(&m_sessions, &SessionManager::sessionStarted, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionStopped, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionFailed, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionOutput, this, &MainWindow::onSessionOutput);
    connect(&m_sessions, &SessionManager::sessionFailed, this,
            [this](const QString &, const QString &err) { QMessageBox::warning(this, tr("Pipeon"), err); });

    setupTray();
    applyUiMode();
}

void MainWindow::setupDisclaimerBar()
{
    if (m_settings.thirdPartyDisclaimerDismissed || m_disclaimerContainer)
        return;

    auto *wrap = new QWidget;
    auto *lay = new QHBoxLayout(wrap);
    lay->setContentsMargins(4, 0, 4, 0);
    lay->setSpacing(8);

    auto *disclaimer = new QLabel(
        tr("Notice: Pipeon does not distribute third-party applications. Dockpipe workflows run on "
           "your machine; install tools from official vendor or distribution channels and accept each "
           "publisher’s terms."));
    disclaimer->setObjectName(QStringLiteral("disclaimerWatermark"));
    disclaimer->setWordWrap(true);
    disclaimer->setAlignment(Qt::AlignLeft | Qt::AlignVCenter);

    auto *dismiss = new QPushButton(tr("Dismiss"));
    dismiss->setObjectName(QStringLiteral("disclaimerDismiss"));
    dismiss->setCursor(Qt::PointingHandCursor);
    connect(dismiss, &QPushButton::clicked, this, &MainWindow::onDismissThirdPartyDisclaimer);

    lay->addWidget(disclaimer, 1);
    lay->addWidget(dismiss, 0, Qt::AlignTop);

    m_disclaimerContainer = wrap;
    statusBar()->addWidget(wrap, 1);
}

void MainWindow::onDismissThirdPartyDisclaimer()
{
    m_settings.thirdPartyDisclaimerDismissed = true;
    m_settings.save();
    if (!m_disclaimerContainer)
        return;
    statusBar()->removeWidget(m_disclaimerContainer);
    QWidget *w = m_disclaimerContainer;
    m_disclaimerContainer = nullptr;
    w->deleteLater();
}

void MainWindow::onRestoreThirdPartyDisclaimer()
{
    m_settings.thirdPartyDisclaimerDismissed = false;
    m_settings.save();
    setupDisclaimerBar();
}

void MainWindow::setupMenuBar()
{
    QMenu *file = menuBar()->addMenu(tr("File"));
    file->addAction(tr("Open project folder…"), this, &MainWindow::onFileOpenProject, QKeySequence::Open);
    file->addAction(tr("Refresh app list"), this, &MainWindow::onRefreshAppList, QKeySequence::Refresh);
    file->addSeparator();
    file->addAction(tr("Quit"), qApp, &QApplication::quit, QKeySequence::Quit);

    QMenu *view = menuBar()->addMenu(tr("View"));
    auto *modeGroup = new QActionGroup(this);
    m_actBasic = view->addAction(tr("Basic mode"));
    m_actBasic->setCheckable(true);
    modeGroup->addAction(m_actBasic);
    m_actAdvanced = view->addAction(tr("Advanced mode"));
    m_actAdvanced->setCheckable(true);
    modeGroup->addAction(m_actAdvanced);
    connect(m_actBasic, &QAction::triggered, this, &MainWindow::onViewBasic);
    connect(m_actAdvanced, &QAction::triggered, this, &MainWindow::onViewAdvanced);

    QMenu *help = menuBar()->addMenu(tr("Help"));
    help->addAction(tr("Show notice in status bar again"), this, &MainWindow::onRestoreThirdPartyDisclaimer);
    help->addAction(tr("Set up Cursor MCP…"), this, &MainWindow::onSetupMcp);
    help->addAction(tr("Third-party software notice…"), this, [this]() {
        QMessageBox::information(
            this, tr("Third-party software"),
            tr("Pipeon is a launcher for dockpipe workflows. It does not ship or bundle third-party "
               "applications.\n\n"
               "If a workflow needs external tools, you install them yourself from official sources. Use of "
               "those products is subject to their respective licensors’ terms."));
    });

    view->addSeparator();
    auto *viewGroup = new QActionGroup(this);
    m_actIcons = view->addAction(tr("Icon grid"));
    m_actIcons->setCheckable(true);
    viewGroup->addAction(m_actIcons);
    m_actList = view->addAction(tr("Compact list"));
    m_actList->setCheckable(true);
    viewGroup->addAction(m_actList);
    connect(m_actIcons, &QAction::triggered, this, &MainWindow::onViewIconGrid);
    connect(m_actList, &QAction::triggered, this, &MainWindow::onViewCompactList);
}

void MainWindow::setupAdvancedPage(QWidget *page)
{
    auto *root = new QVBoxLayout(page);
    root->setSpacing(14);
    root->setContentsMargins(16, 16, 16, 16);

    auto *header = new QFrame;
    header->setObjectName(QStringLiteral("headerBar"));
    auto *headLay = new QVBoxLayout(header);
    headLay->setSpacing(12);
    headLay->setContentsMargins(14, 14, 14, 14);

    auto *title = new QLabel(tr("Contexts (advanced)"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *subtitle = new QLabel(tr("Full DockPipe workflow list per saved folder. Switch to Basic in View for app shortcuts."));
    subtitle->setObjectName(QStringLiteral("appSubtitle"));
    subtitle->setWordWrap(true);
    headLay->addWidget(title);
    headLay->addWidget(subtitle);

    auto *primaryRow = new QHBoxLayout;
    primaryRow->setSpacing(8);
    auto addPrimary = [this, primaryRow](const QString &text, void (MainWindow::*slot)(), const char *objName) {
        auto *b = new QPushButton(text);
        b->setObjectName(QString::fromUtf8(objName));
        connect(b, &QPushButton::clicked, this, slot);
        primaryRow->addWidget(b);
    };
    addPrimary(tr("Launch"), &MainWindow::onLaunch, "primaryButton");
    addPrimary(tr("Relaunch"), &MainWindow::onRelaunch, "primaryButton");
    addPrimary(tr("Stop"), &MainWindow::onStop, "primaryButton");
    addPrimary(tr("Add folder…"), &MainWindow::onAddFolder, "primaryButton");
    primaryRow->addStretch(1);
    headLay->addLayout(primaryRow);

    auto *secondaryRow = new QHBoxLayout;
    secondaryRow->setSpacing(8);
    auto addSecondary = [this, secondaryRow](const QString &text, void (MainWindow::*slot)(), const char *objName) {
        auto *b = new QPushButton(text);
        b->setObjectName(QString::fromUtf8(objName));
        connect(b, &QPushButton::clicked, this, slot);
        secondaryRow->addWidget(b);
    };
    addSecondary(tr("Edit…"), &MainWindow::onEditContext, "secondaryButton");
    addSecondary(tr("Refresh worktrees"), &MainWindow::onRefreshWorktrees, "secondaryButton");
    addSecondary(tr("Open logs"), &MainWindow::onOpenLogs, "secondaryButton");
    addSecondary(tr("Open folder"), &MainWindow::onOpenFolder, "secondaryButton");
    addSecondary(tr("Forget saved row"), &MainWindow::onRemoveContext, "dangerButton");
    addSecondary(tr("Stop all for repo"), &MainWindow::onStopAllForRepo, "secondaryButton");
    secondaryRow->addStretch(1);
    headLay->addLayout(secondaryRow);

    root->addWidget(header);

    m_hint = new QLabel(tr("Saved workflow rows appear below. Add folder scans workflows from this checkout. Right-click a row for actions."));
    m_hint->setObjectName(QStringLiteral("hintText"));
    m_hint->setWordWrap(true);
    root->addWidget(m_hint);

    m_search = new QLineEdit(page);
    m_search->setClearButtonEnabled(true);
    m_search->setPlaceholderText(tr("Search saved rows by label, folder, workflow, resolver…"));
    connect(m_search, &QLineEdit::textChanged, this, &MainWindow::onAdvancedSearchChanged);
    root->addWidget(m_search);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    auto *listPanel = new QFrame;
    listPanel->setObjectName(QStringLiteral("listPanel"));
    auto *listOuter = new QVBoxLayout(listPanel);
    listOuter->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(page);
    m_list->setFrameShape(QFrame::NoFrame);
    m_list->setSpacing(2);
    m_list->setUniformItemSizes(true);
    m_list->setContextMenuPolicy(Qt::CustomContextMenu);
    listOuter->addWidget(m_list, 1);

    m_emptyState = new QFrame;
    m_emptyState->setObjectName(QStringLiteral("emptyState"));
    auto *emptyLay = new QVBoxLayout(m_emptyState);
    emptyLay->setContentsMargins(28, 36, 28, 36);
    emptyLay->setSpacing(8);
    m_emptyTitle = new QLabel(tr("No contexts yet"));
    m_emptyTitle->setObjectName(QStringLiteral("emptyTitle"));
    m_emptyTitle->setAlignment(Qt::AlignCenter);
    m_emptyBody = new QLabel(
        tr("Use Add folder… to import workflows, or use View → Basic mode and open a project folder."));
    m_emptyBody->setObjectName(QStringLiteral("emptyBody"));
    m_emptyBody->setWordWrap(true);
    m_emptyBody->setAlignment(Qt::AlignCenter);
    emptyLay->addWidget(m_emptyTitle);
    emptyLay->addWidget(m_emptyBody);

    listOuter->addWidget(m_emptyState, 1);
    listOuter->addWidget(m_list, 1);
    m_emptyState->hide();
    m_list->hide();

    splitter->addWidget(listPanel);

    auto *consolePanel = new QFrame;
    consolePanel->setObjectName(QStringLiteral("inlineConsolePanel"));
    auto *consoleLay = new QVBoxLayout(consolePanel);
    consoleLay->setContentsMargins(12, 12, 12, 12);
    consoleLay->setSpacing(8);

    m_consoleTitle = new QLabel(tr("Inline CLI"));
    m_consoleTitle->setObjectName(QStringLiteral("consoleTitle"));
    m_consoleMeta = new QLabel(tr("Select a saved row, then launch it to see output here."));
    m_consoleMeta->setObjectName(QStringLiteral("consoleMeta"));
    m_consoleMeta->setWordWrap(true);

    m_console = new QPlainTextEdit(consolePanel);
    m_console->setObjectName(QStringLiteral("inlineConsole"));
    m_console->setReadOnly(true);
    m_console->setPlaceholderText(tr("Command output will appear here."));
    m_console->setLineWrapMode(QPlainTextEdit::NoWrap);
    m_console->setMinimumHeight(120);
    m_console->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));

    consoleLay->addWidget(m_consoleTitle);
    consoleLay->addWidget(m_consoleMeta);
    consoleLay->addWidget(m_console, 1);
    splitter->addWidget(consolePanel);
    splitter->setStretchFactor(0, 3);
    splitter->setStretchFactor(1, 2);
    splitter->setSizes({360, 240});
    root->addWidget(splitter, 1);

    connect(m_list, &QListWidget::customContextMenuRequested, this, [this](const QPoint &p) {
        if (QListWidgetItem *it = m_list->itemAt(p))
            applyContextMenu(it, m_list->mapToGlobal(p));
    });
    connect(m_list, &QListWidget::itemSelectionChanged, this, &MainWindow::onAdvancedSelectionChanged);
}

void MainWindow::applyUiMode()
{
    m_actBasic->blockSignals(true);
    m_actAdvanced->blockSignals(true);
    m_actIcons->blockSignals(true);
    m_actList->blockSignals(true);

    m_stack->setCurrentIndex(m_settings.isAdvanced() ? 1 : 0);
    m_actBasic->setChecked(!m_settings.isAdvanced());
    m_actAdvanced->setChecked(m_settings.isAdvanced());
    const bool basic = !m_settings.isAdvanced();
    m_actIcons->setEnabled(basic);
    m_actList->setEnabled(basic);
    m_actIcons->setChecked(basic && m_settings.isBasicIcons());
    m_actList->setChecked(basic && !m_settings.isBasicIcons());
    m_basicWidget->setViewIconMode(m_settings.isBasicIcons());
    m_basicWidget->setProjectFolder(m_settings.projectFolder);
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.projectFolder.isEmpty()
                                           || !m_settings.recentProjectFolders.isEmpty());
    if (basic)
        m_basicWidget->showHomePage();

    m_actBasic->blockSignals(false);
    m_actAdvanced->blockSignals(false);
    m_actIcons->blockSignals(false);
    m_actList->blockSignals(false);

    updateBasicPage();
}

void MainWindow::onViewBasic()
{
    m_settings.uiMode = QStringLiteral("basic");
    m_settings.save();
    applyUiMode();
}

void MainWindow::onViewAdvanced()
{
    m_settings.uiMode = QStringLiteral("advanced");
    m_settings.save();
    applyUiMode();
}

void MainWindow::onViewIconGrid()
{
    m_settings.basicView = QStringLiteral("icons");
    m_settings.save();
    m_basicWidget->setViewIconMode(true);
    m_actIcons->setChecked(true);
    m_actList->setChecked(false);
}

void MainWindow::onViewCompactList()
{
    m_settings.basicView = QStringLiteral("list");
    m_settings.save();
    m_basicWidget->setViewIconMode(false);
    m_actIcons->setChecked(false);
    m_actList->setChecked(true);
}

void MainWindow::onFileOpenProject()
{
    const QString d = QFileDialog::getExistingDirectory(this, tr("Open project folder"), m_settings.projectFolder);
    if (d.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(d);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->setProjectFolder(m_settings.projectFolder);
    m_basicWidget->showWorkspacePage();
    updateBasicPage();
}

void MainWindow::onBasicBackHome()
{
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.projectFolder.isEmpty()
                                           || !m_settings.recentProjectFolders.isEmpty());
    m_basicWidget->showHomePage();
}

void MainWindow::onBasicOpenRecent(const QString &absPath)
{
    if (absPath.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(absPath);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->setProjectFolder(m_settings.projectFolder);
    m_basicWidget->showWorkspacePage();
    updateBasicPage();
}

void MainWindow::onBasicContinueLast()
{
    QString folder = m_settings.projectFolder;
    if (folder.isEmpty() && !m_settings.recentProjectFolders.isEmpty())
        folder = m_settings.recentProjectFolders.first();
    if (folder.isEmpty())
        return;
    m_settings.projectFolder = QDir::cleanPath(folder);
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(true);
    m_basicWidget->setProjectFolder(m_settings.projectFolder);
    m_basicWidget->showWorkspacePage();
    updateBasicPage();
}

void MainWindow::onSetupMcp()
{
    QString wd = m_settings.projectFolder;
    if (wd.isEmpty()) {
        QMessageBox::information(this, tr("Pipeon"),
                                 tr("Open or select a project folder first (home screen or File → Open project folder)."));
        return;
    }
    wd = QDir::cleanPath(wd);
    const QString script = DockpipeChoices::cursorPrepScriptPath(wd);
    if (script.isEmpty()) {
        QMessageBox::information(
            this, tr("Cursor MCP"),
            tr("Could not find cursor-prep.sh next to a DockPipe checkout.\n"
               "Open a folder inside the dockpipe repository, or set DOCKPIPE_REPO_ROOT to the repo root, then try again."));
        return;
    }
    QProcess proc;
    QProcessEnvironment env = QProcessEnvironment::systemEnvironment();
    env.insert(QStringLiteral("DOCKPIPE_WORKDIR"), wd);
    const QString repo = DockpipeChoices::findRepoRoot(wd);
    if (!repo.isEmpty())
        env.insert(QStringLiteral("DOCKPIPE_REPO_ROOT"), repo);
    proc.setProcessEnvironment(env);
    proc.setWorkingDirectory(wd);
    proc.start(QStringLiteral("bash"), QStringList{script});
    if (!proc.waitForFinished(60000)) {
        proc.kill();
        proc.waitForFinished(3000);
        QMessageBox::warning(this, tr("Cursor MCP"), tr("cursor-prep.sh timed out or could not be run."));
        return;
    }
    if (proc.exitCode() != 0) {
        const QString err = QString::fromUtf8(proc.readAllStandardError() + proc.readAllStandardOutput());
        QMessageBox::warning(this, tr("Cursor MCP"),
                             tr("cursor-prep.sh exited with an error:\n\n%1").arg(err.isEmpty() ? tr("(no output)") : err));
        return;
    }
    QMessageBox::information(
        this, tr("Cursor MCP"),
        tr("Prepared .dockpipe/cursor-dev/ (see AGENT-MCP.md and mcp.json.example).\n"
           "Follow AGENT-MCP.md in Cursor to enable MCP."));
}

void MainWindow::onRefreshAppList()
{
    updateBasicPage();
}

void MainWindow::updateBasicPage()
{
    const QString repo = DockpipeChoices::findRepoRoot(m_settings.projectFolder);
    const QVector<WorkflowMeta> apps = WorkflowCatalog::discoverAppWorkflows(repo, m_settings.projectFolder);
    m_basicWidget->setApps(apps);

    QHash<QString, bool> run;
    if (!m_settings.projectFolder.isEmpty()) {
        const QString wd = QDir::cleanPath(m_settings.projectFolder);
        for (const Context &c : m_store.contexts) {
            if (QDir::cleanPath(c.workdir) != wd)
                continue;
            if (m_sessions.isRunning(c.id))
                run.insert(c.workflow, true);
        }
    }
    m_basicWidget->setRunningByWorkflow(run);
}

void MainWindow::onBasicLaunch(const QString &workflowId)
{
    if (m_settings.projectFolder.isEmpty()) {
        QMessageBox::information(this, tr("Pipeon"),
                                 tr("Choose a project folder first (File → Open project folder, or Choose folder…)."));
        return;
    }
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    Context *c = findContext(m_settings.projectFolder, workflowId, QString());
    if (!c) {
        Context nc = Context::createNew();
        nc.workdir = m_settings.projectFolder;
        nc.workflow = workflowId;
        nc.label = QFileInfo(m_settings.projectFolder).fileName() + QStringLiteral(" — ") + workflowId;
        m_store.contexts.append(nc);
        m_store.save();
        c = &m_store.contexts.last();
    }
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        rebuildUi();
    else if (!m_sessions.isRunning(c->id))
        QMessageBox::warning(this, tr("Pipeon"), tr("Could not start dockpipe (see stderr)."));
}

void MainWindow::setupTray()
{
    m_tray = new QSystemTrayIcon(QIcon(QStringLiteral(":/icon.png")), this);
    m_tray->setToolTip(tr("Pipeon"));
    auto *menu = new QMenu(this);
    menu->addAction(tr("Show"), this, [this]() { show(); raise(); activateWindow(); });
    menu->addSeparator();
    menu->addAction(tr("Quit"), qApp, &QApplication::quit);
    m_tray->setContextMenu(menu);
    connect(m_tray, &QSystemTrayIcon::activated, this, &MainWindow::onTrayActivate);
    m_tray->show();
}

void MainWindow::onTrayActivate(QSystemTrayIcon::ActivationReason reason)
{
    if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick) {
        if (isVisible())
            hide();
        else {
            show();
            raise();
            activateWindow();
        }
    }
}

void MainWindow::closeEvent(QCloseEvent *event)
{
    if (m_tray && m_tray->isVisible()) {
        hide();
        event->ignore();
        return;
    }
    QMainWindow::closeEvent(event);
}

void MainWindow::clearContextList()
{
    while (m_list->count() > 0) {
        QListWidgetItem *it = m_list->item(0);
        QWidget *w = m_list->itemWidget(it);
        m_list->removeItemWidget(it);
        delete w;
        delete m_list->takeItem(0);
    }
}

void MainWindow::rebuildAdvancedContextList()
{
    clearContextList();

    const QString filter = m_search ? m_search->text() : QString();
    int visibleCount = 0;
    const bool noSavedRows = m_store.contexts.isEmpty();

    if (m_emptyTitle && m_emptyBody) {
        if (noSavedRows) {
            m_emptyTitle->setText(tr("No contexts yet"));
            m_emptyBody->setText(
                tr("Use Add folder… to import workflows, or use View → Basic mode and open a project folder."));
        } else {
            m_emptyTitle->setText(tr("No matching rows"));
            m_emptyBody->setText(tr("Try a different search, or clear the filter to show every saved row."));
        }
    }

    for (const Context &c : m_store.contexts) {
        if (!contextMatchesFilter(c, filter))
            continue;
        ++visibleCount;

        bool running = false;
        bool failed = false;
        const QString st = statusLabel(m_sessions, c.id, &running, &failed);

        auto *item = new QListWidgetItem;
        item->setData(Qt::UserRole, c.id);
        item->setSizeHint(QSize(0, 76));
        m_list->addItem(item);

        auto *row = new ContextRowWidget(c, st, running, failed, m_list);
        m_list->setItemWidget(item, row);
    }

    const bool empty = visibleCount == 0;
    m_emptyState->setVisible(empty);
    m_list->setVisible(!empty);
}

void MainWindow::rebuildUi()
{
    rebuildAdvancedContextList();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::onSessionChanged()
{
    rebuildUi();
}

void MainWindow::onAdvancedSearchChanged(const QString &)
{
    rebuildAdvancedContextList();
}

void MainWindow::onAdvancedSelectionChanged()
{
    refreshInlineConsole();
}

void MainWindow::onSessionOutput(const QString &contextId, const QString &text)
{
    if (contextId != m_consoleContextId)
        return;
    appendInlineConsole(text);
}

QListWidgetItem *MainWindow::currentItem()
{
    return m_list->currentItem();
}

Context *MainWindow::findContext(const QString &workdir, const QString &workflow, const QString &workflowFile)
{
    const QString wd = QDir::cleanPath(workdir);
    for (Context &c : m_store.contexts) {
        if (QDir::cleanPath(c.workdir) != wd)
            continue;
        if (c.workflow != workflow)
            continue;
        if (c.workflowFile != workflowFile)
            continue;
        return &c;
    }
    return nullptr;
}

Context *MainWindow::currentContext()
{
    QListWidgetItem *it = currentItem();
    if (!it)
        return nullptr;
    const QString id = it->data(Qt::UserRole).toString();
    for (Context &c : m_store.contexts) {
        if (c.id == id)
            return &c;
    }
    return nullptr;
}

bool MainWindow::hasContext(const Context &c) const
{
    const QString wd = QDir::cleanPath(c.workdir);
    for (const Context &ex : m_store.contexts) {
        if (QDir::cleanPath(ex.workdir) != wd)
            continue;
        if (ex.workflow == c.workflow && ex.workflowFile == c.workflowFile)
            return true;
    }
    return false;
}

void MainWindow::onAddFolder()
{
    const QString dir = QFileDialog::getExistingDirectory(this, tr("Choose folder"));
    if (dir.isEmpty())
        return;
    const QString clean = QDir::cleanPath(dir);
    const QVector<Context> batch = ContextDiscovery::contextsForWorkdir(clean);
    int added = 0;
    for (const Context &c : batch) {
        if (hasContext(c))
            continue;
        m_store.contexts.append(c);
        ++added;
    }
    if (added == 0) {
        QMessageBox::information(this, tr("Pipeon"),
                                 tr("No new contexts to add — all matching workflows already exist for this folder."));
        return;
    }
    m_store.save();
    rebuildUi();
}

void MainWindow::onRefreshWorktrees()
{
    Context *c = currentContext();
    QString path = c ? c->workdir : QString();
    if (path.isEmpty()) {
        path = QFileDialog::getExistingDirectory(this, tr("Choose repository root"));
        if (path.isEmpty())
            return;
    }
    const QString root = GitHelper::repoRoot(path);
    if (root.isEmpty()) {
        QMessageBox::information(this, tr("Pipeon"), tr("Not a git repository."));
        return;
    }
    const QVector<WorktreeRow> rows = GitHelper::listWorktrees(root);
    for (const WorktreeRow &r : rows) {
        bool exists = false;
        for (const Context &ex : m_store.contexts) {
            if (QDir::cleanPath(ex.workdir) == QDir::cleanPath(r.path)) {
                exists = true;
                break;
            }
        }
        if (exists)
            continue;
        Context nc = Context::createNew();
        nc.workdir = r.path;
        nc.label = QFileInfo(r.path).fileName();
        if (!r.branch.isEmpty())
            nc.label += QStringLiteral(" (") + r.branch + QStringLiteral(")");
        nc.workflow = QStringLiteral("vscode");
        m_store.contexts.append(nc);
    }
    m_store.save();
    rebuildUi();
}

void MainWindow::onRemoveContext()
{
    Context *c = currentContext();
    if (!c)
        return;
    if (m_sessions.isRunning(c->id)) {
        QMessageBox::warning(this, tr("Pipeon"), tr("Stop this context before forgetting the saved row."));
        return;
    }
    const QString label = c->label.isEmpty() ? c->workdir : c->label;
    const auto choice = QMessageBox::question(
        this, tr("Forget saved row"),
        tr("Remove \"%1\" from Pipeon?\n\nThis only removes the saved launcher row. It does not delete files from disk.")
            .arg(label));
    if (choice != QMessageBox::Yes) {
        return;
    }
    for (int i = 0; i < m_store.contexts.size(); ++i) {
        if (m_store.contexts[i].id == c->id) {
            m_store.contexts.removeAt(i);
            break;
        }
    }
    m_store.save();
    rebuildUi();
}

void MainWindow::onEditContext()
{
    Context *c = currentContext();
    if (!c)
        return;

    EditContextDialog dlg(*c, this);
    if (dlg.exec() != QDialog::Accepted)
        return;

    *c = dlg.editedContext();
    m_store.save();
    rebuildUi();
}

void MainWindow::onLaunch()
{
    Context *c = currentContext();
    if (!c)
        return;
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        rebuildUi();
    else if (!m_sessions.isRunning(c->id))
        QMessageBox::warning(this, tr("Pipeon"), tr("Could not start dockpipe (see stderr)."));
}

void MainWindow::onRelaunch()
{
    Context *c = currentContext();
    if (!c)
        return;
    if (m_sessions.isRunning(c->id))
        m_sessions.stop(c->id);
    QTimer::singleShot(400, this, [this]() { onLaunch(); });
}

void MainWindow::onStop()
{
    Context *c = currentContext();
    if (!c)
        return;
    m_sessions.stop(c->id);
    rebuildUi();
}

void MainWindow::onStopAllForRepo()
{
    Context *c = currentContext();
    if (!c)
        return;
    const QString root = GitHelper::repoRoot(c->workdir);
    if (root.isEmpty()) {
        QMessageBox::information(this, tr("Pipeon"), tr("Not a git repository; stopping this context only."));
        m_sessions.stop(c->id);
        rebuildUi();
        return;
    }
    for (const Context &x : m_store.contexts) {
        const QString xr = GitHelper::repoRoot(x.workdir);
        if (xr == root && m_sessions.isRunning(x.id))
            m_sessions.stop(x.id);
    }
    rebuildUi();
}

void MainWindow::onOpenLogs()
{
    Context *c = currentContext();
    if (!c)
        return;
    const SessionInfo si = m_sessions.info(c->id);
    QString path = si.logPath;
    if (path.isEmpty()) {
        path = ContextStore::logsDir();
    }
    QDesktopServices::openUrl(QUrl::fromLocalFile(QFileInfo(path).isDir() ? path : QFileInfo(path).path()));
}

void MainWindow::onOpenFolder()
{
    Context *c = currentContext();
    if (!c)
        return;
    QDesktopServices::openUrl(QUrl::fromLocalFile(c->workdir));
}

void MainWindow::applyContextMenu(QListWidgetItem *, const QPoint &globalPos)
{
    QMenu menu(this);
    menu.addAction(tr("Launch"), this, &MainWindow::onLaunch);
    menu.addAction(tr("Relaunch"), this, &MainWindow::onRelaunch);
    menu.addAction(tr("Stop"), this, &MainWindow::onStop);
    menu.addAction(tr("Stop all for repo"), this, &MainWindow::onStopAllForRepo);
    menu.addSeparator();
    menu.addAction(tr("Open logs"), this, &MainWindow::onOpenLogs);
    menu.addAction(tr("Open folder"), this, &MainWindow::onOpenFolder);
    menu.addAction(tr("Edit settings…"), this, &MainWindow::onEditContext);
    menu.addSeparator();
    menu.addAction(tr("Forget saved row"), this, &MainWindow::onRemoveContext);
    menu.exec(globalPos);
}

void MainWindow::refreshInlineConsole()
{
    if (!m_console || !m_consoleTitle || !m_consoleMeta)
        return;

    Context *c = currentContext();
    if (!c) {
        m_consoleContextId.clear();
        m_consoleTitle->setText(tr("Inline CLI"));
        m_consoleMeta->setText(tr("Select a saved row, then launch it to see output here."));
        m_console->setPlainText(QString());
        return;
    }

    m_consoleContextId = c->id;
    m_consoleTitle->setText(c->label.isEmpty() ? tr("Inline CLI") : c->label);
    m_consoleMeta->setText(currentContextCommandLine());

    const SessionInfo si = m_sessions.info(c->id);
    QString text;
    if (!si.logPath.isEmpty()) {
        QFile f(si.logPath);
        if (f.open(QIODevice::ReadOnly | QIODevice::Text))
            text = QString::fromLocal8Bit(f.readAll());
    }
    if (text.isEmpty()) {
        text = tr("# No output yet.\n# Launch this row to run dockpipe inline here.");
    }
    m_console->setPlainText(text);
    auto *bar = m_console->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}

void MainWindow::appendInlineConsole(const QString &text)
{
    if (!m_console || text.isEmpty())
        return;
    m_console->moveCursor(QTextCursor::End);
    m_console->insertPlainText(text);
    auto *bar = m_console->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}

QString MainWindow::currentContextCommandLine() const
{
    const Context *c = const_cast<MainWindow *>(this)->currentContext();
    if (!c)
        return tr("Select a saved row, then launch it to see output here.");

    SessionInfo si = m_sessions.info(c->id);
    QString program = si.program;
    QStringList args = si.arguments;
    if (program.isEmpty()) {
        program = c->dockpipeBinary.trimmed();
        if (program.isEmpty())
            program = QStringLiteral("dockpipe");
        args = SessionManager::dockpipeArguments(*c);
    }

    QStringList parts;
    parts.append(shellQuote(program));
    for (const QString &arg : args)
        parts.append(shellQuote(arg));
    return parts.join(QLatin1Char(' '));
}
