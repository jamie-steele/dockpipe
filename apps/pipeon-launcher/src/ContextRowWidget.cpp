#include "ContextRowWidget.h"

#include <QFileInfo>
#include <QStyle>
#include <QHBoxLayout>
#include <QLabel>
#include <QVBoxLayout>

namespace {

QString metaLine(const Context &c)
{
    QString w;
    if (!c.workflow.isEmpty())
        w = c.workflow;
    else if (!c.workflowFile.isEmpty())
        w = QFileInfo(c.workflowFile).fileName();
    if (!c.resolver.isEmpty()) {
        if (!w.isEmpty())
            w += QStringLiteral(" · ");
        w += c.resolver;
    }
    return w;
}

} // namespace

ContextRowWidget::ContextRowWidget(const Context &ctx, const QString &statusText, bool running, bool failed,
                                   QWidget *parent)
    : QWidget(parent)
{
    setObjectName(QStringLiteral("contextRow"));
    setMinimumHeight(72);

    auto *title = new QLabel(ctx.label.isEmpty() ? ctx.id.left(8) : ctx.label);
    title->setObjectName(QStringLiteral("contextTitle"));
    title->setWordWrap(false);

    auto *path = new QLabel(ctx.workdir);
    path->setObjectName(QStringLiteral("contextPath"));
    path->setWordWrap(false);
    path->setTextInteractionFlags(Qt::TextSelectableByMouse);

    const QString meta = metaLine(ctx);
    QLabel *metaLbl = nullptr;
    if (!meta.isEmpty()) {
        metaLbl = new QLabel(meta);
        metaLbl->setObjectName(QStringLiteral("contextMeta"));
        metaLbl->setWordWrap(false);
    }

    auto *badge = new QLabel(statusText);
    badge->setObjectName(QStringLiteral("statusBadge"));
    badge->setAlignment(Qt::AlignCenter);
    if (failed)
        badge->setProperty("status", QStringLiteral("failed"));
    else if (running)
        badge->setProperty("status", QStringLiteral("running"));
    else
        badge->setProperty("status", QStringLiteral("stopped"));
    badge->style()->unpolish(badge);
    badge->style()->polish(badge);

    auto *left = new QVBoxLayout;
    left->setSpacing(2);
    left->setContentsMargins(0, 0, 0, 0);
    left->addWidget(title);
    left->addWidget(path);
    if (metaLbl)
        left->addWidget(metaLbl);

    auto *row = new QHBoxLayout(this);
    row->setContentsMargins(12, 10, 12, 10);
    row->setSpacing(16);
    row->addLayout(left, 1);
    row->addWidget(badge, 0, Qt::AlignRight | Qt::AlignVCenter);
}
