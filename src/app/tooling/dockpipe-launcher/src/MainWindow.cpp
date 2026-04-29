#include "MainWindow.h"

#include "BasicModeWidget.h"
#include "ContextDiscovery.h"
#include "ContextRowWidget.h"
#include "DockpipeChoices.h"
#include "DockerObservabilityWidget.h"
#include "GitHelper.h"
#include "LogViewerDialog.h"
#include "PackageManagerDialog.h"
#include "PromptDialog.h"
#include "SettingsDialog.h"
#include "WorkflowCatalog.h"

#include <QActionGroup>
#include <QApplication>
#include <QCloseEvent>
#include <QCoreApplication>
#include <QDesktopServices>
#include <QDir>
#include <QDialog>
#include <QFileDialog>
#include <QFileInfo>
#include <QFrame>
#include <QHBoxLayout>
#include <QIcon>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
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
#include <QGuiApplication>
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

QString contextDisplayKey(const Context &c)
{
    return QStringList{
        QDir::cleanPath(c.workdir),
        c.workflow,
        c.workflowFile,
        c.label,
        c.resolver,
        c.runtime,
    }.join(QLatin1Char('\x1f'));
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
    setWindowTitle(tr("DockPipe Launcher"));
    setWindowIcon(QGuiApplication::windowIcon());
    resize(800, 520);

    m_settings.load();
    const QStringList args = QCoreApplication::arguments();
    const bool startHome = args.contains(QStringLiteral("--start-home"));

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

    connect(&m_sessions, &SessionManager::sessionStarted, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionReady, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionStopped, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionFailed, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionOutput, this, &MainWindow::onSessionOutput);
    connect(&m_sessions, &SessionManager::sessionPrompt, this, &MainWindow::onSessionPrompt);
    connect(&m_sessions, &SessionManager::sessionFailed, this,
            [this](const QString &, const QString &err) { QMessageBox::warning(this, tr("DockPipe Launcher"), err); });
    QTimer::singleShot(0, this, [this, startHome]() {
        m_store.load();
        setupTray();
        if (startHome) {
            activateHome();
            QTimer::singleShot(200, this, [this]() {
                rebuildAdvancedContextList();
                refreshInlineConsole();
            });
            return;
        }
        rebuildUi();
        applyUiMode();
    });
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
        tr("Notice: DockPipe Launcher does not distribute third-party applications. Dockpipe workflows run on "
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

    QMenu *settingsMenu = menuBar()->addMenu(tr("Settings"));
    settingsMenu->addAction(tr("Preferences…"), this, &MainWindow::onOpenSettings);

    QMenu *packagesMenu = menuBar()->addMenu(tr("Packages"));
    packagesMenu->addAction(tr("Manage Packages…"), this, &MainWindow::onManagePackages);

    QMenu *help = menuBar()->addMenu(tr("Help"));
    help->addAction(tr("About DockPipe Launcher…"), this, &MainWindow::onAbout);
    help->addSeparator();
    help->addAction(tr("Show notice in status bar again"), this, &MainWindow::onRestoreThirdPartyDisclaimer);
    help->addAction(tr("Third-party software notice…"), this, [this]() {
        QMessageBox::information(
            this, tr("Third-party software"),
            tr("DockPipe Launcher is a launcher for dockpipe workflows. It does not ship or bundle third-party "
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

void MainWindow::onAbout()
{
    QMessageBox box(this);
    box.setWindowTitle(tr("About DockPipe Launcher"));
    box.setIcon(QMessageBox::Information);
    box.setTextFormat(Qt::RichText);
    box.setTextInteractionFlags(Qt::TextBrowserInteraction);
    box.setText(
        tr("<h3>DockPipe Launcher</h3>"
           "<p>DockPipe Launcher is the desktop shell and local-first workspace surface for DockPipe workflows.</p>"
           "<p><a href=\"https://dockpipe.com\">dockpipe.com</a></p>"));
    box.setStandardButtons(QMessageBox::Ok);
    box.exec();
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
    auto *subtitle = new QLabel(tr("Project workflows for the current folder. Right-click a row for launch and session actions."));
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
    addPrimary(tr("Open Logs"), &MainWindow::onOpenLogs, "primaryButton");
    addPrimary(tr("Stop All for Repo"), &MainWindow::onStopAllForRepo, "primaryButton");
    primaryRow->addStretch(1);
    headLay->addLayout(primaryRow);

    root->addWidget(header);
    m_advancedTabs = new QTabWidget(page);
    m_advancedTabs->setObjectName(QStringLiteral("surfaceTabs"));

    auto *contextsPage = new QWidget(m_advancedTabs);
    auto *contextsRoot = new QVBoxLayout(contextsPage);
    contextsRoot->setContentsMargins(0, 0, 0, 0);
    contextsRoot->setSpacing(14);

    m_hint = new QLabel(tr("Workflows below are discovered from the current project folder. Right-click a row for actions."));
    m_hint->setObjectName(QStringLiteral("hintText"));
    m_hint->setWordWrap(true);
    contextsRoot->addWidget(m_hint);

    m_search = new QLineEdit(page);
    m_search->setClearButtonEnabled(true);
    m_search->setPlaceholderText(tr("Search workflows by label, folder, workflow, resolver…"));
    connect(m_search, &QLineEdit::textChanged, this, &MainWindow::onAdvancedSearchChanged);
    contextsRoot->addWidget(m_search);

    m_advancedSearchTimer = new QTimer(this);
    m_advancedSearchTimer->setSingleShot(true);
    m_advancedSearchTimer->setInterval(120);
    connect(m_advancedSearchTimer, &QTimer::timeout, this, &MainWindow::applyAdvancedContextFilter);

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
    m_emptyTitle = new QLabel(tr("No workflows yet"));
    m_emptyTitle->setObjectName(QStringLiteral("emptyTitle"));
    m_emptyTitle->setAlignment(Qt::AlignCenter);
    m_emptyBody = new QLabel(
        tr("Open a project folder in Basic mode or with File → Open project folder."));
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
    m_consoleMeta = new QLabel(tr("Select a workflow row, then launch it to see output here."));
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
    contextsRoot->addWidget(splitter, 1);

    m_advancedDocker = new DockerObservabilityWidget(m_advancedTabs);
    m_advancedTabs->addTab(contextsPage, tr("Contexts"));
    m_advancedTabs->addTab(m_advancedDocker, tr("Docker"));
    connect(m_advancedTabs, &QTabWidget::currentChanged, this, [this](int index) {
        if (m_advancedDocker)
            m_advancedDocker->setActive(index == 1);
    });

    root->addWidget(m_advancedTabs, 1);

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

    rebuildAdvancedContextList();
    updateBasicPage();
    refreshInlineConsole();
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

void MainWindow::onOpenSettings()
{
    SettingsDialog dialog(m_settings, this);
    if (dialog.exec() != QDialog::Accepted)
        return;
    m_settings = dialog.updatedSettings();
    m_settings.save();
    rebuildUi();
}

void MainWindow::onManagePackages()
{
    PackageManagerDialog dialog(m_settings.projectFolder, this);
    dialog.exec();
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
    rebuildUi();
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
    rebuildUi();
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
    rebuildUi();
}

void MainWindow::activateHome()
{
    m_settings.uiMode = QStringLiteral("basic");
    m_settings.projectFolder.clear();
    m_settings.save();
    m_basicWidget->setProjectFolder(QString());
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    m_basicWidget->setContinueLastVisible(!m_settings.recentProjectFolders.isEmpty());
    m_basicWidget->showHomePage();
    applyUiMode();
    show();
    if (isMinimized())
        showNormal();
    raise();
    activateWindow();
}

void MainWindow::onRefreshAppList()
{
    rebuildAdvancedContextList();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::updateBasicPage()
{
    if (m_settings.projectFolder.isEmpty()) {
        m_basicApps.clear();
        m_basicWidget->setApps({});
        m_basicWidget->setRunningByWorkflow({});
        m_basicWidget->clearLaunchingWorkflow();
        return;
    }

    const QString repo = DockpipeChoices::findRepoRoot(m_settings.projectFolder);
    const QVector<WorkflowMeta> apps = WorkflowCatalog::discoverAppWorkflows(repo, m_settings.projectFolder);
    m_basicApps = apps;
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
    if (!m_basicLaunchingWorkflowId.isEmpty()) {
        QString label = m_basicLaunchingWorkflowId;
        for (const WorkflowMeta &meta : m_basicApps) {
            QString metaWorkflowId = meta.workflowId;
            if (metaWorkflowId == QStringLiteral("pipeon") || metaWorkflowId == QStringLiteral("Pipeon"))
                metaWorkflowId = QStringLiteral("pipeon-dev-stack");
            if (metaWorkflowId == m_basicLaunchingWorkflowId) {
                label = meta.displayName;
                break;
            }
        }
        m_basicWidget->setLaunchingWorkflow(m_basicLaunchingWorkflowId, label);
    } else {
        m_basicWidget->clearLaunchingWorkflow();
    }
}

void MainWindow::onBasicLaunch(const QString &workflowId)
{
    if (m_settings.projectFolder.isEmpty()) {
        QMessageBox::information(this, tr("DockPipe Launcher"),
                                 tr("Choose a project folder first (File → Open project folder, or Choose folder…)."));
        return;
    }
    m_settings.addRecentProject(m_settings.projectFolder);
    m_settings.save();
    m_basicWidget->setRecentProjects(m_settings.recentProjectFolders);
    QString effectiveWorkflowId = workflowId;
    if (effectiveWorkflowId == QStringLiteral("pipeon") || effectiveWorkflowId == QStringLiteral("Pipeon"))
        effectiveWorkflowId = QStringLiteral("pipeon-dev-stack");
    for (const WorkflowMeta &meta : m_basicApps) {
        QString metaWorkflowId = meta.workflowId;
        if (metaWorkflowId == QStringLiteral("pipeon") || metaWorkflowId == QStringLiteral("Pipeon"))
            metaWorkflowId = QStringLiteral("pipeon-dev-stack");
        if (metaWorkflowId == effectiveWorkflowId) {
            m_basicLaunchingWorkflowId = effectiveWorkflowId;
            break;
        }
    }
    if (m_basicLaunchingWorkflowId.isEmpty())
        m_basicLaunchingWorkflowId = effectiveWorkflowId;
    updateBasicPage();

    QTimer::singleShot(0, this, [this, effectiveWorkflowId]() {
    Context *c = findContext(m_settings.projectFolder, effectiveWorkflowId, QString());
    if (!c) {
        Context nc = Context::createNew();
        nc.workdir = m_settings.projectFolder;
        nc.workflow = effectiveWorkflowId;
        nc.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder);
        nc.label = QFileInfo(m_settings.projectFolder).fileName() + QStringLiteral(" — ") + effectiveWorkflowId;
        m_store.contexts.append(nc);
        m_store.save();
        c = &m_store.contexts.last();
    } else if (c->dockpipeBinary.trimmed().isEmpty() || c->dockpipeBinary.trimmed() == QStringLiteral("dockpipe")) {
        c->dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder);
        m_store.save();
    }
    m_basicLaunchingContextId = c->id;
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        rebuildUi();
    else if (!m_sessions.isRunning(c->id)) {
        m_basicLaunchingContextId.clear();
        m_basicLaunchingWorkflowId.clear();
        updateBasicPage();
        QMessageBox::warning(this, tr("DockPipe Launcher"), tr("Could not start dockpipe (see stderr)."));
    }
    });
}

void MainWindow::setupTray()
{
    m_tray = new QSystemTrayIcon(QGuiApplication::windowIcon(), this);
    m_tray->setToolTip(tr("DockPipe Launcher"));
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
    const bool hasProject = !m_settings.projectFolder.isEmpty();
    m_advancedSourceContexts.clear();
    if (hasProject)
        m_advancedSourceContexts = ContextDiscovery::contextsForWorkdir(m_settings.projectFolder);
    applyAdvancedContextFilter();
}

void MainWindow::applyAdvancedContextFilter()
{
    QString selectedKey;
    if (Context *current = currentAdvancedDisplayContext())
        selectedKey = contextDisplayKey(*current);

    clearContextList();
    m_advancedContexts.clear();

    const QString filter = m_search ? m_search->text() : QString();
    int visibleCount = 0;
    const bool hasProject = !m_settings.projectFolder.isEmpty();
    const bool noProjectRows = m_advancedSourceContexts.isEmpty();

    if (m_emptyTitle && m_emptyBody) {
        if (!hasProject) {
            m_emptyTitle->setText(tr("No project selected"));
            m_emptyBody->setText(tr("Open a project folder in Basic mode or with File → Open project folder."));
        } else if (noProjectRows) {
            m_emptyTitle->setText(tr("No workflows found"));
            m_emptyBody->setText(
                tr("No DockPipe workflows were discovered for the current project folder."));
        } else {
            m_emptyTitle->setText(tr("No matching workflows"));
            m_emptyBody->setText(tr("Try a different search, or clear the filter to show every workflow."));
        }
    }

    int restoreRow = -1;
    for (Context c : m_advancedSourceContexts) {
        if (Context *stored = findStoredContextForDisplay(c)) {
            c = *stored;
        } else if (c.dockpipeBinary.trimmed().isEmpty()) {
            c.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(c.workdir);
        }
        if (!contextMatchesFilter(c, filter))
            continue;
        ++visibleCount;
        m_advancedContexts.append(c);

        bool running = false;
        bool failed = false;
        QString st = tr("Stopped");
        if (Context *stored = findStoredContextForDisplay(c))
            st = statusLabel(m_sessions, stored->id, &running, &failed);

        auto *item = new QListWidgetItem;
        item->setData(Qt::UserRole, m_advancedContexts.size() - 1);
        item->setSizeHint(QSize(0, 76));
        m_list->addItem(item);
        if (!selectedKey.isEmpty() && contextDisplayKey(c) == selectedKey)
            restoreRow = m_list->count() - 1;

        auto *row = new ContextRowWidget(c, st, running, failed, m_list);
        m_list->setItemWidget(item, row);
    }

    const bool empty = visibleCount == 0;
    m_emptyState->setVisible(empty);
    m_list->setVisible(!empty);
    if (restoreRow >= 0)
        m_list->setCurrentRow(restoreRow);
}

void MainWindow::rebuildUi()
{
    rebuildAdvancedContextList();
    updateBasicPage();
    refreshInlineConsole();
}

void MainWindow::onSessionChanged()
{
    if (!m_basicLaunchingContextId.isEmpty()) {
        const SessionInfo si = m_sessions.info(m_basicLaunchingContextId);
        if (si.ready || si.status == SessionStatus::Failed || si.status == SessionStatus::Stopped) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
        }
    }
    rebuildUi();
}

void MainWindow::onAdvancedSearchChanged(const QString &)
{
    if (m_advancedSearchTimer)
        m_advancedSearchTimer->start();
    else
        applyAdvancedContextFilter();
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

void MainWindow::onSessionPrompt(const QString &contextId, const QString &payload)
{
    const QJsonDocument doc = QJsonDocument::fromJson(payload.toUtf8());
    if (!doc.isObject()) {
        QMessageBox::warning(this, tr("DockPipe Launcher"), tr("Received an invalid DockPipe prompt payload."));
        m_sessions.sendInput(contextId, QString());
        return;
    }

    const QJsonObject obj = doc.object();
    const QString type = obj.value(QStringLiteral("type")).toString();
    const QString title = obj.value(QStringLiteral("title")).toString(tr("DockPipe Prompt"));
    const QString message = obj.value(QStringLiteral("message")).toString();
    const QString defaultValue = obj.value(QStringLiteral("default")).toString();
    const bool sensitive = obj.value(QStringLiteral("sensitive")).toBool(false);

    QStringList items;
    const QJsonArray options = obj.value(QStringLiteral("options")).toArray();
    for (const QJsonValue &value : options) {
        const QString option = value.toString();
        if (!option.isEmpty())
            items.append(option);
    }

    QString response = defaultValue;
    if (type == QStringLiteral("confirm") || type == QStringLiteral("input") || type == QStringLiteral("choice")) {
        PromptDialog dialog({type, title, message, defaultValue, items, sensitive}, this);
        dialog.exec();
        response = dialog.response();
    } else {
        QMessageBox::information(this, tr("DockPipe Launcher"),
                                 tr("Unsupported prompt type from DockPipe: %1").arg(type));
    }

    m_sessions.sendInput(contextId, response);
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

Context *MainWindow::findStoredContextForDisplay(const Context &display)
{
    return findContext(display.workdir, display.workflow, display.workflowFile);
}

Context *MainWindow::ensureStoredContextForDisplay(const Context &display)
{
    if (Context *existing = findStoredContextForDisplay(display))
        return existing;

    Context stored = display;
    if (stored.id.trimmed().isEmpty())
        stored.id = Context::createNew().id;
    if (stored.dockpipeBinary.trimmed().isEmpty())
        stored.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(stored.workdir);
    m_store.contexts.append(stored);
    m_store.save();
    return &m_store.contexts.last();
}

Context *MainWindow::currentAdvancedDisplayContext()
{
    QListWidgetItem *it = currentItem();
    if (!it)
        return nullptr;
    bool ok = false;
    const int index = it->data(Qt::UserRole).toInt(&ok);
    if (!ok || index < 0 || index >= m_advancedContexts.size())
        return nullptr;
    return &m_advancedContexts[index];
}

Context *MainWindow::currentContext()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return nullptr;
    return findStoredContextForDisplay(*display);
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

void MainWindow::onLaunch()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return;
    Context *c = ensureStoredContextForDisplay(*display);
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        rebuildUi();
    else if (!m_sessions.isRunning(c->id))
        QMessageBox::warning(this, tr("DockPipe Launcher"), tr("Could not start dockpipe (see stderr)."));
}

void MainWindow::onRelaunch()
{
    Context *display = currentAdvancedDisplayContext();
    if (!display)
        return;
    Context *c = ensureStoredContextForDisplay(*display);
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
    QString path = m_settings.projectFolder;
    if (path.isEmpty()) {
        if (Context *display = currentAdvancedDisplayContext())
            path = display->workdir;
    }
    if (path.isEmpty())
        return;
    const QString root = GitHelper::repoRoot(path);
    if (root.isEmpty()) {
        if (Context *c = currentContext())
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
    if (!c) {
        const QString wd = QDir::cleanPath(m_settings.projectFolder);
        for (int i = m_store.contexts.size() - 1; i >= 0; --i) {
            if (QDir::cleanPath(m_store.contexts[i].workdir) == wd) {
                c = &m_store.contexts[i];
                break;
            }
        }
    }
    if (!c) {
        QMessageBox::information(this, tr("DockPipe Launcher"), tr("No logs yet for this project."));
        return;
    }
    const SessionInfo si = m_sessions.info(c->id);
    QString path = si.logPath;
    if (path.isEmpty()) {
        const QDir logsRoot(ContextStore::logsDir());
        const QStringList matches = logsRoot.entryList({c->id + QStringLiteral("-*.log")}, QDir::Files, QDir::Time);
        if (!matches.isEmpty())
            path = logsRoot.filePath(matches.first());
    }

    LogViewerDialog dialog(c->label.isEmpty() ? tr("Session logs") : tr("%1 logs").arg(c->label),
                           path,
                           currentContextCommandLine(),
                           m_sessions.isRunning(c->id),
                           this);
    dialog.exec();
}

void MainWindow::onOpenFolder()
{
    const QString path = m_settings.projectFolder.isEmpty()
                             ? (currentAdvancedDisplayContext() ? currentAdvancedDisplayContext()->workdir : QString())
                             : m_settings.projectFolder;
    if (path.isEmpty())
        return;
    QDesktopServices::openUrl(QUrl::fromLocalFile(path));
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
    menu.exec(globalPos);
}

void MainWindow::refreshInlineConsole()
{
    if (!m_console || !m_consoleTitle || !m_consoleMeta)
        return;

    Context *display = currentAdvancedDisplayContext();
    Context *c = currentContext();
    if (!display && !c) {
        m_consoleContextId.clear();
        m_consoleTitle->setText(tr("Inline CLI"));
        m_consoleMeta->setText(tr("Select a workflow row, then launch it to see output here."));
        m_console->setPlainText(QString());
        return;
    }

    const Context *metaContext = c ? c : display;
    m_consoleContextId = c ? c->id : QString();
    m_consoleTitle->setText(metaContext->label.isEmpty() ? tr("Inline CLI") : metaContext->label);
    m_consoleMeta->setText(currentContextCommandLine());

    const SessionInfo si = c ? m_sessions.info(c->id) : SessionInfo{};
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
    Context *display = const_cast<MainWindow *>(this)->currentAdvancedDisplayContext();
    if (!display)
        return tr("Select a workflow row, then launch it to see output here.");
    const Context *c = const_cast<MainWindow *>(this)->currentContext();
    if (!c)
        c = display;

    SessionInfo si = m_sessions.info(c->id);
    QString program = si.program;
    QStringList args = si.arguments;
    if (program.isEmpty()) {
        program = c->dockpipeBinary.trimmed();
        if (program.isEmpty())
            program = DockpipeChoices::preferredDockpipeBinary(c->workdir);
        args = SessionManager::dockpipeArguments(*c);
    }

    QStringList parts;
    parts.append(shellQuote(program));
    for (const QString &arg : args)
        parts.append(shellQuote(arg));
    return parts.join(QLatin1Char(' '));
}
