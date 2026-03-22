#pragma once

#include <QObject>
#include <QString>
#include <QVector>

struct FlathubHit {
    QString appId;
    QString name;
    QString summary;
    QString iconUrl;
    QString mainCategories;
};

class QNetworkAccessManager;

/// POST https://flathub.org/api/v2/search — metadata stays in the launcher, not dockpipe core.
class FlathubClient : public QObject {
    Q_OBJECT
public:
    explicit FlathubClient(QObject *parent = nullptr);

    /// categoryFilter: empty = all; else hits must contain this substring in main_categories (case-insensitive).
    void search(const QString &query, const QString &categoryFilter);

signals:
    void searchFinished(const QVector<FlathubHit> &hits);
    void searchFailed(const QString &message);

private:
    QNetworkAccessManager *m_nam = nullptr;
};
