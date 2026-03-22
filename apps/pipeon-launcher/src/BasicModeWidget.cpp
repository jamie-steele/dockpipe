#include "BasicModeWidget.h"

#include <QDir>
#include <QFileInfo>
#include <QListWidgetItem>
#include <QHBoxLayout>
#include <QLabel>
#include <QListWidget>
#include <QPushButton>
#include <QVBoxLayout>

BasicModeWidget::BasicModeWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName(QStringLiteral("basicMode"));

    auto *root = new QVBoxLayout(this);
    root->setSpacing(12);
    root->setContentsMargins(12, 12, 12, 12);

    auto *title = new QLabel(tr("Apps"));
    title->setObjectName(QStringLiteral("appTitle"));

    auto *sub = new QLabel(
        tr("Pick a project folder, then launch a tool. Your folder is passed to dockpipe as --workdir (mounted in the container)."));
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
    projRow->addWidget(m_projectLabel, 1);
    projRow->addWidget(m_refresh, 0, Qt::AlignRight);
    projRow->addWidget(m_browse, 0, Qt::AlignRight);
    connect(m_browse, &QPushButton::clicked, this, &BasicModeWidget::onBrowse);
    connect(m_refresh, &QPushButton::clicked, this, &BasicModeWidget::onRefresh);

    m_list = new QListWidget(this);
    m_list->setObjectName(QStringLiteral("basicAppList"));
    m_list->setMovement(QListWidget::Static);
    m_list->setResizeMode(QListWidget::Adjust);
    m_list->setSpacing(8);
    m_list->setWordWrap(true);
    connect(m_list, &QListWidget::itemDoubleClicked, this, [this](QListWidgetItem *it) {
        if (!it)
            return;
        const QString id = it->data(Qt::UserRole).toString();
        if (!id.isEmpty())
            emit launchRequested(id);
    });

    root->addWidget(title);
    root->addWidget(sub);
    root->addLayout(projRow);
    root->addWidget(m_list, 1);

    applyViewMode();
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
    const QIcon ico(QStringLiteral(":/icon.png"));
    for (const WorkflowMeta &m : apps) {
        auto *it = new QListWidgetItem;
        it->setIcon(ico);
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
