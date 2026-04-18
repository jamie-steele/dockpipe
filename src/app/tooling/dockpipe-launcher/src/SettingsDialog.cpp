#include "SettingsDialog.h"

#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFormLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QVBoxLayout>

namespace {

QWidget *pathRow(QLineEdit *edit, const QString &buttonText, QObject *receiver, const char *slot)
{
    auto *row = new QWidget;
    auto *layout = new QHBoxLayout(row);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(8);
    auto *button = new QPushButton(buttonText);
    QObject::connect(button, SIGNAL(clicked()), receiver, slot);
    layout->addWidget(edit, 1);
    layout->addWidget(button, 0);
    return row;
}

} // namespace

SettingsDialog::SettingsDialog(const LauncherSettings &settings, QWidget *parent)
    : QDialog(parent), m_settings(settings)
{
    setWindowTitle(tr("Settings"));
    resize(760, 520);

    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(14, 14, 14, 14);
    layout->setSpacing(10);

    auto *intro = new QLabel(
        tr("Configure launcher defaults for package sources and repo discovery. Environment variables still win, and opening a folder inside a repo continues to override the repo-root fallback automatically."));
    intro->setWordWrap(true);
    layout->addWidget(intro);

    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);

    m_repoRootOverride = new QLineEdit(m_settings.repoRootOverride);
    m_repoRootOverride->setPlaceholderText(tr("Optional fallback repo root"));
    form->addRow(tr("Repo Root Fallback"), pathRow(m_repoRootOverride, tr("Browse…"), this, SLOT(browseRepoRoot())));

    m_globalRootOverride = new QLineEdit(m_settings.globalRootOverride);
    m_globalRootOverride->setPlaceholderText(tr("Optional global DockPipe data root"));
    form->addRow(tr("Global Root Override"), pathRow(m_globalRootOverride, tr("Browse…"), this, SLOT(browseGlobalRoot())));

    m_extraWorkflowRoots = new QPlainTextEdit(joinLines(m_settings.extraWorkflowRoots));
    m_extraWorkflowRoots->setPlaceholderText(tr("One workflow root per line"));
    m_extraWorkflowRoots->setTabChangesFocus(true);
    form->addRow(tr("Extra Workflow Roots"), m_extraWorkflowRoots);

    m_packageRemotes = new QPlainTextEdit(joinLines(m_settings.packageRemotes));
    m_packageRemotes->setPlaceholderText(tr("One remote package store URL per line"));
    m_packageRemotes->setTabChangesFocus(true);
    form->addRow(tr("Package Remotes"), m_packageRemotes);

    layout->addLayout(form, 1);

    auto *notes = new QLabel(
        tr("Precedence: environment variables first, then the repo discovered from the opened folder, then these saved defaults."));
    notes->setWordWrap(true);
    layout->addWidget(notes);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
}

LauncherSettings SettingsDialog::updatedSettings() const
{
    LauncherSettings out = m_settings;
    out.repoRootOverride = m_repoRootOverride->text().trimmed();
    out.globalRootOverride = m_globalRootOverride->text().trimmed();
    out.extraWorkflowRoots = splitLines(m_extraWorkflowRoots->toPlainText());
    out.packageRemotes = splitLines(m_packageRemotes->toPlainText());
    return out;
}

void SettingsDialog::browseRepoRoot()
{
    const QString dir = QFileDialog::getExistingDirectory(this, tr("Choose repo root"), m_repoRootOverride->text());
    if (!dir.isEmpty())
        m_repoRootOverride->setText(dir);
}

void SettingsDialog::browseGlobalRoot()
{
    const QString dir = QFileDialog::getExistingDirectory(this, tr("Choose global root"), m_globalRootOverride->text());
    if (!dir.isEmpty())
        m_globalRootOverride->setText(dir);
}

QStringList SettingsDialog::splitLines(const QString &text)
{
    QStringList out;
    for (const QString &line : text.split(QLatin1Char('\n'))) {
        const QString trimmed = line.trimmed();
        if (!trimmed.isEmpty())
            out.append(trimmed);
    }
    return out;
}

QString SettingsDialog::joinLines(const QStringList &values)
{
    return values.join(QLatin1Char('\n'));
}
