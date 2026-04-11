#include "SingleInstanceGuard.h"

#include <QObject>
#include <QMetaObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QSharedMemory>
#include <QThread>
#include <QWidget>

#include <QtGlobal>

namespace {

QString ipcServerName()
{
    return QStringLiteral("org.pipeon.launcher.ipc.v1");
}

QString sharedMemoryKey()
{
    return QStringLiteral("org.pipeon.launcher.shmem.v1");
}

} // namespace

SingleInstanceGuard::SingleInstanceGuard()
    : m_mem(sharedMemoryKey())
{
}

SingleInstanceGuard::~SingleInstanceGuard()
{
    m_server.close();
    if (m_mem.isAttached())
        m_mem.detach();
}

bool SingleInstanceGuard::notifyPrimaryToActivate(bool startHome)
{
    QLocalSocket socket;
    for (int i = 0; i < 25; ++i) {
        socket.connectToServer(ipcServerName());
        if (socket.waitForConnected(200))
            break;
        QThread::msleep(40);
    }
    if (socket.state() != QLocalSocket::ConnectedState)
        return false;
    socket.write(startHome ? "SHOW_HOME\n" : "SHOW\n");
    socket.waitForBytesWritten(2000);
    return true;
}

bool SingleInstanceGuard::tryRunPrimaryInstance(bool startHome)
{
    // Another instance appears to hold the segment. Only exit if we can actually
    // reach that primary process; otherwise recover from stale IPC state.
    if (m_mem.attach()) {
        m_mem.detach();
        if (notifyPrimaryToActivate(startHome))
            return false;
        qWarning("Pipeon: stale single-instance shared memory found; recovering primary instance.");
    }

    if (!m_mem.create(1)) {
        if (m_mem.error() == QSharedMemory::AlreadyExists) {
            QThread::msleep(120);
            if (m_mem.attach()) {
                m_mem.detach();
                if (notifyPrimaryToActivate(startHome))
                    return false;
                qWarning("Pipeon: stale shared memory persisted after retry; continuing as primary.");
                if (!m_mem.create(1)) {
                    qWarning("Pipeon: could not recreate shared memory after stale-state recovery (%s); continuing without guard.",
                             qPrintable(m_mem.errorString()));
                    return true;
                }
            } else if (!m_mem.create(1)) {
                qWarning("Pipeon: single-instance shared memory unavailable after retry (%s); continuing without guard.",
                         qPrintable(m_mem.errorString()));
                return true;
            }
        }
        if (!m_mem.isAttached()) {
            qWarning("Pipeon: single-instance shared memory unavailable (%s); continuing without guard.",
                     qPrintable(m_mem.errorString()));
            return true;
        }
    }

    const QString name = ipcServerName();
    QLocalServer::removeServer(name);
    if (!m_server.listen(name)) {
        qWarning("Pipeon: could not bind IPC server (%s); continuing without single-instance.",
                 qPrintable(m_server.errorString()));
        return true;
    }

    QObject::connect(&m_server, &QLocalServer::newConnection, &m_server, [this]() {
        QLocalSocket *client = m_server.nextPendingConnection();
        if (!client)
            return;

        QObject::connect(client, &QLocalSocket::readyRead, client, [this, client]() {
            const QByteArray data = client->readAll();
            client->deleteLater();

            if (!m_target)
                return;

            if (data.contains("SHOW_HOME")) {
                QMetaObject::invokeMethod(m_target, "activateHome", Qt::QueuedConnection);
                return;
            }
            if (!data.contains("SHOW"))
                return;

            m_target->show();
            if (m_target->isMinimized())
                m_target->showNormal();
            m_target->raise();
            m_target->activateWindow();
        });
    });
    return true;
}
