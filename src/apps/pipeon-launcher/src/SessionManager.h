#pragma once

#include "Context.h"

#include <QHash>
#include <QObject>
#include <QPointer>
#include <QProcess>
#include <QStringList>

enum class SessionStatus { Stopped, Starting, Running, Failed };

struct SessionInfo {
    QString contextId;
    SessionStatus status = SessionStatus::Stopped;
    QString logPath;
    QString program;
    QStringList arguments;
    qint64 pid = 0;
    QString errorString;
};

class SessionManager : public QObject {
    Q_OBJECT
public:
    explicit SessionManager(QObject *parent = nullptr);

    SessionInfo info(const QString &contextId) const;
    bool isRunning(const QString &contextId) const;

    /// Build dockpipe argv (program + args) for a context.
    static QStringList dockpipeArguments(const Context &ctx);

    bool launch(const Context &ctx, const QString &logsDir);
    void stop(const QString &contextId);

signals:
    void sessionStarted(const QString &contextId);
    void sessionStopped(const QString &contextId, int exitCode, QProcess::ExitStatus);
    void sessionFailed(const QString &contextId, const QString &error);
    void sessionOutput(const QString &contextId, const QString &text);

private:
    QHash<QString, QPointer<QProcess>> m_processes;
    QHash<QString, SessionInfo> m_info;
};
