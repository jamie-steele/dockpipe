#include "PackageManagerDialog.h"

#include "DockpipeChoices.h"

#include <QAbstractItemView>
#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QFileInfo>
#include <QHeaderView>
#include <QLabel>
#include <QSplitter>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QTextBrowser>
#include <QVBoxLayout>

namespace {

struct PackageRow {
    QString name;
    QString title;
    QString version;
    QString kind;
    QString provider;
    QString capability;
    QString description;
    QString author;
    QString repository;
    QString packagePath;
    QString source;
    QStringList tags;
    QStringList depends;
    QStringList includesResolvers;
    bool installed = false;
    bool authoring = false;
};

QString stripInlineComment(QString line)
{
    const int hash = line.indexOf(QLatin1Char('#'));
    if (hash >= 0)
        line = line.left(hash);
    return line;
}

QString unquote(QString s)
{
    s = s.trimmed();
    if ((s.startsWith(QLatin1Char('"')) && s.endsWith(QLatin1Char('"')))
        || (s.startsWith(QLatin1Char('\'')) && s.endsWith(QLatin1Char('\'')))) {
        s = s.mid(1, s.size() - 2);
    }
    return s.trimmed();
}

QString scalarAfterKey(const QString &trimmed, const QString &raw, const QString &key)
{
    if (!trimmed.startsWith(key + QLatin1Char(':')))
        return {};
    return raw.mid(raw.indexOf(QLatin1Char(':')) + 1).trimmed();
}

QStringList parseInlineList(QString text)
{
    QString s = text.trimmed();
    if (s.startsWith(QLatin1Char('[')) && s.endsWith(QLatin1Char(']')))
        s = s.mid(1, s.size() - 2);
    QStringList out;
    for (const QString &part : s.split(QLatin1Char(','), Qt::SkipEmptyParts))
        out.append(unquote(part));
    out.removeAll(QString());
    return out;
}

PackageRow parsePackageManifest(const QString &path)
{
    PackageRow row;
    row.packagePath = QDir::cleanPath(path);
    row.name = QFileInfo(path).absoluteDir().dirName();
    row.title = row.name;
    row.kind = QStringLiteral("package");
    row.source = QStringLiteral("Local package");
    row.installed = true;
    row.authoring = true;

    QFile file(path);
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text))
        return row;

    const QStringList lines = QString::fromUtf8(file.readAll()).split(QLatin1Char('\n'));
    bool inDescription = false;
    bool inTags = false;
    bool inDepends = false;
    bool inIncludesResolvers = false;
    QStringList descriptionLines;

    auto flushDescription = [&]() {
        if (!descriptionLines.isEmpty()) {
            row.description = descriptionLines.join(QLatin1Char(' ')).trimmed();
            descriptionLines.clear();
        }
    };

    for (const QString &original : lines) {
        QString raw = stripInlineComment(original);
        const QString trimmed = raw.trimmed();
        if (trimmed.isEmpty()) {
            if (inDescription && !descriptionLines.isEmpty())
                descriptionLines.append(QString());
            continue;
        }

        const bool indented = original.startsWith(QLatin1Char(' ')) || original.startsWith(QLatin1Char('\t'));
        if (!indented) {
            if (inDescription)
                flushDescription();
            inDescription = false;
            inTags = false;
            inDepends = false;
            inIncludesResolvers = false;
        }

        if (inDescription && indented) {
            descriptionLines.append(trimmed);
            continue;
        }
        if (inTags && trimmed.startsWith(QLatin1Char('-'))) {
            row.tags.append(unquote(trimmed.mid(1)));
            continue;
        }
        if (inDepends && trimmed.startsWith(QLatin1Char('-'))) {
            row.depends.append(unquote(trimmed.mid(1)));
            continue;
        }
        if (inIncludesResolvers && trimmed.startsWith(QLatin1Char('-'))) {
            row.includesResolvers.append(unquote(trimmed.mid(1)));
            continue;
        }

        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("name")); !v.isEmpty()) {
            row.name = unquote(v);
            if (row.title.isEmpty() || row.title == QFileInfo(path).absoluteDir().dirName())
                row.title = row.name;
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("title")); !v.isEmpty()) {
            row.title = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("version")); !v.isEmpty()) {
            row.version = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("kind")); !v.isEmpty()) {
            row.kind = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("provider")); !v.isEmpty()) {
            row.provider = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("capability")); !v.isEmpty()) {
            row.capability = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("author")); !v.isEmpty()) {
            row.author = unquote(v);
            continue;
        }
        if (const QString v = scalarAfterKey(trimmed, raw, QStringLiteral("repository")); !v.isEmpty()) {
            row.repository = unquote(v);
            continue;
        }
        if (trimmed.startsWith(QStringLiteral("description:"))) {
            const QString rest = raw.mid(raw.indexOf(QLatin1Char(':')) + 1).trimmed();
            if (rest == QStringLiteral("|") || rest == QStringLiteral(">-") || rest == QStringLiteral(">")) {
                inDescription = true;
                continue;
            }
            row.description = unquote(rest);
            continue;
        }
        if (trimmed.startsWith(QStringLiteral("tags:"))) {
            const QString rest = raw.mid(raw.indexOf(QLatin1Char(':')) + 1).trimmed();
            if (rest.startsWith(QLatin1Char('['))) {
                row.tags = parseInlineList(rest);
            } else {
                inTags = true;
            }
            continue;
        }
        if (trimmed.startsWith(QStringLiteral("depends:"))) {
            const QString rest = raw.mid(raw.indexOf(QLatin1Char(':')) + 1).trimmed();
            if (rest.startsWith(QLatin1Char('['))) {
                row.depends = parseInlineList(rest);
            } else {
                inDepends = true;
            }
            continue;
        }
        if (trimmed.startsWith(QStringLiteral("includes_resolvers:"))) {
            const QString rest = raw.mid(raw.indexOf(QLatin1Char(':')) + 1).trimmed();
            if (rest.startsWith(QLatin1Char('['))) {
                row.includesResolvers = parseInlineList(rest);
            } else {
                inIncludesResolvers = true;
            }
            continue;
        }
    }

    flushDescription();
    if (row.title.trimmed().isEmpty())
        row.title = row.name;
    return row;
}

QVector<PackageRow> discoverPackages(const QString &hintWorkdir)
{
    QVector<PackageRow> out;
    const QString repoRoot = DockpipeChoices::findRepoRoot(hintWorkdir);
    if (repoRoot.isEmpty())
        return out;

    const QDir packagesRoot(QDir(repoRoot).filePath(QStringLiteral("packages")));
    if (!packagesRoot.exists())
        return out;

    QDirIterator it(packagesRoot.path(), QStringList{QStringLiteral("package.yml")}, QDir::Files, QDirIterator::Subdirectories);
    while (it.hasNext()) {
        it.next();
        PackageRow row = parsePackageManifest(it.filePath());
        out.append(row);
    }

    std::sort(out.begin(), out.end(), [](const PackageRow &a, const PackageRow &b) {
        return a.title.localeAwareCompare(b.title) < 0;
    });
    return out;
}

void configureTable(QTableWidget *table)
{
    table->setColumnCount(5);
    table->setHorizontalHeaderLabels(
        QStringList{QObject::tr("Name"), QObject::tr("Version"), QObject::tr("Kind"), QObject::tr("Source"),
                    QObject::tr("Status")});
    table->setSelectionBehavior(QAbstractItemView::SelectRows);
    table->setSelectionMode(QAbstractItemView::SingleSelection);
    table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    table->setAlternatingRowColors(true);
    table->verticalHeader()->setVisible(false);
    table->horizontalHeader()->setStretchLastSection(true);
    table->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);
}

void populateTable(QTableWidget *table, const QVector<PackageRow> &rows, const QString &status)
{
    table->setRowCount(rows.size());
    for (int i = 0; i < rows.size(); ++i) {
        const PackageRow &row = rows[i];
        auto *name = new QTableWidgetItem(row.title);
        name->setData(Qt::UserRole, row.packagePath);
        name->setToolTip(row.packagePath);
        table->setItem(i, 0, name);
        table->setItem(i, 1, new QTableWidgetItem(row.version));
        table->setItem(i, 2, new QTableWidgetItem(row.kind));
        table->setItem(i, 3, new QTableWidgetItem(row.source));
        table->setItem(i, 4, new QTableWidgetItem(status));
    }
    if (table->rowCount() > 0)
        table->selectRow(0);
}

QVector<PackageRow> selectedRows(const QVector<PackageRow> &rows, bool installedOnly, bool authoringOnly)
{
    QVector<PackageRow> out;
    for (const PackageRow &row : rows) {
        if (installedOnly && !row.installed)
            continue;
        if (authoringOnly && !row.authoring)
            continue;
        out.append(row);
    }
    return out;
}

QString listLine(const QString &label, const QStringList &values)
{
    if (values.isEmpty())
        return {};
    return QStringLiteral("<p><b>%1:</b> %2</p>").arg(label.toHtmlEscaped(), values.join(QStringLiteral(", ")).toHtmlEscaped());
}

QString detailsHtml(const PackageRow &row)
{
    QString html;
    html += QStringLiteral("<h2>%1</h2>").arg(row.title.toHtmlEscaped());
    html += QStringLiteral("<p><b>Name:</b> %1</p>").arg(row.name.toHtmlEscaped());
    if (!row.version.isEmpty())
        html += QStringLiteral("<p><b>Version:</b> %1</p>").arg(row.version.toHtmlEscaped());
    if (!row.kind.isEmpty())
        html += QStringLiteral("<p><b>Kind:</b> %1</p>").arg(row.kind.toHtmlEscaped());
    if (!row.author.isEmpty())
        html += QStringLiteral("<p><b>Author:</b> %1</p>").arg(row.author.toHtmlEscaped());
    if (!row.provider.isEmpty())
        html += QStringLiteral("<p><b>Provider:</b> %1</p>").arg(row.provider.toHtmlEscaped());
    if (!row.capability.isEmpty())
        html += QStringLiteral("<p><b>Capability:</b> %1</p>").arg(row.capability.toHtmlEscaped());
    if (!row.description.isEmpty())
        html += QStringLiteral("<p>%1</p>").arg(row.description.toHtmlEscaped());
    html += listLine(QObject::tr("Tags"), row.tags);
    html += listLine(QObject::tr("Depends"), row.depends);
    html += listLine(QObject::tr("Includes Resolvers"), row.includesResolvers);
    if (!row.repository.isEmpty()) {
        const QString repo = row.repository.toHtmlEscaped();
        html += QStringLiteral("<p><b>Repository:</b> <a href=\"%1\">%1</a></p>").arg(repo);
    }
    html += QStringLiteral("<p><b>Manifest:</b> %1</p>").arg(row.packagePath.toHtmlEscaped());
    return html;
}

} // namespace

PackageManagerDialog::PackageManagerDialog(const QString &hintWorkdir, QWidget *parent)
    : QDialog(parent), m_hintWorkdir(hintWorkdir)
{
    setWindowTitle(tr("Package Manager"));
    resize(980, 640);
    buildUi();
    loadPackages();
}

void PackageManagerDialog::buildUi()
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(14, 14, 14, 14);
    layout->setSpacing(10);

    auto *title = new QLabel(
        tr("Browse packages for this checkout. Installed reflects the local packages already present here, Marketplace is reserved for a future remote store, and Authoring shows the package trees you are working on in this repo."));
    title->setWordWrap(true);
    layout->addWidget(title);

    auto *splitter = new QSplitter(this);
    splitter->setOrientation(Qt::Horizontal);

    m_tabs = new QTabWidget(splitter);
    m_installedTable = new QTableWidget(m_tabs);
    m_marketplaceTable = new QTableWidget(m_tabs);
    m_authoringTable = new QTableWidget(m_tabs);
    configureTable(m_installedTable);
    configureTable(m_marketplaceTable);
    configureTable(m_authoringTable);
    m_tabs->addTab(m_installedTable, tr("Installed"));
    m_tabs->addTab(m_marketplaceTable, tr("Marketplace"));
    m_tabs->addTab(m_authoringTable, tr("Authoring"));

    m_details = new QTextBrowser(splitter);
    m_details->setOpenExternalLinks(true);
    m_details->setPlaceholderText(tr("Select a package to inspect its metadata."));

    splitter->addWidget(m_tabs);
    splitter->addWidget(m_details);
    splitter->setStretchFactor(0, 3);
    splitter->setStretchFactor(1, 2);

    layout->addWidget(splitter, 1);

    connect(m_installedTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::onInstalledSelectionChanged);
    connect(m_marketplaceTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::onMarketplaceSelectionChanged);
    connect(m_authoringTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::onAuthoringSelectionChanged);
    connect(m_tabs, &QTabWidget::currentChanged, this, [this](int) {
        if (m_tabs->currentWidget() == m_installedTable)
            refreshDetails(m_installedTable);
        else if (m_tabs->currentWidget() == m_marketplaceTable)
            refreshDetails(m_marketplaceTable);
        else if (m_tabs->currentWidget() == m_authoringTable)
            refreshDetails(m_authoringTable);
    });
}

void PackageManagerDialog::loadPackages()
{
    const QVector<PackageRow> all = discoverPackages(m_hintWorkdir);
    populateTable(m_installedTable, selectedRows(all, true, false), tr("Installed"));
    populateTable(m_marketplaceTable, {}, tr("Available"));
    populateTable(m_authoringTable, selectedRows(all, false, true), tr("Authoring"));
    onInstalledSelectionChanged();
}

void PackageManagerDialog::refreshDetails(QTableWidget *table)
{
    const QVector<PackageRow> all = discoverPackages(m_hintWorkdir);
    const int rowIndex = table->currentRow();
    if (rowIndex < 0 || !table->item(rowIndex, 0)) {
        m_details->setHtml(tr("<p>Select a package to inspect it.</p>"));
        return;
    }
    const QString packagePath = table->item(rowIndex, 0)->data(Qt::UserRole).toString();
    for (const PackageRow &row : all) {
        if (row.packagePath == packagePath) {
            m_details->setHtml(detailsHtml(row));
            return;
        }
    }
    m_details->setHtml(tr("<p>Package metadata could not be loaded.</p>"));
}

void PackageManagerDialog::onInstalledSelectionChanged()
{
    if (m_tabs->currentWidget() == m_installedTable)
        refreshDetails(m_installedTable);
}

void PackageManagerDialog::onMarketplaceSelectionChanged()
{
    if (m_tabs->currentWidget() == m_marketplaceTable) {
        if (m_marketplaceTable->rowCount() == 0) {
            m_details->setHtml(tr("<h2>Marketplace</h2><p>No remote package store is connected yet.</p>"));
            return;
        }
        refreshDetails(m_marketplaceTable);
    }
}

void PackageManagerDialog::onAuthoringSelectionChanged()
{
    if (m_tabs->currentWidget() == m_authoringTable)
        refreshDetails(m_authoringTable);
}
