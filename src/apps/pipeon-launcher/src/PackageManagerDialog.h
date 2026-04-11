#pragma once

#include <QDialog>

class QTabWidget;
class QTableWidget;
class QTextBrowser;

class PackageManagerDialog : public QDialog {
    Q_OBJECT
public:
    explicit PackageManagerDialog(const QString &hintWorkdir, QWidget *parent = nullptr);

private slots:
    void onInstalledSelectionChanged();
    void onMarketplaceSelectionChanged();
    void onAuthoringSelectionChanged();

private:
    void buildUi();
    void loadPackages();
    void refreshDetails(QTableWidget *table);

    QString m_hintWorkdir;
    QTabWidget *m_tabs = nullptr;
    QTableWidget *m_installedTable = nullptr;
    QTableWidget *m_marketplaceTable = nullptr;
    QTableWidget *m_authoringTable = nullptr;
    QTextBrowser *m_details = nullptr;
};
