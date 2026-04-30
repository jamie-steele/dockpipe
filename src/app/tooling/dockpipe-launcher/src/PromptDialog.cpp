#include "PromptDialog.h"

#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
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
      m_options(spec.options), m_sensitive(spec.sensitive)
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
