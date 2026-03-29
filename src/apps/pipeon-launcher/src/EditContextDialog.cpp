#include "EditContextDialog.h"
#include "DockpipeChoices.h"

#include <QComboBox>
#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFormLayout>
#include <QGroupBox>
#include <QHBoxLayout>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QVBoxLayout>

namespace {

void fillCombo(QComboBox *cb, const QStringList &items, const QString &current)
{
    cb->clear();
    cb->addItems(items);
    cb->setCurrentText(current);
    cb->setEditable(true);
    cb->setInsertPolicy(QComboBox::NoInsert);
}

} // namespace

EditContextDialog::EditContextDialog(const Context &ctx, QWidget *parent)
    : QDialog(parent)
    , m_original(ctx)
{
    setWindowTitle(tr("Edit context"));
    setMinimumWidth(520);

    m_label = new QLineEdit(ctx.label);
    m_workdir = new QLineEdit(ctx.workdir);
    m_workdir->setReadOnly(true);

    m_workflow = new QComboBox;
    m_workflowFile = new QComboBox;
    m_resolver = new QComboBox;
    m_strategy = new QComboBox;
    m_runtime = new QComboBox;

    m_dockpipe = new QLineEdit(ctx.dockpipeBinary);
    m_env = new QLineEdit(ctx.envFile);

    m_extraEnv = new QPlainTextEdit;
    m_extraEnv->setPlaceholderText(tr("One KEY=value per line, passed as dockpipe --env (e.g. OPENAI_API_KEY=…)"));
    m_extraEnv->setTabChangesFocus(true);
    m_extraEnv->setPlainText(ctx.extraDockpipeEnv.join(QLatin1Char('\n')));

    populateCombos(ctx.workdir);

    auto *general = new QGroupBox(tr("General"));
    auto *gForm = new QFormLayout(general);
    gForm->setSpacing(10);
    gForm->setHorizontalSpacing(16);
    gForm->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    gForm->addRow(tr("Label"), m_label);
    gForm->addRow(tr("Workdir"), m_workdir);

    auto *workflow = new QGroupBox(tr("Workflow"));
    auto *wForm = new QFormLayout(workflow);
    wForm->setSpacing(10);
    wForm->setHorizontalSpacing(16);
    wForm->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    wForm->addRow(tr("Workflow name"), m_workflow);

    auto *wfRow = new QWidget;
    auto *wfLay = new QHBoxLayout(wfRow);
    wfLay->setContentsMargins(0, 0, 0, 0);
    wfLay->setSpacing(8);
    wfLay->addWidget(m_workflowFile, 1);
    auto *wfBrowse = new QPushButton(tr("Browse…"));
    wfBrowse->setObjectName(QStringLiteral("secondaryButton"));
    wfLay->addWidget(wfBrowse);
    wForm->addRow(tr("Workflow file"), wfRow);

    auto *envRow = new QWidget;
    auto *envLay = new QHBoxLayout(envRow);
    envLay->setContentsMargins(0, 0, 0, 0);
    envLay->setSpacing(8);
    envLay->addWidget(m_env, 1);
    auto *envBrowse = new QPushButton(tr("Browse…"));
    envBrowse->setObjectName(QStringLiteral("secondaryButton"));
    envLay->addWidget(envBrowse);
    wForm->addRow(tr("Env file"), envRow);

    auto *extraRow = new QWidget;
    auto *extraLay = new QVBoxLayout(extraRow);
    extraLay->setContentsMargins(0, 0, 0, 0);
    extraLay->setSpacing(6);
    extraLay->addWidget(m_extraEnv);
    wForm->addRow(tr("Extra dockpipe env"), extraRow);

    auto *exec = new QGroupBox(tr("Execution"));
    auto *eForm = new QFormLayout(exec);
    eForm->setSpacing(10);
    eForm->setHorizontalSpacing(16);
    eForm->setFieldGrowthPolicy(QFormLayout::AllNonFixedFieldsGrow);
    eForm->addRow(tr("Resolver"), m_resolver);
    eForm->addRow(tr("Strategy"), m_strategy);
    eForm->addRow(tr("Runtime"), m_runtime);
    eForm->addRow(tr("dockpipe binary"), m_dockpipe);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setObjectName(QStringLiteral("primaryButton"));
    buttons->button(QDialogButtonBox::Cancel)->setObjectName(QStringLiteral("secondaryButton"));

    auto *root = new QVBoxLayout(this);
    root->setSpacing(14);
    root->addWidget(general);
    root->addWidget(workflow);
    root->addWidget(exec);
    root->addWidget(buttons);

    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    connect(wfBrowse, &QPushButton::clicked, this, &EditContextDialog::browseWorkflowFile);
    connect(envBrowse, &QPushButton::clicked, this, &EditContextDialog::browseEnvFile);
}

void EditContextDialog::populateCombos(const QString &workdir)
{
    DockpipeChoices ch;
    ch.scan(DockpipeChoices::findRepoRoot(workdir));

    fillCombo(m_workflow, ch.workflowNames, m_original.workflow);
    fillCombo(m_workflowFile, ch.workflowConfigPaths, m_original.workflowFile);
    fillCombo(m_resolver, ch.resolvers, m_original.resolver);
    fillCombo(m_strategy, ch.strategies, m_original.strategy);
    fillCombo(m_runtime, ch.runtimes, m_original.runtime);
}

void EditContextDialog::browseWorkflowFile()
{
    const QString path = QFileDialog::getOpenFileName(
        this, tr("Workflow file"), QString(),
        tr("YAML (*.yml *.yaml);;All files (*)"));
    if (!path.isEmpty())
        m_workflowFile->setCurrentText(path);
}

void EditContextDialog::browseEnvFile()
{
    const QString path = QFileDialog::getOpenFileName(this, tr("Env file"), QString(),
                                                      tr("Env (*.env *.sh);;All files (*)"));
    if (!path.isEmpty())
        m_env->setText(path);
}

Context EditContextDialog::editedContext() const
{
    Context c = m_original;
    c.label = m_label->text();
    c.workflow = m_workflow->currentText().trimmed();
    c.workflowFile = m_workflowFile->currentText().trimmed();
    c.resolver = m_resolver->currentText().trimmed();
    c.strategy = m_strategy->currentText().trimmed();
    c.runtime = m_runtime->currentText().trimmed();
    c.dockpipeBinary = m_dockpipe->text().trimmed();
    c.envFile = m_env->text().trimmed();
    c.extraDockpipeEnv.clear();
    const QStringList raw = m_extraEnv->toPlainText().split(QLatin1Char('\n'));
    for (QString line : raw) {
        line = line.trimmed();
        if (line.isEmpty() || line.startsWith(QLatin1Char('#')))
            continue;
        c.extraDockpipeEnv.append(line);
    }
    return c;
}
