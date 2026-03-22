#include "FlathubClient.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QUrl>

FlathubClient::FlathubClient(QObject *parent) : QObject(parent), m_nam(new QNetworkAccessManager(this)) {}

void FlathubClient::search(const QString &query, const QString &categoryFilter)
{
    QNetworkRequest req(QUrl(QStringLiteral("https://flathub.org/api/v2/search")));
    req.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
    QJsonObject body;
    body[QStringLiteral("query")] = query;
    body[QStringLiteral("filters")] = QJsonArray();
    auto *reply = m_nam->post(req, QJsonDocument(body).toJson());
    connect(reply, &QNetworkReply::finished, this, [this, reply, categoryFilter]() {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            emit searchFailed(reply->errorString());
            return;
        }
        const QJsonDocument doc = QJsonDocument::fromJson(reply->readAll());
        const QJsonObject root = doc.object();
        const QJsonArray hits = root.value(QStringLiteral("hits")).toArray();
        QVector<FlathubHit> out;
        out.reserve(hits.size());
        const QString cf = categoryFilter.trimmed().toLower();
        for (const QJsonValue &hv : hits) {
            const QJsonObject o = hv.toObject();
            FlathubHit h;
            h.appId = o.value(QStringLiteral("app_id")).toString();
            h.name = o.value(QStringLiteral("name")).toString();
            h.summary = o.value(QStringLiteral("summary")).toString();
            h.iconUrl = o.value(QStringLiteral("icon")).toString();
            h.mainCategories = o.value(QStringLiteral("main_categories")).toString();
            if (h.appId.isEmpty())
                continue;
            if (!cf.isEmpty() && !h.mainCategories.toLower().contains(cf))
                continue;
            out.append(h);
        }
        emit searchFinished(out);
    });
}
