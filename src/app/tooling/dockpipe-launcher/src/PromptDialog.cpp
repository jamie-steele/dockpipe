#include "PromptDialog.h"

#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QFrame>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPushButton>
#include <QVBoxLayout>

namespace {

QString actionButtonObjectName(const QString &option)
{
    const QString folded = option.trimmed().toCaseFolded();
    if (folded.contains(QStringLiteral("cancel")))
        return QStringLiteral("dangerButton");
    if (folded.contains(QStringLiteral("enable")) || folded.contains(QStringLiteral("allow")) ||
        folded.contains(QStringLiteral("continue")) || folded.contains(QStringLiteral("ok")) ||
        folded.contains(QStringLiteral("yes")))
        return QStringLiteral("primaryButton");
    return QStringLiteral("secondaryButton");
}

QPushButton *makeActionButton(const QString &label, QWidget *parent)
{
    auto *button = new QPushButton(label, parent);
    button->setObjectName(actionButtonObjectName(label));
    button->setCursor(Qt::PointingHandCursor);
    button->setMinimumHeight(42);
    return button;
}

} // namespace

PromptDialog::PromptDialog(const Spec &spec, QWidget *parent)
    : QDialog(parent), m_type(spec.type), m_defaultValue(spec.defaultValue), m_response(spec.defaultValue),
      m_options(spec.options), m_pathMode(spec.pathMode), m_fileFilter(spec.fileFilter), m_baseDir(spec.baseDir),
      m_resourceMode(spec.resourceMode), m_resourceSelection(spec.resourceSelection),
      m_resourceKind(spec.resourceKind), m_filters(spec.filters), m_sensitive(spec.sensitive),
      m_mustExist(spec.mustExist)
{
    setModal(true);
    setWindowTitle(spec.title.isEmpty() ? tr("DockPipe Prompt") : spec.title);
    setMinimumWidth(560);
    resize(640, 0);

    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(20, 20, 20, 20);
    outer->setSpacing(14);

    auto *card = new QFrame(this);
    card->setObjectName(QStringLiteral("promptDialogCard"));
    auto *cardLay = new QVBoxLayout(card);
    cardLay->setContentsMargins(20, 20, 20, 20);
    cardLay->setSpacing(12);

    QString eyebrowText = tr("Action Required");
    QString hintText;
    if (spec.intent == QStringLiteral("host-mutation")) {
        eyebrowText = tr("System Change");
        hintText = tr("This action may install software, change local configuration, or restart services.");
    } else if (spec.intent == QStringLiteral("destructive")) {
        eyebrowText = tr("Review Carefully");
        hintText = tr("This action may overwrite or remove existing state.");
    } else if (spec.intent == QStringLiteral("credential-use")) {
        eyebrowText = tr("Credential Use");
        hintText = tr("This action will use configured credentials to access an external service.");
    }

    auto *eyebrow = new QLabel(eyebrowText, card);
    eyebrow->setObjectName(QStringLiteral("promptEyebrow"));

    auto *title = new QLabel(spec.title.isEmpty() ? tr("DockPipe Prompt") : spec.title, card);
    title->setObjectName(QStringLiteral("promptTitle"));
    title->setWordWrap(true);

    auto *body = new QLabel(spec.message, card);
    body->setObjectName(QStringLiteral("promptBody"));
    body->setWordWrap(true);

    cardLay->addWidget(eyebrow);
    cardLay->addWidget(title);
    cardLay->addWidget(body);
    if (!hintText.isEmpty()) {
        auto *hint = new QLabel(hintText, card);
        hint->setObjectName(QStringLiteral("promptHint"));
        hint->setWordWrap(true);
        cardLay->addWidget(hint);
    }

    outer->addWidget(card);

    if (m_type == QStringLiteral("choice")) {
        buildChoiceUi();
    } else if (m_type == QStringLiteral("resource")) {
        buildResourceUi();
    } else if (m_type == QStringLiteral("file")) {
        buildFileUi();
    } else if (m_type == QStringLiteral("input")) {
        buildInputUi();
    } else {
        buildConfirmUi();
    }
}

QString PromptDialog::response() const
{
    return m_response;
}

void PromptDialog::chooseFilePath()
{
    QString current = m_input ? m_input->text() : m_defaultValue;
    if (current.isEmpty())
        current = m_baseDir;
    QString selected;
    QStringList selectedPaths;
    const QString effectiveFilter = !m_filters.isEmpty() ? m_filters.join(QStringLiteral(";;")) : m_fileFilter;

    if (m_type == QStringLiteral("resource")) {
        const bool multi = (m_resourceSelection == QStringLiteral("multi"));
        const bool directory = (m_resourceKind == QStringLiteral("directory"));
        const bool createNew = (m_resourceMode == QStringLiteral("new"));
        if (directory) {
            selected = QFileDialog::getExistingDirectory(this, windowTitle(), current);
        } else if (multi) {
            selectedPaths = QFileDialog::getOpenFileNames(this, windowTitle(), current, effectiveFilter);
        } else if (createNew) {
            selected = QFileDialog::getSaveFileName(this, windowTitle(), current, effectiveFilter);
        } else {
            selected = QFileDialog::getOpenFileName(this, windowTitle(), current, effectiveFilter);
        }
    } else {
        if (m_pathMode == QStringLiteral("open-dir")) {
            selected = QFileDialog::getExistingDirectory(this, windowTitle(), current);
        } else if (m_pathMode == QStringLiteral("save-file")) {
            selected = QFileDialog::getSaveFileName(this, windowTitle(), current, m_fileFilter);
        } else {
            selected = QFileDialog::getOpenFileName(this, windowTitle(), current, m_fileFilter);
        }
    }

    if (!selectedPaths.isEmpty() && m_input) {
        m_input->setText(selectedPaths.join(QStringLiteral("; ")));
    } else if (!selected.isEmpty() && m_input) {
        m_input->setText(selected);
    }
}

QStringList PromptDialog::resourceEntries() const
{
    const QString raw = m_input ? m_input->text() : m_defaultValue;
    QStringList out;
    const QStringList parts = raw.split(QLatin1Char(';'), Qt::SkipEmptyParts);
    for (QString part : parts) {
        part = part.trimmed();
        if (part.size() >= 2 && ((part.startsWith(QLatin1Char('"')) && part.endsWith(QLatin1Char('"'))) ||
                                 (part.startsWith(QLatin1Char('\'')) && part.endsWith(QLatin1Char('\''))))) {
            part = part.mid(1, part.size() - 2).trimmed();
        }
        if (!part.isEmpty())
            out.append(part);
    }
    return out;
}

QString PromptDialog::resourceResponse() const
{
    const QStringList entries = resourceEntries();
    if (m_resourceSelection == QStringLiteral("multi")) {
        QJsonArray values;
        for (const QString &entry : entries)
            values.append(entry);
        return QString::fromUtf8(QJsonDocument(values).toJson(QJsonDocument::Compact));
    }
    if (entries.isEmpty())
        return QString();
    return entries.first();
}

void PromptDialog::buildChoiceUi()
{
    auto *outer = qobject_cast<QVBoxLayout *>(layout());
    auto *actionsFrame = new QFrame(this);
    actionsFrame->setObjectName(QStringLiteral("promptActionPanel"));
    auto *actionsLay = new QVBoxLayout(actionsFrame);
    actionsLay->setContentsMargins(16, 16, 16, 16);
    actionsLay->setSpacing(10);

    auto *hint = new QLabel(tr("Choose how DockPipe should continue."), actionsFrame);
    hint->setObjectName(QStringLiteral("promptHint"));
    actionsLay->addWidget(hint);

    for (const QString &option : m_options) {
        auto *button = makeActionButton(option, actionsFrame);
        actionsLay->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, option]() {
            m_response = option;
            accept();
        });
    }

    outer->addWidget(actionsFrame);
}

void PromptDialog::buildInputUi()
{
    auto *outer = qobject_cast<QVBoxLayout *>(layout());
    auto *inputFrame = new QFrame(this);
    inputFrame->setObjectName(QStringLiteral("promptActionPanel"));
    auto *inputLay = new QVBoxLayout(inputFrame);
    inputLay->setContentsMargins(16, 16, 16, 16);
    inputLay->setSpacing(12);

    m_input = new QLineEdit(inputFrame);
    m_input->setObjectName(QStringLiteral("promptInput"));
    m_input->setText(m_defaultValue);
    if (m_sensitive)
        m_input->setEchoMode(QLineEdit::Password);
    inputLay->addWidget(m_input);

    auto *buttons = new QHBoxLayout;
    buttons->addStretch(1);
    auto *cancel = makeActionButton(tr("Cancel"), inputFrame);
    auto *submit = makeActionButton(tr("Continue"), inputFrame);
    cancel->setObjectName(QStringLiteral("secondaryButton"));
    submit->setObjectName(QStringLiteral("primaryButton"));
    buttons->addWidget(cancel);
    buttons->addWidget(submit);
    inputLay->addLayout(buttons);

    connect(cancel, &QPushButton::clicked, this, [this]() {
        m_response = m_defaultValue;
        reject();
    });
    connect(submit, &QPushButton::clicked, this, [this]() {
        m_response = m_input ? m_input->text() : m_defaultValue;
        accept();
    });

    outer->addWidget(inputFrame);
    if (m_input)
        m_input->setFocus();
}

void PromptDialog::buildFileUi()
{
    auto *outer = qobject_cast<QVBoxLayout *>(layout());
    auto *inputFrame = new QFrame(this);
    inputFrame->setObjectName(QStringLiteral("promptActionPanel"));
    auto *inputLay = new QVBoxLayout(inputFrame);
    inputLay->setContentsMargins(16, 16, 16, 16);
    inputLay->setSpacing(12);

    if (!m_fileFilter.isEmpty()) {
        auto *hint = new QLabel(tr("Allowed files: %1").arg(m_fileFilter), inputFrame);
        hint->setObjectName(QStringLiteral("promptHint"));
        hint->setWordWrap(true);
        inputLay->addWidget(hint);
    }

    auto *row = new QHBoxLayout;
    row->setSpacing(10);
    m_input = new QLineEdit(inputFrame);
    m_input->setObjectName(QStringLiteral("promptInput"));
    m_input->setText(m_defaultValue);
    row->addWidget(m_input, 1);
    auto *browse = makeActionButton(tr("Browse…"), inputFrame);
    browse->setObjectName(QStringLiteral("secondaryButton"));
    row->addWidget(browse);
    inputLay->addLayout(row);

    auto *buttons = new QHBoxLayout;
    buttons->addStretch(1);
    auto *cancel = makeActionButton(tr("Cancel"), inputFrame);
    auto *submit = makeActionButton(tr("Continue"), inputFrame);
    cancel->setObjectName(QStringLiteral("secondaryButton"));
    submit->setObjectName(QStringLiteral("primaryButton"));
    buttons->addWidget(cancel);
    buttons->addWidget(submit);
    inputLay->addLayout(buttons);

    connect(browse, &QPushButton::clicked, this, [this]() { chooseFilePath(); });
    connect(cancel, &QPushButton::clicked, this, [this]() {
        m_response = m_defaultValue;
        reject();
    });
    connect(submit, &QPushButton::clicked, this, [this]() {
        const QString selected = m_input ? m_input->text().trimmed() : m_defaultValue;
        if (m_mustExist && !selected.isEmpty()) {
            QFileInfo info(selected);
            if (info.isRelative() && !m_baseDir.isEmpty())
                info = QFileInfo(QDir(m_baseDir).filePath(selected));
            const bool ok = (m_pathMode == QStringLiteral("open-dir")) ? info.exists() && info.isDir() : info.exists();
            if (!ok) {
                QMessageBox::warning(this, tr("DockPipe Prompt"),
                                     (m_pathMode == QStringLiteral("open-dir"))
                                         ? tr("Choose an existing directory before continuing.")
                                         : tr("Choose an existing file before continuing."));
                return;
            }
        }
        m_response = selected;
        accept();
    });

    outer->addWidget(inputFrame);
    if (m_input)
        m_input->setFocus();
}

void PromptDialog::buildResourceUi()
{
    auto *outer = qobject_cast<QVBoxLayout *>(layout());
    auto *inputFrame = new QFrame(this);
    inputFrame->setObjectName(QStringLiteral("promptActionPanel"));
    auto *inputLay = new QVBoxLayout(inputFrame);
    inputLay->setContentsMargins(16, 16, 16, 16);
    inputLay->setSpacing(12);

    QStringList hints;
    if (!m_filters.isEmpty())
        hints.append(tr("Allowed files: %1").arg(m_filters.join(QStringLiteral(";;"))));
    if (m_resourceSelection == QStringLiteral("multi"))
        hints.append(tr("Separate multiple paths with ';' or use Browse…"));
    if (!hints.isEmpty()) {
        auto *hint = new QLabel(hints.join(QStringLiteral("\n")), inputFrame);
        hint->setObjectName(QStringLiteral("promptHint"));
        hint->setWordWrap(true);
        inputLay->addWidget(hint);
    }

    auto *row = new QHBoxLayout;
    row->setSpacing(10);
    m_input = new QLineEdit(inputFrame);
    m_input->setObjectName(QStringLiteral("promptInput"));
    m_input->setText(m_defaultValue);
    row->addWidget(m_input, 1);
    auto *browse = makeActionButton(tr("Browse…"), inputFrame);
    browse->setObjectName(QStringLiteral("secondaryButton"));
    row->addWidget(browse);
    inputLay->addLayout(row);

    auto *buttons = new QHBoxLayout;
    buttons->addStretch(1);
    auto *cancel = makeActionButton(tr("Cancel"), inputFrame);
    auto *submit = makeActionButton(tr("Continue"), inputFrame);
    cancel->setObjectName(QStringLiteral("secondaryButton"));
    submit->setObjectName(QStringLiteral("primaryButton"));
    buttons->addWidget(cancel);
    buttons->addWidget(submit);
    inputLay->addLayout(buttons);

    connect(browse, &QPushButton::clicked, this, [this]() { chooseFilePath(); });
    connect(cancel, &QPushButton::clicked, this, [this]() {
        m_response = m_defaultValue;
        reject();
    });
    connect(submit, &QPushButton::clicked, this, [this]() {
        const QStringList entries = resourceEntries();
        if (m_mustExist) {
            for (const QString &entry : entries) {
                QFileInfo info(entry);
                if (info.isRelative() && !m_baseDir.isEmpty())
                    info = QFileInfo(QDir(m_baseDir).filePath(entry));
                const bool ok = (m_resourceKind == QStringLiteral("directory")) ? info.exists() && info.isDir() : info.exists();
                if (!ok) {
                    QMessageBox::warning(this, tr("DockPipe Prompt"),
                                         (m_resourceKind == QStringLiteral("directory"))
                                             ? tr("Choose an existing directory before continuing.")
                                             : tr("Choose an existing file before continuing."));
                    return;
                }
            }
        }
        m_response = resourceResponse();
        accept();
    });

    outer->addWidget(inputFrame);
    if (m_input)
        m_input->setFocus();
}

void PromptDialog::buildConfirmUi()
{
    auto *outer = qobject_cast<QVBoxLayout *>(layout());
    auto *actionsFrame = new QFrame(this);
    actionsFrame->setObjectName(QStringLiteral("promptActionPanel"));
    auto *actionsLay = new QHBoxLayout(actionsFrame);
    actionsLay->setContentsMargins(16, 16, 16, 16);
    actionsLay->setSpacing(10);
    actionsLay->addStretch(1);

    const QString yesLabel = tr("Yes");
    const QString noLabel = tr("No");

    auto *negative = makeActionButton(noLabel, actionsFrame);
    auto *positive = makeActionButton(yesLabel, actionsFrame);
    negative->setObjectName(QStringLiteral("secondaryButton"));
    positive->setObjectName(QStringLiteral("primaryButton"));
    actionsLay->addWidget(negative);
    actionsLay->addWidget(positive);

    connect(negative, &QPushButton::clicked, this, [this, noLabel]() {
        m_response = noLabel.toCaseFolded() == m_defaultValue.toCaseFolded() ? m_defaultValue : QStringLiteral("no");
        reject();
    });
    connect(positive, &QPushButton::clicked, this, [this]() {
        m_response = QStringLiteral("yes");
        accept();
    });

    outer->addWidget(actionsFrame);
}
