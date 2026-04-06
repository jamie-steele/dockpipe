#include "BasicModeWidget.h"

#include <QAbstractItemView>
#include <QWidget>
#include <QDir>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QListWidget>
#include <QListWidgetItem>
#include <QPushButton>
#include <QSizePolicy>
#include <QStackedWidget>
#include <QVBoxLayout>

namespace {

QIcon appIconForWorkflow(const QString &workflowId)
{
    if (workflowId == QStringLiteral("pipeon-dev-stack") || workflowId == QStringLiteral("pipeon")) {
        return QIcon(QStringLiteral(":/icon.png"));
    }
    if (workflowId == QStringLiteral("vscode")) {
        return QIcon(QStringLiteral(":/app-vscode.png"));
    }
    if (workflowId == QStringLiteral("cursor-dev")) {
        return QIcon(QStringLiteral(":/app-cursor-dev.png"));
    }
    return QIcon(QStringLiteral(":/icon.png"));
}

} // namespace

BasicModeWidget::BasicModeWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName(QStringLiteral("basicMode"));

    m_stack = new QStackedWidget(this);
    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->addWidget(m_stack);

    // --- Home ---
    m_homePage = new QWidget;
    auto *homeLay = new QVBoxLayout(m_homePage);
    homeLay->setSpacing(12);
    homeLay->setContentsMargins(12, 12, 12, 12);

    auto *homeTitle = new QLabel(tr("Pipeon"));
    homeTitle->setObjectName(QStringLiteral("appTitle"));
    auto *homeSub = new QLabel(
        tr("Open a project folder to see DockPipe workflows. Recent folders appear here — pick one or browse."));
    homeSub->setObjectName(QStringLiteral("appSubtitle"));
    homeSub->setWordWrap(true);

    m_recentList = new QListWidget;
    m_recentList->setObjectName(QStringLiteral("basicRecentList"));
    m_recentList->setSpacing(2);
    m_recentList->setUniformItemSizes(true);
    m_recentList->setVerticalScrollMode(QAbstractItemView::ScrollPerPixel);
    m_recentList->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_recentList->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    auto emitRecent = [this](QListWidgetItem *it) {
        if (!it)
            return;
        const QString p = it->data(Qt::UserRole).toString();
        if (!p.isEmpty())
            emit recentProjectSelected(p);
    };
    connect(m_recentList, &QListWidget::itemClicked, this, emitRecent);
    connect(m_recentList, &QListWidget::itemActivated, this, emitRecent);

    m_homeEmptyHint = new QLabel(tr("No recent projects yet. Use Open project… below."));
    m_homeEmptyHint->setObjectName(QStringLiteral("hintText"));
    m_homeEmptyHint->setWordWrap(true);

    auto *homeBtns = new QHBoxLayout;
    homeBtns->setSpacing(8);
    m_openProjectHome = new QPushButton(tr("Open project…"));
    m_openProjectHome->setObjectName(QStringLiteral("primaryButton"));
    m_continueLast = new QPushButton(tr("Continue last project"));
    m_continueLast->setObjectName(QStringLiteral("secondaryButton"));
    m_continueLast->setVisible(false);
    connect(m_openProjectHome, &QPushButton::clicked, this, &BasicModeWidget::openProjectRequested);
    connect(m_continueLast, &QPushButton::clicked, this, &BasicModeWidget::continueLastRequested);
    homeBtns->addWidget(m_openProjectHome);
    homeBtns->addWidget(m_continueLast);
    homeBtns->addStretch(1);

    homeLay->addWidget(homeTitle);
    homeLay->addWidget(homeSub);
    homeLay->addWidget(m_homeEmptyHint);
    homeLay->addWidget(m_recentList, 0);
    homeLay->addStretch(1);
    homeLay->addLayout(homeBtns);

    // --- Workspace ---
    m_workspacePage = new QWidget;
    auto *root = new QVBoxLayout(m_workspacePage);
    root->setSpacing(12);
    root->setContentsMargins(12, 12, 12, 12);

    auto *navRow = new QHBoxLayout;
    m_backHome = new QPushButton(tr("← Projects"));
    m_backHome->setObjectName(QStringLiteral("secondaryButton"));
    m_backHome->setToolTip(tr("Back to the project list"));
    connect(m_backHome, &QPushButton::clicked, this, &BasicModeWidget::backToHomeRequested);
    navRow->addWidget(m_backHome);
    navRow->addStretch(1);
    root->addLayout(navRow);

    auto *title = new QLabel(tr("Apps"));
    title->setObjectName(QStringLiteral("appTitle"));

    auto *sub = new QLabel(
        tr("Launch a tool for this folder. It is passed to dockpipe as --workdir (mounted in the container).\n"
           "The cursor-dev app starts a long-lived Docker session then opens Cursor on the host — "
           "that is not the same as “Set up Cursor MCP”, which only writes package-scoped hints under bin/.dockpipe and does not start Docker."));
    sub->setObjectName(QStringLiteral("appSubtitle"));
    sub->setWordWrap(true);

    auto *projRow = new QHBoxLayout;
    m_projectLabel = new QLabel(tr("No project folder"));
    m_projectLabel->setObjectName(QStringLiteral("hintText"));
    m_projectLabel->setWordWrap(true);
    m_browse = new QPushButton(tr("Choose folder…"));
    m_browse->setObjectName(QStringLiteral("primaryButton"));
    m_refresh = new QPushButton(tr("Refresh apps"));
    m_refresh->setObjectName(QStringLiteral("secondaryButton"));
    m_refresh->setToolTip(tr("Reload the app list from disk (new workflows, category changes)."));
    m_setupMcp = new QPushButton(tr("Set up Cursor MCP"));
    m_setupMcp->setObjectName(QStringLiteral("secondaryButton"));
    m_setupMcp->setToolTip(tr(
        "Run cursor-prep.sh only — writes bin/.dockpipe/packages/cursor-dev/ (AGENT-MCP.md, mcp.json.example). "
        "Does not run dockpipe or start Docker. To get a session container + Cursor, double-click the "
        "cursor-dev app in the list above."));
    connect(m_browse, &QPushButton::clicked, this, &BasicModeWidget::onBrowse);
    connect(m_refresh, &QPushButton::clicked, this, &BasicModeWidget::onRefresh);
    connect(m_setupMcp, &QPushButton::clicked, this, &BasicModeWidget::setupMcpRequested);
    projRow->addWidget(m_projectLabel, 1);
    projRow->addWidget(m_setupMcp, 0, Qt::AlignRight);
    projRow->addWidget(m_refresh, 0, Qt::AlignRight);
    projRow->addWidget(m_browse, 0, Qt::AlignRight);

    m_list = new QListWidget(m_workspacePage);
    m_list->setObjectName(QStringLiteral("basicAppList"));
    m_list->setMovement(QListWidget::Static);
    m_list->setResizeMode(QListWidget::Adjust);
    m_list->setSpacing(8);
    m_list->setWordWrap(true);
    auto launchItem = [this](QListWidgetItem *it) {
        if (!it)
            return;
        const QString id = it->data(Qt::UserRole).toString();
        if (!id.isEmpty())
            emit launchRequested(id);
    };
    connect(m_list, &QListWidget::itemClicked, this, launchItem);
    connect(m_list, &QListWidget::itemDoubleClicked, this, launchItem);
    connect(m_list, &QListWidget::itemActivated, this, launchItem);

    root->addWidget(title);
    root->addWidget(sub);
    root->addLayout(projRow);
    root->addWidget(m_list, 1);

    m_stack->addWidget(m_homePage);
    m_stack->addWidget(m_workspacePage);

    applyViewMode();
}

void BasicModeWidget::showHomePage()
{
    m_stack->setCurrentWidget(m_homePage);
}

void BasicModeWidget::showWorkspacePage()
{
    m_stack->setCurrentWidget(m_workspacePage);
}

void BasicModeWidget::setRecentProjects(const QStringList &paths)
{
    m_recentPaths = paths;
    rebuildRecentList();
}

void BasicModeWidget::setContinueLastVisible(bool visible)
{
    m_continueLast->setVisible(visible);
}

void BasicModeWidget::rebuildRecentList()
{
    m_recentList->clear();
    for (const QString &p : m_recentPaths) {
        if (p.isEmpty())
            continue;
        const QFileInfo fi(p);
        auto *it = new QListWidgetItem;
        it->setText(fi.isDir() ? fi.fileName() : fi.filePath());
        it->setData(Qt::UserRole, QDir::cleanPath(p));
        it->setToolTip(QDir::toNativeSeparators(QDir::cleanPath(p)));
        m_recentList->addItem(it);
    }
    const bool empty = m_recentList->count() == 0;
    m_homeEmptyHint->setVisible(empty);
    m_recentList->setVisible(!empty);
    if (!empty) {
        const int n = m_recentList->count();
        const int rowH = 36;
        const int chrome = 8;
        const int maxVisible = 6;
        const int h = qMin(n, maxVisible) * rowH + chrome;
        m_recentList->setMinimumHeight(h);
        m_recentList->setMaximumHeight(h);
    } else {
        m_recentList->setMinimumHeight(0);
        m_recentList->setMaximumHeight(QWIDGETSIZE_MAX);
    }
}

void BasicModeWidget::setProjectFolder(const QString &absPath)
{
    if (absPath.isEmpty()) {
        m_projectLabel->setText(tr("No project folder — use File → Open project folder or Choose folder…"));
        return;
    }
    m_projectLabel->setText(tr("Project: %1").arg(QDir::toNativeSeparators(absPath)));
}

void BasicModeWidget::applyViewMode()
{
    if (m_iconMode) {
        m_list->setViewMode(QListWidget::IconMode);
        m_list->setIconSize(QSize(56, 56));
        m_list->setGridSize(QSize(128, 148));
    } else {
        m_list->setViewMode(QListWidget::ListMode);
        m_list->setIconSize(QSize(28, 28));
    }
}

void BasicModeWidget::setViewIconMode(bool icons)
{
    if (m_iconMode == icons)
        return;
    m_iconMode = icons;
    applyViewMode();
    rebuildItemTexts();
}

void BasicModeWidget::setApps(const QVector<WorkflowMeta> &apps)
{
    m_apps = apps;
    m_list->clear();
    for (const WorkflowMeta &m : apps) {
        auto *it = new QListWidgetItem;
        it->setIcon(appIconForWorkflow(m.workflowId));
        it->setData(Qt::UserRole, m.workflowId);
        it->setToolTip(m.description.isEmpty() ? m.displayName : m.description);
        m_list->addItem(it);
    }
    rebuildItemTexts();
}

void BasicModeWidget::rebuildItemTexts()
{
    for (int i = 0; i < m_list->count() && i < m_apps.size(); ++i) {
        const WorkflowMeta &m = m_apps[i];
        QListWidgetItem *it = m_list->item(i);
        const bool run = m_running.value(m.workflowId, false);
        QString t = m.displayName;
        if (run)
            t += tr(" — Running");
        it->setText(t);
        if (!m_iconMode) {
            QString sub = m.description;
            if (sub.length() > 120)
                sub = sub.left(117) + QStringLiteral("…");
            it->setText(m.displayName + QStringLiteral("\n") + sub + (run ? tr("\n(Running)") : QString()));
        }
    }
}

void BasicModeWidget::setRunningByWorkflow(const QHash<QString, bool> &running)
{
    m_running = running;
    rebuildItemTexts();
}

void BasicModeWidget::onBrowse()
{
    emit openProjectRequested();
}

void BasicModeWidget::onRefresh()
{
    emit refreshAppsRequested();
}
