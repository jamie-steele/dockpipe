#pragma once

#include "FlathubClient.h"

#include <QDialog>

class QComboBox;
class QLineEdit;
class QListWidget;
class QPushButton;

/// Search Flathub (public API) and return an app id for FLATHUB_APP_ID / dockpipe --env.
class FlathubSearchDialog : public QDialog {
    Q_OBJECT
public:
    explicit FlathubSearchDialog(QWidget *parent = nullptr);

    QString selectedAppId() const;

private slots:
    void runSearch();
    void onHits(const QVector<FlathubHit> &hits);
    void onFailed(const QString &message);

private:
    FlathubClient *m_client = nullptr;
    QLineEdit *m_query = nullptr;
    QComboBox *m_category = nullptr;
    QPushButton *m_searchBtn = nullptr;
    QListWidget *m_list = nullptr;
    QString m_selectedAppId;
};
