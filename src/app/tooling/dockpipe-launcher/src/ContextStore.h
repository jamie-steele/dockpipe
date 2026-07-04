#pragma once

#include "Context.h"

#include <QVector>

class ContextStore {
public:
    static QString configDir();
    static QString contextsPath();
    static QString statePath();
    static QString logsDir();

    bool load();
    bool save() const;

    QVector<Context> contexts;
};
