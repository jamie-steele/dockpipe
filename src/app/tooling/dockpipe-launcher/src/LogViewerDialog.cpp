#include "LogViewerDialog.h"

#include <QDialogButtonBox>
#include <QFile>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScrollBar>
#include <QTimer>
#include <QVBoxLayout>

namespace {

QString elideMiddle(const QString &text, int maxChars)
{
    if (text.size() <= maxChars)
        return text;
    const int keep = qMax(8, maxChars / 2 - 2);
    return text.left(keep) + QStringLiteral(" … ") + text.right(keep);
}

} // namespace

LogViewerDialog::LogViewerDialog(const QString &title,
                                 const QString &logPath,
                                 const QString &commandLine,
                                 bool running,
                                 QWidget *parent)
    : QDialog(parent), m_logPath(logPath)
{
    setWindowTitle(title.isEmpty() ? tr("Logs") : title);
    resize(900, 620);

    auto *outer = new QVBoxLayout(this);

    m_status = new QLabel(running ? tr("Live session output") : tr("Last captured session output"));
    m_status->setObjectName(QStringLiteral("logViewerStatus"));

    m_command = new QLabel(commandLine.isEmpty() ? tr("Command unavailable") : commandLine);
    m_command->setObjectName(QStringLiteral("logViewerCommand"));
    m_command->setWordWrap(true);
    m_command->setTextInteractionFlags(Qt::TextSelectableByMouse);

    m_path = new QLabel(logPath.isEmpty() ? tr("No log file path recorded.") : elideMiddle(logPath, 120));
    m_path->setObjectName(QStringLiteral("logViewerPath"));
    m_path->setToolTip(logPath);
    m_path->setTextInteractionFlags(Qt::TextSelectableByMouse);

    m_output = new QPlainTextEdit(this);
    m_output->setObjectName(QStringLiteral("detailConsole"));
    m_output->setReadOnly(true);
    m_output->setLineWrapMode(QPlainTextEdit::NoWrap);
    m_output->setPlaceholderText(tr("No log output yet."));

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close, this);
    auto *refresh = new QPushButton(tr("Refresh"), this);
    refresh->setObjectName(QStringLiteral("secondaryButton"));
    buttons->addButton(refresh, QDialogButtonBox::ActionRole);

    outer->addWidget(m_status);
    outer->addWidget(m_command);
    outer->addWidget(m_path);
    outer->addWidget(m_output, 1);
    outer->addWidget(buttons);

    connect(refresh, &QPushButton::clicked, this, &LogViewerDialog::refreshLog);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);

    if (running) {
        m_timer = new QTimer(this);
        m_timer->setInterval(900);
        connect(m_timer, &QTimer::timeout, this, &LogViewerDialog::refreshLog);
        m_timer->start();
    }

    refreshLog();
}

void LogViewerDialog::refreshLog()
{
    if (m_logPath.isEmpty()) {
        m_output->setPlainText(tr("# No log file path recorded for this session."));
        return;
    }

    QFile file(m_logPath);
    if (!file.exists()) {
        m_output->setPlainText(tr("# Log file does not exist yet.\n# %1").arg(m_logPath));
        return;
    }
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text)) {
        m_output->setPlainText(tr("# Could not open log file.\n# %1").arg(m_logPath));
        return;
    }

    const QByteArray data = file.readAll();
    if (data.size() == m_lastSize)
        return;

    m_lastSize = data.size();
    m_output->setPlainText(QString::fromLocal8Bit(data));
    auto *bar = m_output->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}
