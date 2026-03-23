#pragma once

#include "Context.h"

#include <QWidget>

class QLabel;

/// One row in the context list: title, path, optional metadata, status badge.
class ContextRowWidget : public QWidget {
    Q_OBJECT
public:
    explicit ContextRowWidget(const Context &ctx, const QString &statusText, bool running, bool failed,
                              QWidget *parent = nullptr);
};
