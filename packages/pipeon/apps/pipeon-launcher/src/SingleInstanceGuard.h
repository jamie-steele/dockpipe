#pragma once

#include <QLocalServer>
#include <QSharedMemory>

class QWidget;

/**
 * Ensures only one Pipeon Launcher process runs. A second start notifies the first
 * (via QLocalSocket) to show/raise the main window, then exits.
 *
 * Pass --allow-second-instance to skip this (debugging).
 *
 * Not QObject-based: connect uses QLocalServer/QLocalSocket as receivers so MOC is not
 * required for this type (no Q_OBJECT + final).
 */
class SingleInstanceGuard final {
public:
    SingleInstanceGuard();
    ~SingleInstanceGuard();

    SingleInstanceGuard(const SingleInstanceGuard &) = delete;
    SingleInstanceGuard &operator=(const SingleInstanceGuard &) = delete;

    /** @return true if this process should continue as primary; false if secondary (exit main). */
    bool tryRunPrimaryInstance(bool startHome = false);

    void setActivationTarget(QWidget *w) { m_target = w; }

private:
    bool notifyPrimaryToActivate(bool startHome);

    QSharedMemory m_mem;
    QLocalServer m_server;
    QWidget *m_target = nullptr;
};
