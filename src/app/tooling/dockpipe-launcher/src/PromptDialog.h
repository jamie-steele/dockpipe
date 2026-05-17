#pragma once

#include <QDialog>
#include <QStringList>

class QLineEdit;

class PromptDialog : public QDialog {
    Q_OBJECT
public:
    struct Spec {
        QString type;
        QString title;
        QString message;
        QString defaultValue;
        QString intent;
        QString automationGroup;
        QString pathMode;
        QString fileFilter;
        QString baseDir;
        QStringList options;
        bool sensitive = false;
        bool mustExist = false;
    };

    explicit PromptDialog(const Spec &spec, QWidget *parent = nullptr);

    QString response() const;

private:
    void buildChoiceUi();
    void buildInputUi();
    void buildConfirmUi();
    void buildFileUi();
    void chooseFilePath();

    QString m_type;
    QString m_defaultValue;
    QString m_response;
    QStringList m_options;
    QString m_pathMode;
    QString m_fileFilter;
    QString m_baseDir;
    bool m_sensitive = false;
    bool m_mustExist = false;
    QLineEdit *m_input = nullptr;
};
