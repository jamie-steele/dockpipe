#pragma once

#include "Context.h"

#include <QDialog>

class QComboBox;
class QLineEdit;
class QPlainTextEdit;
class QPushButton;

class EditContextDialog : public QDialog {
    Q_OBJECT
public:
    explicit EditContextDialog(const Context &ctx, QWidget *parent = nullptr);

    Context editedContext() const;

private:
    void populateCombos(const QString &workdir);
    void browseWorkflowFile();
    void browseEnvFile();
    void browseFlathub();

    Context m_original;

    QLineEdit *m_label = nullptr;
    QLineEdit *m_workdir = nullptr;
    QComboBox *m_workflow = nullptr;
    QComboBox *m_workflowFile = nullptr;
    QComboBox *m_resolver = nullptr;
    QComboBox *m_strategy = nullptr;
    QComboBox *m_runtime = nullptr;
    QLineEdit *m_dockpipe = nullptr;
    QLineEdit *m_env = nullptr;
    QPlainTextEdit *m_extraEnv = nullptr;
    QPushButton *m_flathubBtn = nullptr;
};
