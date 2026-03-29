#include "FlathubSearchDialog.h"

#include <QComboBox>
#include <QListWidgetItem>
#include <QDialogButtonBox>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QVBoxLayout>

namespace {

void addCategory(QComboBox *cb, const QString &label, const QString &filterValue)
{
    cb->addItem(label, filterValue);
}

} // namespace

FlathubSearchDialog::FlathubSearchDialog(QWidget *parent) : QDialog(parent)
{
    setWindowTitle(tr("Flathub search"));
    resize(580, 440);

    m_client = new FlathubClient(this);

    auto *root = new QVBoxLayout(this);
    root->setSpacing(10);

    auto *hint = new QLabel(tr("Search Flathub (public API). Pick an app to set FLATHUB_APP_ID for dockpipe — use with scripts/flathub-host/flathub-host-run.sh (.staging/packages/dockpipe/bundles/flathub-host/)."));
    hint->setWordWrap(true);
    hint->setObjectName(QStringLiteral("appSubtitle"));
    root->addWidget(hint);

    auto *row = new QHBoxLayout;
    m_query = new QLineEdit;
    m_query->setPlaceholderText(tr("Search query (e.g. steam, editor)"));
    m_searchBtn = new QPushButton(tr("Search"));
    m_searchBtn->setObjectName(QStringLiteral("primaryButton"));
    row->addWidget(m_query, 1);
    row->addWidget(m_searchBtn);
    root->addLayout(row);

    m_category = new QComboBox;
    addCategory(m_category, tr("Category: all"), QString());
    addCategory(m_category, tr("Game"), QStringLiteral("game"));
    addCategory(m_category, tr("Utility"), QStringLiteral("utility"));
    addCategory(m_category, tr("Audio / video"), QStringLiteral("audiovideo"));
    addCategory(m_category, tr("Network"), QStringLiteral("network"));
    addCategory(m_category, tr("Development"), QStringLiteral("development"));
    root->addWidget(m_category);

    m_list = new QListWidget;
    m_list->setAlternatingRowColors(true);
    root->addWidget(m_list, 1);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setObjectName(QStringLiteral("primaryButton"));
    buttons->button(QDialogButtonBox::Cancel)->setObjectName(QStringLiteral("secondaryButton"));
    root->addWidget(buttons);

    connect(m_searchBtn, &QPushButton::clicked, this, &FlathubSearchDialog::runSearch);
    connect(m_query, &QLineEdit::returnPressed, this, &FlathubSearchDialog::runSearch);
    connect(m_client, &FlathubClient::searchFinished, this, &FlathubSearchDialog::onHits);
    connect(m_client, &FlathubClient::searchFailed, this, &FlathubSearchDialog::onFailed);
    connect(m_list, &QListWidget::currentItemChanged, this, [this](QListWidgetItem *cur) {
        m_selectedAppId = cur ? cur->data(Qt::UserRole).toString() : QString();
    });
    connect(m_list, &QListWidget::itemDoubleClicked, this, [this](QListWidgetItem *it) {
        if (it) {
            m_selectedAppId = it->data(Qt::UserRole).toString();
            accept();
        }
    });
    connect(buttons, &QDialogButtonBox::accepted, this, [this]() {
        if (m_selectedAppId.isEmpty()) {
            QMessageBox::information(this, tr("Flathub"), tr("Select an app from the list."));
            return;
        }
        accept();
    });
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
}

QString FlathubSearchDialog::selectedAppId() const
{
    return m_selectedAppId;
}

void FlathubSearchDialog::runSearch()
{
    m_list->clear();
    m_selectedAppId.clear();
    const QString q = m_query->text().trimmed();
    if (q.isEmpty()) {
        QMessageBox::information(this, tr("Flathub"), tr("Enter a search query."));
        return;
    }
    const QString cat = m_category->currentData().toString();
    m_client->search(q, cat);
}

void FlathubSearchDialog::onHits(const QVector<FlathubHit> &hits)
{
    m_list->clear();
    for (const FlathubHit &h : hits) {
        const QString title = h.name + QStringLiteral(" — ") + h.appId;
        auto *it = new QListWidgetItem(title);
        it->setData(Qt::UserRole, h.appId);
        it->setToolTip(h.summary.isEmpty() ? h.appId : h.summary);
        m_list->addItem(it);
    }
    if (hits.isEmpty())
        QMessageBox::information(this, tr("Flathub"), tr("No results (try another query or category)."));
}

void FlathubSearchDialog::onFailed(const QString &message)
{
    QMessageBox::warning(this, tr("Flathub"), tr("Search failed: %1").arg(message));
}
