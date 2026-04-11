#include "DockerObservabilityWidget.h"

#include <QHeaderView>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QSplitter>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QVBoxLayout>
#include <QFrame>

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

} // namespace

DockerObservabilityWidget::DockerObservabilityWidget(QWidget *parent) : QWidget(parent)
{
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
    auto *subtitle = new QLabel(tr("Readonly containers, logs, bindings, networks, and volumes. Refreshes only when you land here."));
    subtitle->setObjectName(QStringLiteral("appSubtitle"));
    subtitle->setWordWrap(true);
    heroLay->addWidget(title);
    heroLay->addWidget(subtitle);

    auto *topRow = new QHBoxLayout;
    m_status = new QLabel(tr("Readonly Docker observability. Opens cold and refreshes on demand."));
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

void DockerObservabilityWidget::setStatus(const QString &text)
{
    if (m_status)
        m_status->setText(text);
}

void DockerObservabilityWidget::refresh()
{
    loadContainers();
    loadNetworks();
    loadVolumes();
    updateSummary();
    setStatus(tr("Readonly Docker observability refreshed."));
}

void DockerObservabilityWidget::setActive(bool active)
{
    if (active) {
        refresh();
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
    setStatus(tr("Readonly Docker observability. Opens cold and refreshes on demand."));
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
}

void DockerObservabilityWidget::loadContainers()
{
    const CommandResult result = runDocker({QStringLiteral("ps"),
                                            QStringLiteral("--format"),
                                            QStringLiteral("{{json .}}")});
    m_containers->setRowCount(0);
    if (!result.ok) {
        m_containerDetails->setPlainText(tr("Could not query docker containers.\n\n%1").arg(result.stderrText.trimmed()));
        return;
    }

    const QStringList lines = result.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
    for (const QString &line : lines) {
        const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
        if (!doc.isObject())
            continue;
        const QJsonObject o = doc.object();
        const int row = m_containers->rowCount();
        m_containers->insertRow(row);
        m_containers->setItem(row, 0, roItem(o.value(QStringLiteral("Names")).toString(),
                                             o.value(QStringLiteral("ID")).toString()));
        m_containers->setItem(row, 1, statusItem(o.value(QStringLiteral("Status")).toString()));
        m_containers->setItem(row, 2, roItem(o.value(QStringLiteral("Image")).toString()));
        m_containers->setItem(row, 3, roItem(o.value(QStringLiteral("Ports")).toString()));
        m_containers->setItem(row, 4, roItem(o.value(QStringLiteral("RunningFor")).toString()));
    }
    applyContainerFilter();
}

void DockerObservabilityWidget::loadNetworks()
{
    const CommandResult result = runDocker({QStringLiteral("network"),
                                            QStringLiteral("ls"),
                                            QStringLiteral("--format"),
                                            QStringLiteral("{{json .}}")});
    m_networks->setRowCount(0);
    if (!result.ok) {
        m_networkDetails->setPlainText(tr("Could not query docker networks.\n\n%1").arg(result.stderrText.trimmed()));
        return;
    }

    const QStringList lines = result.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
    for (const QString &line : lines) {
        const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
        if (!doc.isObject())
            continue;
        const QJsonObject o = doc.object();
        const int row = m_networks->rowCount();
        m_networks->insertRow(row);
        m_networks->setItem(row, 0, roItem(o.value(QStringLiteral("Name")).toString(),
                                           o.value(QStringLiteral("ID")).toString()));
        m_networks->setItem(row, 1, roItem(o.value(QStringLiteral("Driver")).toString()));
        m_networks->setItem(row, 2, roItem(o.value(QStringLiteral("Scope")).toString()));
    }
}

void DockerObservabilityWidget::loadVolumes()
{
    const CommandResult result = runDocker({QStringLiteral("volume"),
                                            QStringLiteral("ls"),
                                            QStringLiteral("--format"),
                                            QStringLiteral("{{json .}}")});
    m_volumes->setRowCount(0);
    if (!result.ok) {
        m_volumeDetails->setPlainText(tr("Could not query docker volumes.\n\n%1").arg(result.stderrText.trimmed()));
        return;
    }

    const QStringList lines = result.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
    for (const QString &line : lines) {
        const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
        if (!doc.isObject())
            continue;
        const QJsonObject o = doc.object();
        const QString name = o.value(QStringLiteral("Name")).toString();
        const CommandResult inspect = runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), name});
        QString mountpoint;
        if (inspect.ok) {
            const QJsonDocument inspectDoc = QJsonDocument::fromJson(inspect.stdoutText.toUtf8());
            if (inspectDoc.isArray() && !inspectDoc.array().isEmpty() && inspectDoc.array().first().isObject())
                mountpoint = inspectDoc.array().first().toObject().value(QStringLiteral("Mountpoint")).toString();
        }
        const int row = m_volumes->rowCount();
        m_volumes->insertRow(row);
        m_volumes->setItem(row, 0, roItem(name, name));
        m_volumes->setItem(row, 1, roItem(o.value(QStringLiteral("Driver")).toString()));
        m_volumes->setItem(row, 2, roItem(mountpoint));
    }
}

void DockerObservabilityWidget::renderContainerDetails(const QString &containerId)
{
    const CommandResult inspect = runDocker({QStringLiteral("inspect"), containerId});
    if (!inspect.ok) {
        m_containerDetails->setPlainText(tr("Could not inspect container.\n\n%1").arg(inspect.stderrText.trimmed()));
    } else {
        m_containerDetails->setPlainText(prettyJson(inspect.stdoutText));
    }

    const CommandResult logs = runDocker({QStringLiteral("logs"), QStringLiteral("--tail"), QStringLiteral("200"), containerId});
    if (!logs.ok) {
        m_containerLogs->setPlainText(tr("Could not load docker logs.\n\n%1").arg(logs.stderrText.trimmed()));
    } else {
        m_containerLogs->setPlainText(logs.stdoutText.trimmed());
    }
}

void DockerObservabilityWidget::renderNetworkDetails(const QString &networkId)
{
    const CommandResult inspect = runDocker({QStringLiteral("network"), QStringLiteral("inspect"), networkId});
    if (!inspect.ok) {
        m_networkDetails->setPlainText(tr("Could not inspect network.\n\n%1").arg(inspect.stderrText.trimmed()));
        return;
    }
    m_networkDetails->setPlainText(prettyJson(inspect.stdoutText));
}

void DockerObservabilityWidget::renderVolumeDetails(const QString &volumeName)
{
    const CommandResult inspect = runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), volumeName});
    if (!inspect.ok) {
        m_volumeDetails->setPlainText(tr("Could not inspect volume.\n\n%1").arg(inspect.stderrText.trimmed()));
        return;
    }
    m_volumeDetails->setPlainText(prettyJson(inspect.stdoutText));
}

void DockerObservabilityWidget::onContainersSelectionChanged()
{
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return;
    const QString containerId = selected.first()->data(Qt::UserRole).toString();
    if (!containerId.isEmpty())
        renderContainerDetails(containerId);
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
