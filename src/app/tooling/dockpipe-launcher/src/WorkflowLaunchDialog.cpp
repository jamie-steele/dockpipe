#include "WorkflowLaunchDialog.h"

#include <QCheckBox>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFormLayout>
#include <QGroupBox>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMap>
#include <QPushButton>
#include <QScrollArea>
#include <QVBoxLayout>

namespace {

QString inputAttribute(const WorkflowInputMeta &input, const QString &key)
{
    return input.attributes.value(key.trimmed().toLower()).trimmed();
}

bool envLooksBoolean(const WorkflowInputMeta &input)
{
    const QString control = inputAttribute(input, QStringLiteral("control")).toLower();
    if (control == QStringLiteral("checkbox") || control == QStringLiteral("boolean") || control == QStringLiteral("bool"))
        return true;
    const QString def = input.defaultValue.trimmed().toLower();
    return def == QStringLiteral("0") || def == QStringLiteral("1") || def == QStringLiteral("true")
           || def == QStringLiteral("false") || def == QStringLiteral("yes") || def == QStringLiteral("no")
           || input.envName.contains(QStringLiteral("_ENABLE")) || input.envName.contains(QStringLiteral("_REQUIRED"))
           || input.envName.contains(QStringLiteral("_AUTO"));
}

bool envLooksSensitive(const WorkflowInputMeta &input)
{
    const QString env = input.envName.toUpper();
    return env.contains(QStringLiteral("PASSWORD")) || env.contains(QStringLiteral("TOKEN"))
           || env.contains(QStringLiteral("SECRET")) || env.contains(QStringLiteral("KEY"));
}

bool envLooksPath(const WorkflowInputMeta &input)
{
    const QString control = inputAttribute(input, QStringLiteral("control")).toLower();
    if (control == QStringLiteral("file") || control == QStringLiteral("path") || control == QStringLiteral("directory")
        || control == QStringLiteral("dir"))
        return true;
    const QString env = input.envName.toUpper();
    return env.contains(QStringLiteral("PATH")) || env.contains(QStringLiteral("FILE"))
           || env.contains(QStringLiteral("ISO")) || env.contains(QStringLiteral("DISK"))
           || env.contains(QStringLiteral("FIRMWARE")) || env.contains(QStringLiteral("BIOS"))
           || env.contains(QStringLiteral("DIR"));
}

QString displayNameForInput(const WorkflowInputMeta &input)
{
    const QString display = inputAttribute(input, QStringLiteral("displayname"));
    if (!display.isEmpty())
        return display;
    if (!input.fieldName.trimmed().isEmpty())
        return input.fieldName.trimmed();
    return input.envName.trimmed();
}

} // namespace

WorkflowLaunchDialog::WorkflowLaunchDialog(const WorkflowMeta &workflow, const QMap<QString, QString> &currentValues,
                                           QWidget *parent)
    : QDialog(parent)
    , m_workflow(workflow)
{
    setWindowTitle(tr("Workflow Settings — %1").arg(workflow.displayName));
    resize(760, 620);

    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(14, 14, 14, 14);
    layout->setSpacing(10);

    auto *intro = new QLabel(workflow.description.trimmed().isEmpty()
                                 ? tr("Provide workflow settings before launch. Values are stored for this project/workflow and passed through DockPipe as vars/env.")
                                 : workflow.description.trimmed());
    intro->setWordWrap(true);
    layout->addWidget(intro);

    auto *scroll = new QScrollArea(this);
    scroll->setWidgetResizable(true);
    auto *content = new QWidget(scroll);
    auto *contentLayout = new QVBoxLayout(content);
    contentLayout->setContentsMargins(0, 0, 0, 0);
    contentLayout->setSpacing(12);

    QMap<QString, QFormLayout *> formsByGroup;
    auto ensureGroup = [&](const QString &groupName) -> QFormLayout * {
        const QString key = groupName.trimmed().isEmpty() ? tr("Inputs") : groupName.trimmed();
        if (formsByGroup.contains(key))
            return formsByGroup.value(key);
        auto *box = new QGroupBox(key);
        auto *form = new QFormLayout(box);
        form->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);
        form->setSpacing(10);
        form->setHorizontalSpacing(16);
        contentLayout->addWidget(box);
        formsByGroup.insert(key, form);
        return form;
    };

    for (const WorkflowInputMeta &input : workflow.inputs) {
        const QString envName = input.envName.trimmed();
        QString value = currentValues.value(envName);
        if (value.isEmpty())
            value = input.defaultValue;
        QFormLayout *form = ensureGroup(inputAttribute(input, QStringLiteral("group")));
        QWidget *groupWidget = form->parentWidget();

        if (envLooksBoolean(input)) {
            auto *check = new QCheckBox(groupWidget);
            const QString lower = value.trimmed().toLower();
            check->setChecked(lower == QStringLiteral("1") || lower == QStringLiteral("true")
                              || lower == QStringLiteral("yes") || lower == QStringLiteral("on"));
            m_editors.insert(envName, check);
            form->addRow(displayNameForInput(input), check);
        } else {
            auto *edit = new QLineEdit(value, groupWidget);
            const QString placeholder = inputAttribute(input, QStringLiteral("placeholder"));
            if (!placeholder.isEmpty())
                edit->setPlaceholderText(placeholder);
            if (envLooksSensitive(input))
                edit->setEchoMode(QLineEdit::PasswordEchoOnEdit);
            QWidget *fieldWidget = edit;
            if (envLooksPath(input)) {
                auto *row = new QWidget(groupWidget);
                auto *rowLayout = new QHBoxLayout(row);
                rowLayout->setContentsMargins(0, 0, 0, 0);
                rowLayout->setSpacing(8);
                auto *browse = new QPushButton(tr("Browse…"), row);
                rowLayout->addWidget(edit, 1);
                rowLayout->addWidget(browse, 0);
                QObject::connect(browse, &QPushButton::clicked, this, [this, edit, input]() {
                    const QString control = inputAttribute(input, QStringLiteral("control")).toLower();
                    const QString fileMode = inputAttribute(input, QStringLiteral("filemode")).toLower();
                    const QString env = input.envName.toUpper();
                    QString selected;
                    if (control == QStringLiteral("directory") || control == QStringLiteral("dir")
                        || fileMode == QStringLiteral("open-dir") || env.contains(QStringLiteral("DIR"))) {
                        selected = QFileDialog::getExistingDirectory(this, tr("Choose folder"), edit->text());
                    } else if (fileMode == QStringLiteral("save-file")
                               || (env.contains(QStringLiteral("DISK")) && !env.contains(QStringLiteral("CDROM"))
                                   && !env.contains(QStringLiteral("ISO")))) {
                        selected = QFileDialog::getSaveFileName(this, tr("Choose file"), edit->text(),
                                                                tr("All files (*)"));
                    } else {
                        selected = QFileDialog::getOpenFileName(this, tr("Choose file"), edit->text(),
                                                                tr("All files (*)"));
                    }
                    if (!selected.isEmpty())
                        edit->setText(selected);
                });
                fieldWidget = row;
            }
            m_editors.insert(envName, edit);
            form->addRow(displayNameForInput(input), fieldWidget);
        }

        QString help = input.description.trimmed();
        if (!envName.isEmpty()) {
            if (!help.isEmpty())
                help += QLatin1Char('\n');
            help += tr("Maps to %1").arg(envName);
        }
        if (!input.defaultValue.trimmed().isEmpty()) {
            if (!help.isEmpty())
                help += QLatin1Char('\n');
            help += tr("Default: %1").arg(input.defaultValue.trimmed());
        }
        if (!help.isEmpty()) {
            auto *helpLabel = new QLabel(help, groupWidget);
            helpLabel->setObjectName(QStringLiteral("hintText"));
            helpLabel->setWordWrap(true);
            form->addRow(QString(), helpLabel);
        }
    }

    contentLayout->addStretch(1);
    scroll->setWidget(content);
    layout->addWidget(scroll, 1);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel, this);
    if (QPushButton *ok = buttons->button(QDialogButtonBox::Ok))
        ok->setText(tr("Launch"));
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
}

QMap<QString, QString> WorkflowLaunchDialog::values() const
{
    QMap<QString, QString> out;
    for (auto it = m_editors.begin(); it != m_editors.end(); ++it) {
        if (auto *check = qobject_cast<QCheckBox *>(it.value())) {
            out.insert(it.key(), check->isChecked() ? QStringLiteral("1") : QStringLiteral("0"));
            continue;
        }
        if (auto *edit = qobject_cast<QLineEdit *>(it.value())) {
            out.insert(it.key(), edit->text().trimmed());
        }
    }
    return out;
}
