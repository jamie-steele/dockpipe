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
#include <QShowEvent>
#include <QSplitter>
#include <QStyle>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
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

} // namespace

DockerObservabilityWidget::DockerObservabilityWidget(QWidget *parent) : QWidget(parent)
{
    m_refreshWatcher = new QFutureWatcher<DockerSnapshot>(this);
    connect(m_refreshWatcher, &QFutureWatcher<DockerSnapshot>::finished, this, [this]() {
        applySnapshot(m_refreshWatcher->result());
        m_hasLoadedOnce = true;
        if (m_refreshPending) {
            m_refreshPending = false;
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        }
    });

    m_containerDetailWatcher = new QFutureWatcher<ContainerDetailSnapshot>(this);
    connect(m_containerDetailWatcher, &QFutureWatcher<ContainerDetailSnapshot>::finished, this, [this]() {
        const ContainerDetailSnapshot snapshot = m_containerDetailWatcher->result();
        if (snapshot.containerId != m_pendingContainerDetailId)
            return;
        if (m_containerDetails)
            m_containerDetails->setPlainText(snapshot.inspectText);
        if (m_containerLogs)
            m_containerLogs->setPlainText(snapshot.logsText);
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
    if (m_refreshWatcher && m_refreshWatcher->isRunning()) {
        m_refreshPending = true;
        setLoadingState(true, tr("Loading Docker objects…"));
        return;
    }

    setLoadingState(true, tr("Loading Docker objects…"));
    if (m_refreshWatcher)
        m_refreshWatcher->setFuture(QtConcurrent::run([]() { return DockerObservabilityWidget::collectDockerSnapshot(); }));
}

void DockerObservabilityWidget::setActive(bool active)
{
    m_active = active;
    if (active) {
        if (!m_hasLoadedOnce)
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        return;
    }

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
    m_pendingNetworkDetailId.clear();
    m_pendingVolumeDetailId.clear();
    setLoadingState(false);
    setStatus(tr("Docker observability opens cold and refreshes on demand."));
}

void DockerObservabilityWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
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

void DockerObservabilityWidget::applySnapshot(const DockerSnapshot &snapshot)
{
    m_containers->setRowCount(0);
    if (!snapshot.containersOk) {
        m_containerDetails->setPlainText(tr("Could not query docker containers.\n\n%1").arg(snapshot.containersError));
    } else if (snapshot.containers.isEmpty()) {
        m_containerDetails->setPlainText(tr("No Docker containers found. This view includes stopped containers too."));
    }
    for (const ContainerRowData &rowData : snapshot.containers) {
        const QString state = rowData.state;
        const int row = m_containers->rowCount();
        m_containers->insertRow(row);
        auto *nameItem = roItem(rowData.name, rowData.id);
        nameItem->setData(Qt::UserRole + 1, state);
        nameItem->setIcon(statusIconForState(state, m_containers));
        m_containers->setItem(row, 0, nameItem);
        auto *stateItem = statusItem(QString());
        stateItem->setData(Qt::UserRole + 1, state);
        stateItem->setData(Qt::ToolTipRole, rowData.statusText);
        m_containers->setItem(row, 1, stateItem);
        m_containers->setCellWidget(row, 1, makeStatusPill(rowData.statusText, state, m_containers));
        m_containers->setItem(row, 2, roItem(rowData.image));
        m_containers->setItem(row, 3, roItem(rowData.ports));
        m_containers->setItem(row, 4, roItem(rowData.created));
    }

    m_networks->setRowCount(0);
    if (!snapshot.networksOk) {
        m_networkDetails->setPlainText(tr("Could not query docker networks.\n\n%1").arg(snapshot.networksError));
    }
    for (const NetworkRowData &rowData : snapshot.networks) {
        const int row = m_networks->rowCount();
        m_networks->insertRow(row);
        m_networks->setItem(row, 0, roItem(rowData.name, rowData.id));
        m_networks->setItem(row, 1, roItem(rowData.driver));
        m_networks->setItem(row, 2, roItem(rowData.scope));
    }

    m_volumes->setRowCount(0);
    if (!snapshot.volumesOk) {
        m_volumeDetails->setPlainText(tr("Could not query docker volumes.\n\n%1").arg(snapshot.volumesError));
    }
    for (const VolumeRowData &rowData : snapshot.volumes) {
        const int row = m_volumes->rowCount();
        m_volumes->insertRow(row);
        m_volumes->setItem(row, 0, roItem(rowData.name, rowData.name));
        m_volumes->setItem(row, 1, roItem(rowData.driver));
        m_volumes->setItem(row, 2, roItem(rowData.mountpoint));
    }

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
    m_pendingContainerDetailId = containerId;
    if (m_containerDetails)
        m_containerDetails->setPlainText(tr("Loading container details…"));
    if (m_containerLogs)
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
