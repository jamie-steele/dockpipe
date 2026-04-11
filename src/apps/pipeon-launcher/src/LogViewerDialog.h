#pragma once

#include <QDialog>

class QLabel;
class QPlainTextEdit;
class QTimer;

class LogViewerDialog : public QDialog {
    Q_OBJECT
public:
    explicit LogViewerDialog(const QString &title,
                             const QString &logPath,
                             const QString &commandLine,
                             bool running,
                             QWidget *parent = nullptr);

private slots:
    void refreshLog();

private:
    QString m_logPath;
    qint64 m_lastSize = -1;
    QLabel *m_status = nullptr;
    QLabel *m_command = nullptr;
    QLabel *m_path = nullptr;
    QPlainTextEdit *m_output = nullptr;
    QTimer *m_timer = nullptr;
};
