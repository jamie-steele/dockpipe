#include "SingleInstanceGuard.h"

#include <QObject>
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

bool SingleInstanceGuard::notifyPrimaryToActivate()
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
    socket.write("SHOW\n");
    socket.waitForBytesWritten(2000);
    return true;
}

bool SingleInstanceGuard::tryRunPrimaryInstance()
{
    // Another instance holds the segment
    if (m_mem.attach()) {
        m_mem.detach();
        notifyPrimaryToActivate();
        return false;
    }

    if (!m_mem.create(1)) {
        if (m_mem.error() == QSharedMemory::AlreadyExists) {
            QThread::msleep(120);
            if (m_mem.attach()) {
                m_mem.detach();
                notifyPrimaryToActivate();
                return false;
            }
        }
        qWarning("Pipeon: single-instance shared memory unavailable (%s); continuing without guard.",
                 qPrintable(m_mem.errorString()));
        return true;
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

            if (!data.contains("SHOW") || !m_target)
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
