#include "DockerObservabilityWidget.h"

#include <QHeaderView>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QSplitter>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QVBoxLayout>

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

} // namespace

DockerObservabilityWidget::DockerObservabilityWidget(QWidget *parent) : QWidget(parent)
{
    buildUi();
}

void DockerObservabilityWidget::buildUi()
{
    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(10);

    auto *topRow = new QHBoxLayout;
    m_status = new QLabel(tr("Readonly Docker observability. Opens cold and refreshes on demand."));
    m_status->setWordWrap(true);
    auto *refreshButton = new QPushButton(tr("Refresh"));
    refreshButton->setObjectName(QStringLiteral("secondaryButton"));
    connect(refreshButton, &QPushButton::clicked, this, &DockerObservabilityWidget::refresh);
    topRow->addWidget(m_status, 1);
    topRow->addWidget(refreshButton);
    outer->addLayout(topRow);

    m_tabs = new QTabWidget(this);
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
    m_containers->setColumnCount(4);
    m_containers->setHorizontalHeaderLabels({tr("Name"), tr("Status"), tr("Image"), tr("Ports")});
    m_containers->horizontalHeader()->setStretchLastSection(true);
    m_containers->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_containers->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_containers->setSelectionMode(QAbstractItemView::SingleSelection);
    m_containers->setEditTriggers(QAbstractItemView::NoEditTriggers);

    auto *detailSplitter = new QSplitter(Qt::Horizontal, splitter);
    detailSplitter->setChildrenCollapsible(false);
    m_containerDetails = new QPlainTextEdit(detailSplitter);
    m_containerDetails->setReadOnly(true);
    m_containerDetails->setPlaceholderText(tr("Select a container to inspect bindings, mounts, networks, and config."));
    m_containerLogs = new QPlainTextEdit(detailSplitter);
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
    m_networks->setColumnCount(3);
    m_networks->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Scope")});
    m_networks->horizontalHeader()->setStretchLastSection(true);
    m_networks->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_networks->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_networks->setSelectionMode(QAbstractItemView::SingleSelection);
    m_networks->setEditTriggers(QAbstractItemView::NoEditTriggers);

    m_networkDetails = new QPlainTextEdit(splitter);
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
    m_volumes->setColumnCount(3);
    m_volumes->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Mountpoint")});
    m_volumes->horizontalHeader()->setStretchLastSection(true);
    m_volumes->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_volumes->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_volumes->setSelectionMode(QAbstractItemView::SingleSelection);
    m_volumes->setEditTriggers(QAbstractItemView::NoEditTriggers);

    m_volumeDetails = new QPlainTextEdit(splitter);
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
        m_containers->setItem(row, 1, roItem(o.value(QStringLiteral("Status")).toString()));
        m_containers->setItem(row, 2, roItem(o.value(QStringLiteral("Image")).toString()));
        m_containers->setItem(row, 3, roItem(o.value(QStringLiteral("Ports")).toString()));
    }
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
