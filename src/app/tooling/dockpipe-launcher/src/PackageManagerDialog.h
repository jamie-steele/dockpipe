#pragma once

#include <QDialog>

class QLabel;
class QLineEdit;
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
    void onSearchChanged(const QString &text);

private:
    void buildUi();
    void loadPackages();
    void refreshDetails(QTableWidget *table);
    void applyFilter();

    QString m_hintWorkdir;
    QLineEdit *m_search = nullptr;
    QLabel *m_installedCount = nullptr;
    QLabel *m_marketplaceCount = nullptr;
    QTabWidget *m_tabs = nullptr;
    QTableWidget *m_installedTable = nullptr;
    QTableWidget *m_marketplaceTable = nullptr;
    QTextBrowser *m_details = nullptr;
};
