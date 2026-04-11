#pragma once

#include <QWidget>

class QLabel;
class QPlainTextEdit;
class QSplitter;
class QTabWidget;
class QTableWidget;

class DockerObservabilityWidget : public QWidget {
    Q_OBJECT
public:
    explicit DockerObservabilityWidget(QWidget *parent = nullptr);

public slots:
    void refresh();
    void setActive(bool active);

private slots:
    void onContainersSelectionChanged();
    void onNetworksSelectionChanged();
    void onVolumesSelectionChanged();

private:
    struct CommandResult {
        bool ok = false;
        QString stdoutText;
        QString stderrText;
    };

    void buildUi();
    QWidget *buildContainersPage();
    QWidget *buildNetworksPage();
    QWidget *buildVolumesPage();

    static CommandResult runDocker(const QStringList &args);
    void setStatus(const QString &text);

    void loadContainers();
    void loadNetworks();
    void loadVolumes();

    void renderContainerDetails(const QString &containerId);
    void renderNetworkDetails(const QString &networkId);
    void renderVolumeDetails(const QString &volumeName);

    QLabel *m_status = nullptr;
    QTabWidget *m_tabs = nullptr;

    QTableWidget *m_containers = nullptr;
    QPlainTextEdit *m_containerDetails = nullptr;
    QPlainTextEdit *m_containerLogs = nullptr;

    QTableWidget *m_networks = nullptr;
    QPlainTextEdit *m_networkDetails = nullptr;

    QTableWidget *m_volumes = nullptr;
    QPlainTextEdit *m_volumeDetails = nullptr;
};
