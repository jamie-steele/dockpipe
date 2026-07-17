#include "DockerObservabilityWidget.h"

#include <QHeaderView>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QMenu>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QScrollBar>
#include <QShowEvent>
#include <QSignalBlocker>
#include <QSplitter>
#include <QStyle>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QTextCursor>
#include <QTimer>
#include <QSet>
#include <QVBoxLayout>
#include <QFrame>
#include <QtConcurrent>

namespace {

QTableWidgetItem *roItem(const QString &text, const QString &id = QString())
{
    auto *item = new QTableWidgetItem(text);
    item->setFlags(item->flags() & ~Qt::ItemIsEditable);
    if (!id.isEmpty())
        item->setData(Qt::UserRole, id);
    return item;
}

QString prettyJson(const QString &text)
{
    const QJsonDocument doc = QJsonDocument::fromJson(text.toUtf8());
    if (doc.isNull())
        return text;
    return QString::fromUtf8(doc.toJson(QJsonDocument::Indented));
}

QString firstIdFromInspect(const QString &inspectJson)
{
    const QJsonDocument doc = QJsonDocument::fromJson(inspectJson.toUtf8());
    if (!doc.isArray() || doc.array().isEmpty() || !doc.array().first().isObject())
        return {};
    return doc.array().first().toObject().value(QStringLiteral("Id")).toString();
}

QLabel *makeMetricPill(const QString &title)
{
    auto *label = new QLabel(title);
    label->setObjectName(QStringLiteral("dockerMetric"));
    label->setAlignment(Qt::AlignCenter);
    return label;
}

QTableWidgetItem *statusItem(const QString &text)
{
    auto *item = roItem(text);
    const QString lower = text.toLower();
    if (lower.contains(QStringLiteral("healthy")))
        item->setData(Qt::UserRole + 1, QStringLiteral("healthy"));
    else if (lower.contains(QStringLiteral("running")))
        item->setData(Qt::UserRole + 1, QStringLiteral("running"));
    else
        item->setData(Qt::UserRole + 1, QStringLiteral("other"));
    return item;
}

QString normalizeContainerState(QString state, const QString &statusText)
{
    state = state.trimmed().toLower();
    const QString lowerStatus = statusText.toLower();
    if (lowerStatus.contains(QStringLiteral("healthy")))
        return QStringLiteral("healthy");
    if (state == QStringLiteral("running"))
        return QStringLiteral("running");
    if (state == QStringLiteral("paused"))
        return QStringLiteral("paused");
    if (state == QStringLiteral("restarting"))
        return QStringLiteral("restarting");
    if (state == QStringLiteral("exited"))
        return QStringLiteral("exited");
    if (state == QStringLiteral("created"))
        return QStringLiteral("created");
    return QStringLiteral("other");
}

QLabel *makeStatusPill(const QString &text, const QString &state, QWidget *parent)
{
    auto *label = new QLabel(text, parent);
    label->setObjectName(QStringLiteral("dockerStatusPill"));
    label->setProperty("state", state);
    label->setAlignment(Qt::AlignCenter);
    return label;
}

QIcon statusIconForState(const QString &state, QWidget *widget)
{
    const QStyle *style = widget ? widget->style() : nullptr;
    if (!style)
        return {};
    if (state == QStringLiteral("healthy") || state == QStringLiteral("running"))
        return style->standardIcon(QStyle::SP_DialogApplyButton);
    if (state == QStringLiteral("paused"))
        return style->standardIcon(QStyle::SP_MediaPause);
    if (state == QStringLiteral("restarting"))
        return style->standardIcon(QStyle::SP_BrowserReload);
    if (state == QStringLiteral("exited") || state == QStringLiteral("created"))
        return style->standardIcon(QStyle::SP_MediaStop);
    return style->standardIcon(QStyle::SP_MessageBoxInformation);
}

int rowForId(QTableWidget *table, const QString &id)
{
    if (!table || id.isEmpty())
        return -1;
    for (int row = 0; row < table->rowCount(); ++row) {
        auto *item = table->item(row, 0);
        if (item && item->data(Qt::UserRole).toString() == id)
            return row;
    }
    return -1;
}

void removeRowsNotIn(QTableWidget *table, const QSet<QString> &seenIds)
{
    if (!table)
        return;
    for (int row = table->rowCount() - 1; row >= 0; --row) {
        auto *item = table->item(row, 0);
        const QString id = item ? item->data(Qt::UserRole).toString() : QString();
        if (id.isEmpty() || !seenIds.contains(id))
            table->removeRow(row);
    }
}

void setItemText(QTableWidget *table, int row, int column, const QString &text, const QString &id = QString())
{
    auto *item = table->item(row, column);
    if (!item) {
        table->setItem(row, column, roItem(text, id));
        return;
    }
    item->setText(text);
    if (!id.isEmpty())
        item->setData(Qt::UserRole, id);
}

bool isScrolledToBottom(const QPlainTextEdit *edit)
{
    if (!edit || !edit->verticalScrollBar())
        return true;
    const QScrollBar *bar = edit->verticalScrollBar();
    return bar->value() >= bar->maximum() - 4;
}

void restoreScroll(QPlainTextEdit *edit, bool wasAtBottom, int previousValue)
{
    if (!edit || !edit->verticalScrollBar())
        return;
    QScrollBar *bar = edit->verticalScrollBar();
    if (wasAtBottom)
        bar->setValue(bar->maximum());
    else
        bar->setValue(qMin(previousValue, bar->maximum()));
}

void replacePlainTextWithoutFlash(QPlainTextEdit *edit, const QString &text)
{
    if (!edit || edit->toPlainText() == text)
        return;
    const bool wasAtBottom = isScrolledToBottom(edit);
    const int previousValue = edit->verticalScrollBar() ? edit->verticalScrollBar()->value() : 0;
    const QSignalBlocker blocker(edit);
    edit->setPlainText(text);
    restoreScroll(edit, wasAtBottom, previousValue);
}

void streamPlainTextWithoutFlash(QPlainTextEdit *edit, const QString &text)
{
    if (!edit)
        return;
    const QString current = edit->toPlainText();
    if (current == text)
        return;

    const bool wasAtBottom = isScrolledToBottom(edit);
    const int previousValue = edit->verticalScrollBar() ? edit->verticalScrollBar()->value() : 0;
    const QSignalBlocker blocker(edit);
    if (!current.isEmpty() && text.startsWith(current)) {
        edit->moveCursor(QTextCursor::End);
        edit->insertPlainText(text.mid(current.size()));
    } else {
        edit->setPlainText(text);
    }
    restoreScroll(edit, wasAtBottom, previousValue);
}

} // namespace

DockerObservabilityWidget::DockerObservabilityWidget(QWidget *parent) : QWidget(parent)
{
    m_refreshWatcher = new QFutureWatcher<DockerSnapshot>(this);
    connect(m_refreshWatcher, &QFutureWatcher<DockerSnapshot>::finished, this, [this]() {
        const bool quiet = m_currentRefreshQuiet;
        m_currentRefreshQuiet = false;
        applySnapshot(m_refreshWatcher->result());
        m_hasLoadedOnce = true;
        if (m_refreshPending) {
            const bool pendingQuiet = m_refreshPendingQuiet;
            m_refreshPending = false;
            m_refreshPendingQuiet = false;
            if (pendingQuiet)
                QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refreshQuietly, Qt::QueuedConnection);
            else
                QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        } else if (quiet) {
            setLoadingState(false);
        }
    });

    m_containerDetailWatcher = new QFutureWatcher<ContainerDetailSnapshot>(this);
    connect(m_containerDetailWatcher, &QFutureWatcher<ContainerDetailSnapshot>::finished, this, [this]() {
        const ContainerDetailSnapshot snapshot = m_containerDetailWatcher->result();
        if (snapshot.containerId != m_pendingContainerDetailId)
            return;
        m_displayedContainerDetailId = snapshot.containerId;
        if (m_containerDetails)
            replacePlainTextWithoutFlash(m_containerDetails, snapshot.inspectText);
        if (m_containerLogs)
            streamPlainTextWithoutFlash(m_containerLogs, snapshot.logsText);
    });

    m_networkDetailWatcher = new QFutureWatcher<ObjectDetailSnapshot>(this);
    connect(m_networkDetailWatcher, &QFutureWatcher<ObjectDetailSnapshot>::finished, this, [this]() {
        const ObjectDetailSnapshot snapshot = m_networkDetailWatcher->result();
        if (snapshot.objectId != m_pendingNetworkDetailId)
            return;
        if (m_networkDetails)
            m_networkDetails->setPlainText(snapshot.detailText);
    });

    m_volumeDetailWatcher = new QFutureWatcher<ObjectDetailSnapshot>(this);
    connect(m_volumeDetailWatcher, &QFutureWatcher<ObjectDetailSnapshot>::finished, this, [this]() {
        const ObjectDetailSnapshot snapshot = m_volumeDetailWatcher->result();
        if (snapshot.objectId != m_pendingVolumeDetailId)
            return;
        if (m_volumeDetails)
            m_volumeDetails->setPlainText(snapshot.detailText);
    });

    m_autoRefreshTimer = new QTimer(this);
    m_autoRefreshTimer->setInterval(4000);
    connect(m_autoRefreshTimer, &QTimer::timeout, this, &DockerObservabilityWidget::refreshQuietly);

    buildUi();
}

void DockerObservabilityWidget::buildUi()
{
    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(12);

    auto *hero = new QFrame(this);
    hero->setObjectName(QStringLiteral("dockerHero"));
    auto *heroLay = new QVBoxLayout(hero);
    heroLay->setContentsMargins(14, 14, 14, 14);
    heroLay->setSpacing(10);

    auto *title = new QLabel(tr("Docker observability"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *subtitle = new QLabel(tr("Inspect containers, logs, bindings, networks, and volumes. Right-click a container for quick actions."));
    subtitle->setObjectName(QStringLiteral("appSubtitle"));
    subtitle->setWordWrap(true);
    heroLay->addWidget(title);
    heroLay->addWidget(subtitle);

    auto *topRow = new QHBoxLayout;
    m_status = new QLabel(tr("Docker observability opens cold. Right-click a container row to start or stop it."));
    m_status->setWordWrap(true);
    m_search = new QLineEdit(this);
    m_search->setObjectName(QStringLiteral("surfaceSearch"));
    m_search->setClearButtonEnabled(true);
    m_search->setPlaceholderText(tr("Search containers…"));
    connect(m_search, &QLineEdit::textChanged, this, &DockerObservabilityWidget::onContainerSearchChanged);
    auto *refreshButton = new QPushButton(tr("Refresh"));
    refreshButton->setObjectName(QStringLiteral("secondaryButton"));
    connect(refreshButton, &QPushButton::clicked, this, &DockerObservabilityWidget::refresh);
    topRow->addWidget(m_status, 1);
    topRow->addWidget(m_search);
    topRow->addWidget(refreshButton);
    heroLay->addLayout(topRow);

    auto *metrics = new QHBoxLayout;
    metrics->setSpacing(8);
    m_containerCount = makeMetricPill(tr("Containers 0"));
    m_networkCount = makeMetricPill(tr("Networks 0"));
    m_volumeCount = makeMetricPill(tr("Volumes 0"));
    metrics->addWidget(m_containerCount);
    metrics->addWidget(m_networkCount);
    metrics->addWidget(m_volumeCount);
    metrics->addStretch(1);
    heroLay->addLayout(metrics);

    outer->addWidget(hero);

    m_tabs = new QTabWidget(this);
    m_tabs->setObjectName(QStringLiteral("surfaceTabs"));
    m_tabs->addTab(buildContainersPage(), tr("Containers"));
    m_tabs->addTab(buildNetworksPage(), tr("Networks"));
    m_tabs->addTab(buildVolumesPage(), tr("Volumes"));
    outer->addWidget(m_tabs, 1);
}

QWidget *DockerObservabilityWidget::buildContainersPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_containers = new QTableWidget(splitter);
    m_containers->setObjectName(QStringLiteral("dockerTable"));
    m_containers->setColumnCount(5);
    m_containers->setHorizontalHeaderLabels({tr("Name"), tr("State"), tr("Image"), tr("Ports"), tr("Created")});
    m_containers->horizontalHeader()->setStretchLastSection(false);
    m_containers->horizontalHeader()->setSectionResizeMode(0, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(1, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(2, QHeaderView::Stretch);
    m_containers->horizontalHeader()->setSectionResizeMode(3, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(4, QHeaderView::ResizeToContents);
    m_containers->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_containers->setSelectionMode(QAbstractItemView::SingleSelection);
    m_containers->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_containers->setAlternatingRowColors(true);
    m_containers->setShowGrid(false);
    m_containers->verticalHeader()->setVisible(false);
    m_containers->verticalHeader()->setDefaultSectionSize(36);
    m_containers->setContextMenuPolicy(Qt::CustomContextMenu);

    auto *detailSplitter = new QSplitter(Qt::Horizontal, splitter);
    detailSplitter->setChildrenCollapsible(false);
    m_containerDetails = new QPlainTextEdit(detailSplitter);
    m_containerDetails->setObjectName(QStringLiteral("detailConsole"));
    m_containerDetails->setReadOnly(true);
    m_containerDetails->setPlaceholderText(tr("Select a container to inspect bindings, mounts, networks, and config."));
    m_containerLogs = new QPlainTextEdit(detailSplitter);
    m_containerLogs->setObjectName(QStringLiteral("detailConsole"));
    m_containerLogs->setReadOnly(true);
    m_containerLogs->setPlaceholderText(tr("Recent docker logs appear here."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_containers, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onContainersSelectionChanged);
    connect(m_containers, &QWidget::customContextMenuRequested, this,
            &DockerObservabilityWidget::onContainersContextMenuRequested);
    return page;
}

QWidget *DockerObservabilityWidget::buildNetworksPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_networks = new QTableWidget(splitter);
    m_networks->setObjectName(QStringLiteral("dockerTable"));
    m_networks->setColumnCount(3);
    m_networks->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Scope")});
    m_networks->horizontalHeader()->setStretchLastSection(true);
    m_networks->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_networks->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_networks->setSelectionMode(QAbstractItemView::SingleSelection);
    m_networks->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_networks->setAlternatingRowColors(true);
    m_networks->setShowGrid(false);
    m_networks->verticalHeader()->setVisible(false);

    m_networkDetails = new QPlainTextEdit(splitter);
    m_networkDetails->setObjectName(QStringLiteral("detailConsole"));
    m_networkDetails->setReadOnly(true);
    m_networkDetails->setPlaceholderText(tr("Select a network to inspect attached containers and configuration."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_networks, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onNetworksSelectionChanged);
    return page;
}

QWidget *DockerObservabilityWidget::buildVolumesPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_volumes = new QTableWidget(splitter);
    m_volumes->setObjectName(QStringLiteral("dockerTable"));
    m_volumes->setColumnCount(3);
    m_volumes->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Mountpoint")});
    m_volumes->horizontalHeader()->setStretchLastSection(true);
    m_volumes->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_volumes->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_volumes->setSelectionMode(QAbstractItemView::SingleSelection);
    m_volumes->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_volumes->setAlternatingRowColors(true);
    m_volumes->setShowGrid(false);
    m_volumes->verticalHeader()->setVisible(false);

    m_volumeDetails = new QPlainTextEdit(splitter);
    m_volumeDetails->setObjectName(QStringLiteral("detailConsole"));
    m_volumeDetails->setReadOnly(true);
    m_volumeDetails->setPlaceholderText(tr("Select a volume to inspect usage details."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_volumes, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onVolumesSelectionChanged);
    return page;
}

DockerObservabilityWidget::CommandResult DockerObservabilityWidget::runDocker(const QStringList &args)
{
    QProcess proc;
    proc.start(QStringLiteral("docker"), args);
    proc.waitForFinished(15000);

    CommandResult result;
    result.ok = proc.exitStatus() == QProcess::NormalExit && proc.exitCode() == 0;
    result.stdoutText = QString::fromLocal8Bit(proc.readAllStandardOutput());
    result.stderrText = QString::fromLocal8Bit(proc.readAllStandardError());
    if (!result.ok && result.stderrText.trimmed().isEmpty())
        result.stderrText = proc.errorString();
    return result;
}

DockerObservabilityWidget::DockerSnapshot DockerObservabilityWidget::collectDockerSnapshot()
{
    DockerSnapshot snapshot;

    const auto containersResult = runDocker(
        {QStringLiteral("container"), QStringLiteral("ls"), QStringLiteral("--all"), QStringLiteral("--format"),
         QStringLiteral("{{json .}}")});
    snapshot.containersOk = containersResult.ok;
    if (!containersResult.ok) {
        snapshot.containersError = containersResult.stderrText.trimmed();
    } else {
        const QStringList lines = containersResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            ContainerRowData row;
            row.id = o.value(QStringLiteral("ID")).toString();
            row.name = o.value(QStringLiteral("Names")).toString();
            row.statusText = o.value(QStringLiteral("Status")).toString();
            row.state = normalizeContainerState(o.value(QStringLiteral("State")).toString(), row.statusText);
            row.image = o.value(QStringLiteral("Image")).toString();
            row.ports = o.value(QStringLiteral("Ports")).toString();
            row.created = o.value(QStringLiteral("RunningFor")).toString();
            snapshot.containers.append(row);
        }
    }

    const auto networksResult = runDocker(
        {QStringLiteral("network"), QStringLiteral("ls"), QStringLiteral("--format"), QStringLiteral("{{json .}}")});
    snapshot.networksOk = networksResult.ok;
    if (!networksResult.ok) {
        snapshot.networksError = networksResult.stderrText.trimmed();
    } else {
        const QStringList lines = networksResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            NetworkRowData row;
            row.id = o.value(QStringLiteral("ID")).toString();
            row.name = o.value(QStringLiteral("Name")).toString();
            row.driver = o.value(QStringLiteral("Driver")).toString();
            row.scope = o.value(QStringLiteral("Scope")).toString();
            snapshot.networks.append(row);
        }
    }

    const auto volumesResult = runDocker(
        {QStringLiteral("volume"), QStringLiteral("ls"), QStringLiteral("--format"), QStringLiteral("{{json .}}")});
    snapshot.volumesOk = volumesResult.ok;
    if (!volumesResult.ok) {
        snapshot.volumesError = volumesResult.stderrText.trimmed();
    } else {
        const QStringList lines = volumesResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            VolumeRowData row;
            row.name = o.value(QStringLiteral("Name")).toString();
            row.driver = o.value(QStringLiteral("Driver")).toString();
            const auto inspect = runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), row.name});
            if (inspect.ok) {
                const QJsonDocument inspectDoc = QJsonDocument::fromJson(inspect.stdoutText.toUtf8());
                if (inspectDoc.isArray() && !inspectDoc.array().isEmpty() && inspectDoc.array().first().isObject())
                    row.mountpoint = inspectDoc.array().first().toObject().value(QStringLiteral("Mountpoint")).toString();
            }
            snapshot.volumes.append(row);
        }
    }

    return snapshot;
}

void DockerObservabilityWidget::setStatus(const QString &text)
{
    if (m_status)
        m_status->setText(text);
}

void DockerObservabilityWidget::refresh()
{
    m_currentRefreshQuiet = false;
    if (m_refreshWatcher && m_refreshWatcher->isRunning()) {
        m_refreshPending = true;
        m_refreshPendingQuiet = false;
        setLoadingState(true, tr("Loading Docker objects…"));
        return;
    }

    setLoadingState(true, tr("Loading Docker objects…"));
    if (m_refreshWatcher)
        m_refreshWatcher->setFuture(QtConcurrent::run([]() { return DockerObservabilityWidget::collectDockerSnapshot(); }));
}

void DockerObservabilityWidget::refreshQuietly()
{
    if (!m_active)
        return;
    if (m_refreshWatcher && m_refreshWatcher->isRunning()) {
        if (!m_refreshPending) {
            m_refreshPending = true;
            m_refreshPendingQuiet = true;
        }
        return;
    }

    m_currentRefreshQuiet = true;
    if (!hasAnyObjects())
        setLoadingState(true, tr("Loading Docker objects…"));
    if (m_refreshWatcher)
        m_refreshWatcher->setFuture(QtConcurrent::run([]() { return DockerObservabilityWidget::collectDockerSnapshot(); }));
}

void DockerObservabilityWidget::setActive(bool active)
{
    m_active = active;
    if (active) {
        if (m_autoRefreshTimer && !m_autoRefreshTimer->isActive())
            m_autoRefreshTimer->start();
        if (!m_hasLoadedOnce)
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        else
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refreshQuietly, Qt::QueuedConnection);
        return;
    }

    if (m_autoRefreshTimer)
        m_autoRefreshTimer->stop();
    if (m_containerDetails)
        m_containerDetails->clear();
    if (m_containerLogs)
        m_containerLogs->clear();
    if (m_networkDetails)
        m_networkDetails->clear();
    if (m_volumeDetails)
        m_volumeDetails->clear();
    updateContainerActionState();
    m_pendingContainerDetailId.clear();
    m_displayedContainerDetailId.clear();
    m_pendingNetworkDetailId.clear();
    m_pendingVolumeDetailId.clear();
    m_refreshPending = false;
    m_refreshPendingQuiet = false;
    m_currentRefreshQuiet = false;
    setLoadingState(false);
    setStatus(tr("Docker observability opens cold and refreshes on demand."));
}

void DockerObservabilityWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (isVisible() && m_active && m_autoRefreshTimer && !m_autoRefreshTimer->isActive())
        m_autoRefreshTimer->start();
    if (!m_hasLoadedOnce && isVisible() && m_active)
        QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
}

void DockerObservabilityWidget::updateSummary()
{
    if (m_containerCount)
        m_containerCount->setText(tr("Containers %1").arg(m_containers ? m_containers->rowCount() : 0));
    if (m_networkCount)
        m_networkCount->setText(tr("Networks %1").arg(m_networks ? m_networks->rowCount() : 0));
    if (m_volumeCount)
        m_volumeCount->setText(tr("Volumes %1").arg(m_volumes ? m_volumes->rowCount() : 0));
}

void DockerObservabilityWidget::applyContainerFilter()
{
    if (!m_containers)
        return;
    const QString needle = m_search ? m_search->text().trimmed().toCaseFolded() : QString();
    for (int row = 0; row < m_containers->rowCount(); ++row) {
        QStringList values;
        for (int col = 0; col < m_containers->columnCount(); ++col) {
            if (auto *item = m_containers->item(row, col))
                values << item->text();
        }
        const bool hidden = !needle.isEmpty() && !values.join(QLatin1Char(' ')).toCaseFolded().contains(needle);
        m_containers->setRowHidden(row, hidden);
    }
    updateContainerActionState();
}

void DockerObservabilityWidget::updateContainerActionState()
{
    const QString state = selectedContainerState();
    Q_UNUSED(state);
}

bool DockerObservabilityWidget::hasAnyObjects() const
{
    const int containerCount = m_containers ? m_containers->rowCount() : 0;
    const int networkCount = m_networks ? m_networks->rowCount() : 0;
    const int volumeCount = m_volumes ? m_volumes->rowCount() : 0;
    return (containerCount + networkCount + volumeCount) > 0;
}

void DockerObservabilityWidget::setLoadingState(bool loading, const QString &text)
{
    m_loading = loading;
    if (!loading)
        return;
    if (!text.isEmpty())
        setStatus(text);
    if (!hasAnyObjects()) {
        if (m_containerDetails)
            m_containerDetails->setPlainText(tr("Loading Docker containers…"));
        if (m_containerLogs)
            m_containerLogs->setPlainText(tr("Loading container logs…"));
        if (m_networkDetails)
            m_networkDetails->setPlainText(tr("Loading Docker networks…"));
        if (m_volumeDetails)
            m_volumeDetails->setPlainText(tr("Loading Docker volumes…"));
    }
}

QString DockerObservabilityWidget::selectedContainerId() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

QString DockerObservabilityWidget::selectedContainerName() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->text().trimmed();
}

QString DockerObservabilityWidget::selectedContainerState() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole + 1).toString();
}

QString DockerObservabilityWidget::selectedNetworkId() const
{
    if (!m_networks)
        return {};
    const auto selected = m_networks->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

QString DockerObservabilityWidget::selectedVolumeName() const
{
    if (!m_volumes)
        return {};
    const auto selected = m_volumes->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

void DockerObservabilityWidget::applySnapshot(const DockerSnapshot &snapshot)
{
    const QString previousContainerId = selectedContainerId();
    const QString previousNetworkId = selectedNetworkId();
    const QString previousVolumeName = selectedVolumeName();
    int restoredContainerRow = -1;
    int restoredNetworkRow = -1;
    int restoredVolumeRow = -1;

    const QSignalBlocker blockContainers(m_containers);
    const QSignalBlocker blockNetworks(m_networks);
    const QSignalBlocker blockVolumes(m_volumes);

    if (!snapshot.containersOk) {
        replacePlainTextWithoutFlash(m_containerDetails, tr("Could not query docker containers.\n\n%1").arg(snapshot.containersError));
    } else {
        QSet<QString> seenIds;
        for (const ContainerRowData &rowData : snapshot.containers) {
            const QString state = rowData.state;
            seenIds.insert(rowData.id);
            int row = rowForId(m_containers, rowData.id);
            if (row < 0) {
                row = m_containers->rowCount();
                m_containers->insertRow(row);
            }
            setItemText(m_containers, row, 0, rowData.name, rowData.id);
            auto *nameItem = m_containers->item(row, 0);
            nameItem->setData(Qt::UserRole + 1, state);
            nameItem->setIcon(statusIconForState(state, m_containers));

            auto *stateItem = m_containers->item(row, 1);
            if (!stateItem) {
                stateItem = statusItem(QString());
                m_containers->setItem(row, 1, stateItem);
            }
            stateItem->setData(Qt::UserRole + 1, state);
            stateItem->setData(Qt::ToolTipRole, rowData.statusText);
            m_containers->setCellWidget(row, 1, makeStatusPill(rowData.statusText, state, m_containers));
            setItemText(m_containers, row, 2, rowData.image);
            setItemText(m_containers, row, 3, rowData.ports);
            setItemText(m_containers, row, 4, rowData.created);
            if (!previousContainerId.isEmpty() && rowData.id == previousContainerId)
                restoredContainerRow = row;
        }
        removeRowsNotIn(m_containers, seenIds);
        if (snapshot.containers.isEmpty())
            replacePlainTextWithoutFlash(m_containerDetails, tr("No Docker containers found. This view includes stopped containers too."));
    }

    if (!snapshot.networksOk) {
        replacePlainTextWithoutFlash(m_networkDetails, tr("Could not query docker networks.\n\n%1").arg(snapshot.networksError));
    } else {
        QSet<QString> seenIds;
        for (const NetworkRowData &rowData : snapshot.networks) {
            seenIds.insert(rowData.id);
            int row = rowForId(m_networks, rowData.id);
            if (row < 0) {
                row = m_networks->rowCount();
                m_networks->insertRow(row);
            }
            setItemText(m_networks, row, 0, rowData.name, rowData.id);
            setItemText(m_networks, row, 1, rowData.driver);
            setItemText(m_networks, row, 2, rowData.scope);
            if (!previousNetworkId.isEmpty() && rowData.id == previousNetworkId)
                restoredNetworkRow = row;
        }
        removeRowsNotIn(m_networks, seenIds);
    }

    if (!snapshot.volumesOk) {
        replacePlainTextWithoutFlash(m_volumeDetails, tr("Could not query docker volumes.\n\n%1").arg(snapshot.volumesError));
    } else {
        QSet<QString> seenIds;
        for (const VolumeRowData &rowData : snapshot.volumes) {
            seenIds.insert(rowData.name);
            int row = rowForId(m_volumes, rowData.name);
            if (row < 0) {
                row = m_volumes->rowCount();
                m_volumes->insertRow(row);
            }
            setItemText(m_volumes, row, 0, rowData.name, rowData.name);
            setItemText(m_volumes, row, 1, rowData.driver);
            setItemText(m_volumes, row, 2, rowData.mountpoint);
            if (!previousVolumeName.isEmpty() && rowData.name == previousVolumeName)
                restoredVolumeRow = row;
        }
        removeRowsNotIn(m_volumes, seenIds);
    }

    if (!previousContainerId.isEmpty())
        restoredContainerRow = rowForId(m_containers, previousContainerId);
    if (!previousNetworkId.isEmpty())
        restoredNetworkRow = rowForId(m_networks, previousNetworkId);
    if (!previousVolumeName.isEmpty())
        restoredVolumeRow = rowForId(m_volumes, previousVolumeName);

    if (restoredContainerRow >= 0)
        m_containers->selectRow(restoredContainerRow);
    if (restoredNetworkRow >= 0)
        m_networks->selectRow(restoredNetworkRow);
    if (restoredVolumeRow >= 0)
        m_volumes->selectRow(restoredVolumeRow);

    applyContainerFilter();
    updateSummary();
    updateContainerActionState();
    const int totalObjects = snapshot.containers.size() + snapshot.networks.size() + snapshot.volumes.size();
    if (totalObjects > 0) {
        setStatus(tr("Docker observability refreshed."));
    } else if (snapshot.containersOk && snapshot.networksOk && snapshot.volumesOk) {
        setStatus(tr("No Docker objects found."));
    } else {
        setStatus(tr("Docker refresh completed with errors."));
    }
    m_loading = false;

    if (restoredContainerRow >= 0)
        refreshContainerDetailsAsync(previousContainerId);
    if (restoredNetworkRow >= 0)
        refreshNetworkDetailsAsync(previousNetworkId);
    if (restoredVolumeRow >= 0)
        refreshVolumeDetailsAsync(previousVolumeName);
}

void DockerObservabilityWidget::renderContainerDetails(const QString &containerId)
{
    refreshContainerDetailsAsync(containerId);
}

void DockerObservabilityWidget::renderNetworkDetails(const QString &networkId)
{
    refreshNetworkDetailsAsync(networkId);
}

void DockerObservabilityWidget::renderVolumeDetails(const QString &volumeName)
{
    refreshVolumeDetailsAsync(volumeName);
}

void DockerObservabilityWidget::refreshContainerDetailsAsync(const QString &containerId)
{
    if (containerId.isEmpty())
        return;
    const bool sameVisibleContainer = containerId == m_displayedContainerDetailId;
    m_pendingContainerDetailId = containerId;
    if (!sameVisibleContainer && m_containerDetails)
        m_containerDetails->setPlainText(tr("Loading container details…"));
    if (!sameVisibleContainer && m_containerLogs)
        m_containerLogs->setPlainText(tr("Loading container logs…"));
    if (m_containerDetailWatcher) {
        m_containerDetailWatcher->setFuture(QtConcurrent::run([containerId]() {
            ContainerDetailSnapshot snapshot;
            snapshot.containerId = containerId;
            const CommandResult inspect = DockerObservabilityWidget::runDocker({QStringLiteral("inspect"), containerId});
            if (!inspect.ok) {
                snapshot.inspectText = QObject::tr("Could not inspect container.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.inspectText = prettyJson(inspect.stdoutText);
            }
            const CommandResult logs =
                DockerObservabilityWidget::runDocker({QStringLiteral("logs"), QStringLiteral("--tail"), QStringLiteral("200"), containerId});
            if (!logs.ok) {
                snapshot.logsText = QObject::tr("Could not load docker logs.\n\n%1").arg(logs.stderrText.trimmed());
            } else {
                snapshot.logsText = logs.stdoutText.trimmed();
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshNetworkDetailsAsync(const QString &networkId)
{
    if (networkId.isEmpty())
        return;
    m_pendingNetworkDetailId = networkId;
    if (m_networkDetails)
        m_networkDetails->setPlainText(tr("Loading network details…"));
    if (m_networkDetailWatcher) {
        m_networkDetailWatcher->setFuture(QtConcurrent::run([networkId]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = networkId;
            const CommandResult inspect =
                DockerObservabilityWidget::runDocker({QStringLiteral("network"), QStringLiteral("inspect"), networkId});
            if (!inspect.ok) {
                snapshot.detailText = QObject::tr("Could not inspect network.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = prettyJson(inspect.stdoutText);
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshVolumeDetailsAsync(const QString &volumeName)
{
    if (volumeName.isEmpty())
        return;
    m_pendingVolumeDetailId = volumeName;
    if (m_volumeDetails)
        m_volumeDetails->setPlainText(tr("Loading volume details…"));
    if (m_volumeDetailWatcher) {
        m_volumeDetailWatcher->setFuture(QtConcurrent::run([volumeName]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = volumeName;
            const CommandResult inspect =
                DockerObservabilityWidget::runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), volumeName});
            if (!inspect.ok) {
                snapshot.detailText = QObject::tr("Could not inspect volume.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = prettyJson(inspect.stdoutText);
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::onContainersSelectionChanged()
{
    updateContainerActionState();
    const QString containerId = selectedContainerId();
    if (containerId.isEmpty())
        return;
    renderContainerDetails(containerId);
}

void DockerObservabilityWidget::onContainersContextMenuRequested(const QPoint &pos)
{
    if (!m_containers)
        return;
    if (auto *item = m_containers->itemAt(pos))
        m_containers->selectRow(item->row());
    const QString containerId = selectedContainerId();
    if (containerId.isEmpty())
        return;

    updateContainerActionState();
    const QString state = selectedContainerState();
    const QString name = selectedContainerName();

    QMenu menu(this);
    menu.setObjectName(QStringLiteral("dockerContextMenu"));

    QAction *inspectAction = menu.addAction(style()->standardIcon(QStyle::SP_FileDialogDetailedView),
                                            tr("Inspect %1").arg(name));
    QAction *startAction = menu.addAction(style()->standardIcon(QStyle::SP_MediaPlay), tr("Start container"));
    QAction *stopAction = menu.addAction(style()->standardIcon(QStyle::SP_MediaStop), tr("Stop container"));
    menu.addSeparator();
    QAction *refreshAction = menu.addAction(style()->standardIcon(QStyle::SP_BrowserReload), tr("Refresh Docker view"));

    startAction->setEnabled(state != QStringLiteral("healthy") && state != QStringLiteral("running"));
    stopAction->setEnabled(state == QStringLiteral("healthy") || state == QStringLiteral("running")
                           || state == QStringLiteral("paused") || state == QStringLiteral("restarting"));

    QAction *chosen = menu.exec(m_containers->viewport()->mapToGlobal(pos));
    if (chosen == inspectAction) {
        renderContainerDetails(containerId);
    } else if (chosen == startAction) {
        onStartSelectedContainer();
    } else if (chosen == stopAction) {
        onStopSelectedContainer();
    } else if (chosen == refreshAction) {
        refresh();
    }
}

void DockerObservabilityWidget::onNetworksSelectionChanged()
{
    const auto selected = m_networks->selectedItems();
    if (selected.isEmpty())
        return;
    const QString networkId = selected.first()->data(Qt::UserRole).toString();
    if (!networkId.isEmpty())
        renderNetworkDetails(networkId);
}

void DockerObservabilityWidget::onVolumesSelectionChanged()
{
    const auto selected = m_volumes->selectedItems();
    if (selected.isEmpty())
        return;
    const QString volumeName = selected.first()->data(Qt::UserRole).toString();
    if (!volumeName.isEmpty())
        renderVolumeDetails(volumeName);
}

void DockerObservabilityWidget::onContainerSearchChanged(const QString &)
{
    applyContainerFilter();
}

void DockerObservabilityWidget::onStartSelectedContainer()
{
    const QString containerId = selectedContainerId();
    const QString containerName = selectedContainerName();
    if (containerId.isEmpty())
        return;

    setStatus(tr("Starting %1…").arg(containerName.isEmpty() ? containerId : containerName));
    const CommandResult result = runDocker({QStringLiteral("start"), containerId});
    if (!result.ok) {
        setStatus(tr("Could not start %1.").arg(containerName.isEmpty() ? containerId : containerName));
        m_containerLogs->setPlainText(tr("Could not start container.\n\n%1").arg(result.stderrText.trimmed()));
        return;
    }

    refresh();
    renderContainerDetails(containerId);
    setStatus(tr("Started %1.").arg(containerName.isEmpty() ? containerId : containerName));
}

void DockerObservabilityWidget::onStopSelectedContainer()
{
    const QString containerId = selectedContainerId();
    const QString containerName = selectedContainerName();
    if (containerId.isEmpty())
        return;

    setStatus(tr("Stopping %1…").arg(containerName.isEmpty() ? containerId : containerName));
    const CommandResult result = runDocker({QStringLiteral("stop"), containerId});
    if (!result.ok) {
        setStatus(tr("Could not stop %1.").arg(containerName.isEmpty() ? containerId : containerName));
        m_containerLogs->setPlainText(tr("Could not stop container.\n\n%1").arg(result.stderrText.trimmed()));
        return;
    }

    refresh();
    renderContainerDetails(containerId);
    setStatus(tr("Stopped %1.").arg(containerName.isEmpty() ? containerId : containerName));
}
