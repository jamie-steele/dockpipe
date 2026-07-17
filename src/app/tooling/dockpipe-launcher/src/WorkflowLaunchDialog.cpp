#include "WorkflowLaunchDialog.h"

#include <QCheckBox>
#include <QComboBox>
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
#include <QTabWidget>
#include <QVBoxLayout>
#include <functional>

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

bool inputIsList(const WorkflowInputMeta &input)
{
    return input.type.trimmed().startsWith(QStringLiteral("List<")) || !input.elementType.trimmed().isEmpty();
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

QString displayNameForSection(const WorkflowViewSectionMeta &section)
{
    if (!section.title.trimmed().isEmpty())
        return section.title.trimmed();
    if (!section.id.trimmed().isEmpty())
        return section.id.trimmed();
    return QObject::tr("Section");
}

QString displayNameForPage(const WorkflowViewPageMeta &page)
{
    if (!page.title.trimmed().isEmpty())
        return page.title.trimmed();
    if (!page.id.trimmed().isEmpty())
        return page.id.trimmed();
    return QObject::tr("Page");
}

QString displayNameForEntryOption(const WorkflowViewEntryOptionMeta &option)
{
    if (!option.label.trimmed().isEmpty())
        return option.label.trimmed();
    return option.value.trimmed();
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

    if (!workflow.view.entry.options.isEmpty() || !workflow.view.pages.isEmpty())
        addViewPagesToLayout(contentLayout, currentValues);
    else
        addInputsToLayout(contentLayout, workflow.inputs, currentValues, true);

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

const WorkflowInputMeta *WorkflowLaunchDialog::findInputByPath(const QString &path) const
{
    const QStringList parts = path.split(QStringLiteral("."), Qt::SkipEmptyParts);
    if (parts.isEmpty())
        return nullptr;

    std::function<const WorkflowInputMeta *(const QVector<WorkflowInputMeta> &, int)> findIn =
        [&](const QVector<WorkflowInputMeta> &inputs, int index) -> const WorkflowInputMeta * {
            const QString want = parts.value(index).trimmed();
            if (want.isEmpty())
                return nullptr;
            for (const WorkflowInputMeta &input : inputs) {
                if (input.fieldName.trimmed() != want)
                    continue;
                if (index == parts.size() - 1)
                    return &input;
                return findIn(input.children, index + 1);
            }
            return nullptr;
        };

    return findIn(m_workflow.inputs, 0);
}

QVector<WorkflowViewPageMeta> WorkflowLaunchDialog::routedPagesForSelection(const QString &selectedValue) const
{
    if (m_workflow.view.entry.options.isEmpty())
        return m_workflow.view.pages;

    QStringList allowed;
    for (const WorkflowViewEntryOptionMeta &option : m_workflow.view.entry.options) {
        if (option.value.trimmed() != selectedValue.trimmed())
            continue;
        allowed = option.pages;
        if (allowed.isEmpty() && !option.next.trimmed().isEmpty())
            allowed = { option.next.trimmed() };
        break;
    }
    if (allowed.isEmpty())
        return m_workflow.view.pages;

    QVector<WorkflowViewPageMeta> routed;
    for (const QString &pageId : allowed) {
        for (const WorkflowViewPageMeta &page : m_workflow.view.pages) {
            if (page.id.trimmed() != pageId.trimmed())
                continue;
            routed.append(page);
            break;
        }
    }
    return routed.isEmpty() ? m_workflow.view.pages : routed;
}

void WorkflowLaunchDialog::rebuildRoutedPages(const QMap<QString, QString> &currentValues)
{
    if (!m_routedPagesHost || !m_routedPagesContainer)
        return;

    QMap<QString, QString> effectiveValues = currentValues;
    const QMap<QString, QString> liveValues = values();
    for (auto it = liveValues.begin(); it != liveValues.end(); ++it)
        effectiveValues.insert(it.key(), it.value());

    for (auto it = m_editors.begin(); it != m_editors.end();) {
        QWidget *widget = it.value();
        bool underRoutedPages = false;
        for (QWidget *p = widget; p != nullptr; p = p->parentWidget()) {
            if (p == m_routedPagesContainer) {
                underRoutedPages = true;
                break;
            }
        }
        if (underRoutedPages)
            it = m_editors.erase(it);
        else
            ++it;
    }

    QLayoutItem *child = nullptr;
    while ((child = m_routedPagesHost->takeAt(0)) != nullptr) {
        if (child->widget())
            child->widget()->deleteLater();
        delete child;
    }

    QString selectedValue;
    if (m_entryChoice)
        selectedValue = m_entryChoice->currentData().toString().trimmed();
    const QVector<WorkflowViewPageMeta> pages = routedPagesForSelection(selectedValue);

    if (pages.size() == 1) {
        const WorkflowViewPageMeta &page = pages.first();
        if (!page.description.trimmed().isEmpty()) {
            auto *desc = new QLabel(page.description.trimmed(), m_routedPagesContainer);
            desc->setWordWrap(true);
            m_routedPagesHost->addWidget(desc);
        }
        for (const WorkflowViewSectionMeta &section : page.sections)
            addViewSectionToLayout(m_routedPagesHost, section, effectiveValues);
        return;
    }

    auto *tabs = new QTabWidget(m_routedPagesContainer);
    for (const WorkflowViewPageMeta &page : pages) {
        auto *pageWidget = new QWidget(tabs);
        auto *pageLayout = new QVBoxLayout(pageWidget);
        pageLayout->setContentsMargins(10, 10, 10, 10);
        pageLayout->setSpacing(12);
        if (!page.description.trimmed().isEmpty()) {
            auto *desc = new QLabel(page.description.trimmed(), pageWidget);
            desc->setWordWrap(true);
            pageLayout->addWidget(desc);
        }
        for (const WorkflowViewSectionMeta &section : page.sections)
            addViewSectionToLayout(pageLayout, section, effectiveValues);
        pageLayout->addStretch(1);
        tabs->addTab(pageWidget, displayNameForPage(page));
    }
    m_routedPagesHost->addWidget(tabs, 1);
}

void WorkflowLaunchDialog::addViewPagesToLayout(QVBoxLayout *layout, const QMap<QString, QString> &currentValues)
{
    if (!layout)
        return;
    if (!m_workflow.view.entry.options.isEmpty()) {
        const WorkflowInputMeta *entryInput = findInputByPath(m_workflow.view.entry.field);
        if (entryInput && entryInput->children.isEmpty() && !entryInput->envName.trimmed().isEmpty()) {
            auto *box = new QGroupBox(m_workflow.view.entry.title.trimmed().isEmpty() ? tr("Start") : m_workflow.view.entry.title.trimmed(),
                                      layout->parentWidget());
            auto *boxLayout = new QVBoxLayout(box);
            boxLayout->setContentsMargins(12, 12, 12, 12);
            boxLayout->setSpacing(10);
            if (!m_workflow.view.entry.description.trimmed().isEmpty()) {
                auto *desc = new QLabel(m_workflow.view.entry.description.trimmed(), box);
                desc->setWordWrap(true);
                boxLayout->addWidget(desc);
            }
            m_entryChoice = new QComboBox(box);
            const QString envName = entryInput->envName.trimmed();
            QString selectedValue = currentValues.value(envName);
            if (selectedValue.isEmpty())
                selectedValue = entryInput->defaultValue;
            for (const WorkflowViewEntryOptionMeta &option : m_workflow.view.entry.options) {
                m_entryChoice->addItem(displayNameForEntryOption(option), option.value);
            }
            int selectedIndex = m_entryChoice->findData(selectedValue);
            if (selectedIndex < 0)
                selectedIndex = 0;
            m_entryChoice->setCurrentIndex(selectedIndex);
            m_editors.insert(envName, m_entryChoice);
            boxLayout->addWidget(m_entryChoice);
            layout->addWidget(box);
            connect(m_entryChoice, &QComboBox::currentIndexChanged, this, [this, currentValues](int) {
                rebuildRoutedPages(currentValues);
            });
        }
    }

    m_routedPagesContainer = new QWidget(layout->parentWidget());
    m_routedPagesHost = new QVBoxLayout(m_routedPagesContainer);
    m_routedPagesHost->setContentsMargins(0, 0, 0, 0);
    m_routedPagesHost->setSpacing(12);
    layout->addWidget(m_routedPagesContainer, 1);
    rebuildRoutedPages(currentValues);
}

void WorkflowLaunchDialog::addViewSectionToLayout(QVBoxLayout *layout, const WorkflowViewSectionMeta &section,
                                                  const QMap<QString, QString> &currentValues)
{
    if (!layout || section.fields.isEmpty())
        return;

    auto *box = new QGroupBox(displayNameForSection(section), layout->parentWidget());
    auto *boxLayout = new QVBoxLayout(box);
    boxLayout->setContentsMargins(12, 12, 12, 12);
    boxLayout->setSpacing(10);

    if (!section.description.trimmed().isEmpty()) {
        auto *help = new QLabel(section.description.trimmed(), box);
        help->setWordWrap(true);
        boxLayout->addWidget(help);
    }

    QFormLayout *leafForm = nullptr;
    for (const QString &fieldPath : section.fields) {
        const WorkflowInputMeta *input = findInputByPath(fieldPath);
        if (!input)
            continue;
        if (!input->children.isEmpty()) {
            addInputsToLayout(boxLayout, { *input }, currentValues, false);
            continue;
        }
        if (!leafForm) {
            leafForm = new QFormLayout();
            leafForm->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);
            leafForm->setSpacing(10);
            leafForm->setHorizontalSpacing(16);
            boxLayout->addLayout(leafForm);
        }
        const QString envName = input->envName.trimmed();
        QString value = currentValues.value(envName);
        if (value.isEmpty())
            value = input->defaultValue;
        addLeafInputRow(leafForm, *input, value, box);
    }

    layout->addWidget(box);
}

QMap<QString, QString> WorkflowLaunchDialog::values() const
{
    QMap<QString, QString> out;
    for (auto it = m_editors.begin(); it != m_editors.end(); ++it) {
        if (auto *check = qobject_cast<QCheckBox *>(it.value())) {
            out.insert(it.key(), check->isChecked() ? QStringLiteral("1") : QStringLiteral("0"));
            continue;
        }
        if (auto *combo = qobject_cast<QComboBox *>(it.value())) {
            out.insert(it.key(), combo->currentData().toString().trimmed());
            continue;
        }
        if (auto *edit = qobject_cast<QLineEdit *>(it.value())) {
            out.insert(it.key(), edit->text().trimmed());
        }
    }
    return out;
}

void WorkflowLaunchDialog::addInputsToLayout(QVBoxLayout *layout, const QVector<WorkflowInputMeta> &inputs,
                                             const QMap<QString, QString> &currentValues, bool topLevel)
{
    if (!layout)
        return;
    QMap<QString, QFormLayout *> formsByGroup;
    auto ensureGroup = [&](const QString &groupName) -> QFormLayout * {
        const QString key = groupName.trimmed().isEmpty() ? tr("Inputs") : groupName.trimmed();
        if (formsByGroup.contains(key))
            return formsByGroup.value(key);
        auto *box = new QGroupBox(key, layout->parentWidget());
        auto *form = new QFormLayout(box);
        form->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);
        form->setSpacing(10);
        form->setHorizontalSpacing(16);
        layout->addWidget(box);
        formsByGroup.insert(key, form);
        return form;
    };

    for (const WorkflowInputMeta &input : inputs) {
        if (!input.children.isEmpty()) {
            auto *box = new QGroupBox(displayNameForInput(input), layout->parentWidget());
            auto *boxLayout = new QVBoxLayout(box);
            boxLayout->setContentsMargins(12, 12, 12, 12);
            boxLayout->setSpacing(10);
            const QString help = helpTextForInput(input);
            if (!help.isEmpty()) {
                auto *helpLabel = new QLabel(help, box);
                helpLabel->setObjectName(QStringLiteral("hintText"));
                helpLabel->setWordWrap(true);
                boxLayout->addWidget(helpLabel);
            }
            addInputsToLayout(boxLayout, input.children, currentValues, false);
            layout->addWidget(box);
            continue;
        }

        const QString envName = input.envName.trimmed();
        QString value = currentValues.value(envName);
        if (value.isEmpty())
            value = input.defaultValue;
        QFormLayout *form = ensureGroup(topLevel ? inputAttribute(input, QStringLiteral("group")) : QString());
        addLeafInputRow(form, input, value, form->parentWidget());
    }
}

void WorkflowLaunchDialog::addLeafInputRow(QFormLayout *form, const WorkflowInputMeta &input, const QString &value,
                                           QWidget *groupWidget)
{
    if (!form || !groupWidget)
        return;
    const QString envName = input.envName.trimmed();
    if (envLooksBoolean(input)) {
        auto *check = new QCheckBox(groupWidget);
        const QString lower = value.trimmed().toLower();
        check->setChecked(lower == QStringLiteral("1") || lower == QStringLiteral("true")
                          || lower == QStringLiteral("yes") || lower == QStringLiteral("on"));
        m_editors.insert(envName, check);
        form->addRow(displayNameForInput(input), check);
    } else {
        auto *edit = new QLineEdit(value, groupWidget);
        QString placeholder = inputAttribute(input, QStringLiteral("placeholder"));
        if (placeholder.isEmpty() && inputIsList(input))
            placeholder = tr("Separate values with ';'");
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

    const QString help = helpTextForInput(input);
    if (!help.isEmpty()) {
        auto *helpLabel = new QLabel(help, groupWidget);
        helpLabel->setObjectName(QStringLiteral("hintText"));
        helpLabel->setWordWrap(true);
        form->addRow(QString(), helpLabel);
    }
}

QString WorkflowLaunchDialog::helpTextForInput(const WorkflowInputMeta &input) const
{
    QString help = input.description.trimmed();
    if (!input.type.trimmed().isEmpty()) {
        if (!help.isEmpty())
            help += QLatin1Char('\n');
        help += tr("Type: %1").arg(input.type.trimmed());
    }
    if (!input.envName.trimmed().isEmpty()) {
        if (!help.isEmpty())
            help += QLatin1Char('\n');
        help += tr("Maps to %1").arg(input.envName.trimmed());
    }
    if (!input.defaultValue.trimmed().isEmpty()) {
        if (!help.isEmpty())
            help += QLatin1Char('\n');
        help += tr("Default: %1").arg(input.defaultValue.trimmed());
    }
    if (inputIsList(input) && input.children.isEmpty()) {
        if (!help.isEmpty())
            help += QLatin1Char('\n');
        help += tr("List values currently use ';' as the launcher input separator.");
    }
    return help;
}
