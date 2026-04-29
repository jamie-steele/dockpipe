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
        QStringList options;
        bool sensitive = false;
    };

    explicit PromptDialog(const Spec &spec, QWidget *parent = nullptr);

    QString response() const;

private:
    void buildChoiceUi();
    void buildInputUi();
    void buildConfirmUi();

    QString m_type;
    QString m_defaultValue;
    QString m_response;
    QStringList m_options;
    bool m_sensitive = false;
    QLineEdit *m_input = nullptr;
};
