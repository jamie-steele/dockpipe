#include "SessionManager.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QProcess>

#ifdef Q_OS_WIN
// taskkill used below
#else
#include <signal.h>
#include <sys/types.h>
#include <unistd.h>
#endif

SessionManager::SessionManager(QObject *parent) : QObject(parent) {}

SessionInfo SessionManager::info(const QString &contextId) const
{
    return m_info.value(contextId);
}

bool SessionManager::isRunning(const QString &contextId) const
{
    const QPointer<QProcess> p = m_processes.value(contextId);
    return p && p->state() != QProcess::NotRunning;
}

QStringList SessionManager::dockpipeArguments(const Context &ctx)
{
    QStringList args;
    if (!ctx.workflowFile.isEmpty()) {
        args << QStringLiteral("--workflow-file") << ctx.workflowFile;
    } else if (!ctx.workflow.isEmpty()) {
        args << QStringLiteral("--workflow") << ctx.workflow;
    } else {
        args << QStringLiteral("--workflow") << QStringLiteral("vscode");
    }
    args << QStringLiteral("--workdir") << QDir::toNativeSeparators(ctx.workdir);
    if (!ctx.resolver.isEmpty())
        args << QStringLiteral("--resolver") << ctx.resolver;
    if (!ctx.runtime.isEmpty())
        args << QStringLiteral("--runtime") << ctx.runtime;
    if (!ctx.strategy.isEmpty())
        args << QStringLiteral("--strategy") << ctx.strategy;
    if (!ctx.envFile.isEmpty())
        args << QStringLiteral("--env-file") << QDir::toNativeSeparators(ctx.envFile);
    return args;
}

bool SessionManager::launch(const Context &ctx, const QString &logsDir)
{
    if (ctx.workdir.isEmpty())
        return false;
    if (m_processes.contains(ctx.id) && m_processes[ctx.id] && m_processes[ctx.id]->state() != QProcess::NotRunning)
        return false;

    QString program = ctx.dockpipeBinary.trimmed();
    if (program.isEmpty())
        program = QStringLiteral("dockpipe");

    const QString ts = QDateTime::currentDateTime().toString(QStringLiteral("yyyyMMdd-HHmmss"));
    const QString logPath = logsDir + QLatin1Char('/') + ctx.id + QLatin1Char('-') + ts + QStringLiteral(".log");

    auto *proc = new QProcess(this);
    proc->setProgram(program);
    proc->setArguments(dockpipeArguments(ctx));
    proc->setProcessChannelMode(QProcess::MergedChannels);
    proc->setWorkingDirectory(QDir::homePath());

    auto *logFile = new QFile(logPath, proc);
    if (!logFile->open(QIODevice::WriteOnly | QIODevice::Append)) {
        delete proc;
        emit sessionFailed(ctx.id, QStringLiteral("Cannot open log file: ") + logPath);
        return false;
    }

    SessionInfo si;
    si.contextId = ctx.id;
    si.status = SessionStatus::Starting;
    si.logPath = logPath;
    m_info[ctx.id] = si;

    connect(proc, &QProcess::readyReadStandardOutput, proc, [proc, logFile]() {
        logFile->write(proc->readAllStandardOutput());
        logFile->flush();
    });

    connect(proc, QOverload<int, QProcess::ExitStatus>::of(&QProcess::finished), this,
            [this, ctx, proc, logFile](int exitCode, QProcess::ExitStatus st) {
                logFile->close();
                logFile->deleteLater();
                m_processes.remove(ctx.id);
                SessionInfo &s = m_info[ctx.id];
                s.status = SessionStatus::Stopped;
                s.pid = 0;
                emit sessionStopped(ctx.id, exitCode, st);
            });

    connect(proc, &QProcess::errorOccurred, this, [this, ctx, proc, logFile](QProcess::ProcessError) {
        if (proc->state() == QProcess::NotRunning) {
            logFile->close();
            logFile->deleteLater();
            m_processes.remove(ctx.id);
            SessionInfo &s = m_info[ctx.id];
            s.status = SessionStatus::Failed;
            s.errorString = proc->errorString();
            emit sessionFailed(ctx.id, proc->errorString());
        }
    });

    proc->start();
    if (!proc->waitForStarted(15000)) {
        emit sessionFailed(ctx.id, proc->errorString());
        logFile->close();
        logFile->deleteLater();
        proc->deleteLater();
        m_info[ctx.id].status = SessionStatus::Failed;
        return false;
    }

    m_info[ctx.id].status = SessionStatus::Running;
    m_info[ctx.id].pid = proc->processId();
    m_processes[ctx.id] = proc;
    emit sessionStarted(ctx.id);
    return true;
}

void SessionManager::stop(const QString &contextId)
{
    QPointer<QProcess> proc = m_processes.value(contextId);
    if (!proc)
        return;
    const qint64 pid = proc->processId();
#ifdef Q_OS_WIN
    proc->terminate();
    if (!proc->waitForFinished(8000)) {
        QProcess::execute(QStringLiteral("taskkill"),
                            {QStringLiteral("/PID"), QString::number(pid), QStringLiteral("/T"), QStringLiteral("/F")});
    }
#else
    if (pid > 0) {
        // Prefer process-group signal so bash child scripts can tear down docker.
        if (::kill(static_cast<pid_t>(-pid), SIGTERM) != 0)
            proc->terminate();
    } else {
        proc->terminate();
    }
    if (!proc->waitForFinished(12000)) {
        proc->kill();
        proc->waitForFinished(3000);
    }
#endif
}
