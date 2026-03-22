#include "MainWindow.h"

#include "ContextDiscovery.h"
#include "ContextRowWidget.h"
#include "EditContextDialog.h"
#include "GitHelper.h"

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
#include <QListWidget>
#include <QMenu>
#include <QMessageBox>
#include <QProcess>
#include <QPushButton>
#include <QTimer>
#include <QUrl>
#include <QVBoxLayout>

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

} // namespace

MainWindow::MainWindow(QWidget *parent) : QMainWindow(parent), m_sessions(this)
{
    setWindowTitle(tr("Pipeon"));
    resize(720, 480);

    auto *central = new QWidget(this);
    central->setObjectName(QStringLiteral("mainCentral"));
    auto *root = new QVBoxLayout(central);
    root->setSpacing(14);
    root->setContentsMargins(16, 16, 16, 16);

    auto *header = new QFrame;
    header->setObjectName(QStringLiteral("headerBar"));
    auto *headLay = new QVBoxLayout(header);
    headLay->setSpacing(12);
    headLay->setContentsMargins(14, 14, 14, 14);

    auto *title = new QLabel(tr("Pipeon"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *subtitle = new QLabel(tr("Launch dockpipe per saved folder. Execution stays in DockPipe."));
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
    addSecondary(tr("Remove"), &MainWindow::onRemoveContext, "dangerButton");
    addSecondary(tr("Stop all for repo"), &MainWindow::onStopAllForRepo, "secondaryButton");
    secondaryRow->addStretch(1);
    headLay->addLayout(secondaryRow);

    root->addWidget(header);

    m_hint = new QLabel(tr("Saved contexts appear below. Add folder scans this checkout’s dockpipe workflows when a repo is found. Right-click a row for the same actions."));
    m_hint->setObjectName(QStringLiteral("hintText"));
    m_hint->setWordWrap(true);
    root->addWidget(m_hint);

    auto *listPanel = new QFrame;
    listPanel->setObjectName(QStringLiteral("listPanel"));
    auto *listOuter = new QVBoxLayout(listPanel);
    listOuter->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(this);
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
    auto *emptyTitle = new QLabel(tr("No contexts yet"));
    emptyTitle->setObjectName(QStringLiteral("emptyTitle"));
    emptyTitle->setAlignment(Qt::AlignCenter);
    auto *emptyBody = new QLabel(
        tr("Add a folder to create a context, then launch dockpipe. You can refine workflow and runtime in Edit."));
    emptyBody->setObjectName(QStringLiteral("emptyBody"));
    emptyBody->setWordWrap(true);
    emptyBody->setAlignment(Qt::AlignCenter);
    emptyLay->addWidget(emptyTitle);
    emptyLay->addWidget(emptyBody);

    listOuter->addWidget(m_emptyState, 1);
    listOuter->addWidget(m_list, 1);
    m_emptyState->hide();
    m_list->hide();

    root->addWidget(listPanel, 1);

    setCentralWidget(central);

    m_store.load();
    rebuildContextList();

    connect(m_list, &QListWidget::itemDoubleClicked, this, [this]() { onLaunch(); });
    connect(m_list, &QListWidget::customContextMenuRequested, this, [this](const QPoint &p) {
        if (QListWidgetItem *it = m_list->itemAt(p))
            applyContextMenu(it, m_list->mapToGlobal(p));
    });

    connect(&m_sessions, &SessionManager::sessionStarted, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionStopped, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionFailed, this, &MainWindow::onSessionChanged);
    connect(&m_sessions, &SessionManager::sessionFailed, this,
            [this](const QString &, const QString &err) { QMessageBox::warning(this, tr("Pipeon"), err); });

    setupTray();
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

void MainWindow::rebuildContextList()
{
    clearContextList();

    const bool empty = m_store.contexts.isEmpty();
    m_emptyState->setVisible(empty);
    m_list->setVisible(!empty);

    for (const Context &c : m_store.contexts) {
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
}

void MainWindow::onSessionChanged()
{
    rebuildContextList();
}

QListWidgetItem *MainWindow::currentItem()
{
    return m_list->currentItem();
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
    rebuildContextList();
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
    rebuildContextList();
}

void MainWindow::onRemoveContext()
{
    Context *c = currentContext();
    if (!c)
        return;
    if (m_sessions.isRunning(c->id)) {
        QMessageBox::warning(this, tr("Pipeon"), tr("Stop the session before removing."));
        return;
    }
    for (int i = 0; i < m_store.contexts.size(); ++i) {
        if (m_store.contexts[i].id == c->id) {
            m_store.contexts.removeAt(i);
            break;
        }
    }
    m_store.save();
    rebuildContextList();
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
    rebuildContextList();
}

void MainWindow::onLaunch()
{
    Context *c = currentContext();
    if (!c)
        return;
    if (m_sessions.launch(*c, ContextStore::logsDir()))
        rebuildContextList();
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
    rebuildContextList();
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
        rebuildContextList();
        return;
    }
    for (const Context &x : m_store.contexts) {
        const QString xr = GitHelper::repoRoot(x.workdir);
        if (xr == root && m_sessions.isRunning(x.id))
            m_sessions.stop(x.id);
    }
    rebuildContextList();
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
    menu.addAction(tr("Remove context"), this, &MainWindow::onRemoveContext);
    menu.exec(globalPos);
}
