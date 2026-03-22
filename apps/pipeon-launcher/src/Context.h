#pragma once

#include <QString>
#include <QJsonObject>
#include <QUuid>

struct Context {
    QString id;
    QString label;
    QString workdir;
    /// Bundled workflow name (e.g. vscode); empty if using workflowFile only.
    QString workflow;
    /// Path to dockpipe.yml / config; empty if using workflow name only.
    QString workflowFile;
    QString resolver;
    QString strategy;
    QString runtime;
    QString dockpipeBinary;
    QString envFile;

    static Context createNew();
    static Context fromJson(const QJsonObject &o);
    QJsonObject toJson() const;
};
