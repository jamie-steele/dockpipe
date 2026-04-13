#include "PackageManagerDialog.h"

#include "DockpipeChoices.h"

#include <QAbstractItemView>
#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QFileInfo>
#include <QHeaderView>
#include <QLineEdit>
#include <QLabel>
#include <QFrame>
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
    QStringList pills;
    for (const QString &value : values)
        pills << QStringLiteral("<span style=\"display:inline-block;margin-right:6px;margin-bottom:6px;padding:3px 9px;border-radius:999px;background:#2f333b;border:1px solid #3a404a;\">%1</span>")
                     .arg(value.toHtmlEscaped());
    return QStringLiteral("<p><b>%1</b></p><p>%2</p>").arg(label.toHtmlEscaped(), pills.join(QStringLiteral(" ")));
}

QString detailsHtml(const PackageRow &row)
{
    QString html;
    html += QStringLiteral("<div style=\"font-family:sans-serif;line-height:1.5;\">");
    html += QStringLiteral("<h2 style=\"margin:0 0 6px;\">%1</h2>").arg(row.title.toHtmlEscaped());
    html += QStringLiteral("<p style=\"margin:0 0 12px;color:#b9c0cb;\">%1</p>").arg(row.name.toHtmlEscaped());
    html += QStringLiteral("<p>");
    if (!row.version.isEmpty())
        html += QStringLiteral("<span style=\"display:inline-block;margin-right:6px;margin-bottom:6px;padding:4px 10px;border-radius:999px;background:#2f333b;border:1px solid #3a404a;\"><b>Version</b> %1</span> ").arg(row.version.toHtmlEscaped());
    if (!row.kind.isEmpty())
        html += QStringLiteral("<span style=\"display:inline-block;margin-right:6px;margin-bottom:6px;padding:4px 10px;border-radius:999px;background:#2f333b;border:1px solid #3a404a;\"><b>Kind</b> %1</span> ").arg(row.kind.toHtmlEscaped());
    if (row.authoring)
        html += QStringLiteral("<span style=\"display:inline-block;margin-right:6px;margin-bottom:6px;padding:4px 10px;border-radius:999px;background:rgba(27,153,255,0.16);border:1px solid rgba(27,153,255,0.32);\"><b>Authoring</b></span> ");
    if (row.installed)
        html += QStringLiteral("<span style=\"display:inline-block;margin-right:6px;margin-bottom:6px;padding:4px 10px;border-radius:999px;background:rgba(46,160,67,0.16);border:1px solid rgba(46,160,67,0.32);\"><b>Installed</b></span> ");
    html += QStringLiteral("</p>");
    if (!row.description.isEmpty())
        html += QStringLiteral("<p style=\"margin:0 0 14px;\">%1</p>").arg(row.description.toHtmlEscaped());
    html += QStringLiteral("<p><b>Manifest</b><br><code>%1</code></p>").arg(row.packagePath.toHtmlEscaped());
    if (!row.author.isEmpty())
        html += QStringLiteral("<p><b>Author</b><br>%1</p>").arg(row.author.toHtmlEscaped());
    if (!row.provider.isEmpty())
        html += QStringLiteral("<p><b>Provider</b><br>%1</p>").arg(row.provider.toHtmlEscaped());
    if (!row.capability.isEmpty())
        html += QStringLiteral("<p><b>Capability</b><br>%1</p>").arg(row.capability.toHtmlEscaped());
    html += listLine(QObject::tr("Tags"), row.tags);
    html += listLine(QObject::tr("Depends"), row.depends);
    html += listLine(QObject::tr("Includes Resolvers"), row.includesResolvers);
    if (!row.repository.isEmpty()) {
        const QString repo = row.repository.toHtmlEscaped();
        html += QStringLiteral("<p><b>Repository</b><br><a href=\"%1\">%1</a></p>").arg(repo);
    }
    html += QStringLiteral("</div>");
    return html;
}

bool packageMatchesFilter(const PackageRow &row, const QString &filter)
{
    const QString needle = filter.trimmed().toCaseFolded();
    if (needle.isEmpty())
        return true;
    const QString haystack = QStringList{row.name,
                                         row.title,
                                         row.version,
                                         row.kind,
                                         row.provider,
                                         row.capability,
                                         row.description,
                                         row.author,
                                         row.repository,
                                         row.packagePath,
                                         row.source,
                                         row.tags.join(QLatin1Char(' ')),
                                         row.depends.join(QLatin1Char(' ')),
                                         row.includesResolvers.join(QLatin1Char(' '))}
                                 .join(QLatin1Char('\n'))
                                 .toCaseFolded();
    return haystack.contains(needle);
}

QLabel *metricPill(const QString &text)
{
    auto *label = new QLabel(text);
    label->setObjectName(QStringLiteral("dockerMetric"));
    label->setAlignment(Qt::AlignCenter);
    return label;
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
    layout->setSpacing(12);

    auto *hero = new QFrame(this);
    hero->setObjectName(QStringLiteral("dockerHero"));
    auto *heroLay = new QVBoxLayout(hero);
    heroLay->setContentsMargins(14, 14, 14, 14);
    heroLay->setSpacing(10);

    auto *title = new QLabel(tr("Packages"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *subtitle = new QLabel(
        tr("Installed shows the local packages already present here. Marketplace is reserved for the future remote store."));
    subtitle->setObjectName(QStringLiteral("appSubtitle"));
    subtitle->setWordWrap(true);
    heroLay->addWidget(title);
    heroLay->addWidget(subtitle);

    auto *topRow = new QHBoxLayout;
    m_search = new QLineEdit(this);
    m_search->setPlaceholderText(tr("Search packages…"));
    connect(m_search, &QLineEdit::textChanged, this, &PackageManagerDialog::onSearchChanged);
    topRow->addWidget(m_search, 1);
    heroLay->addLayout(topRow);

    auto *metrics = new QHBoxLayout;
    metrics->setSpacing(8);
    m_installedCount = metricPill(tr("Installed 0"));
    m_marketplaceCount = metricPill(tr("Marketplace 0"));
    metrics->addWidget(m_installedCount);
    metrics->addWidget(m_marketplaceCount);
    metrics->addStretch(1);
    heroLay->addLayout(metrics);

    layout->addWidget(hero);

    auto *splitter = new QSplitter(this);
    splitter->setOrientation(Qt::Horizontal);

    m_tabs = new QTabWidget(splitter);
    m_tabs->setObjectName(QStringLiteral("surfaceTabs"));
    m_installedTable = new QTableWidget(m_tabs);
    m_marketplaceTable = new QTableWidget(m_tabs);
    m_installedTable->setObjectName(QStringLiteral("dockerTable"));
    m_marketplaceTable->setObjectName(QStringLiteral("dockerTable"));
    configureTable(m_installedTable);
    configureTable(m_marketplaceTable);
    m_tabs->addTab(m_installedTable, tr("Installed"));
    m_tabs->addTab(m_marketplaceTable, tr("Marketplace"));

    m_details = new QTextBrowser(splitter);
    m_details->setObjectName(QStringLiteral("detailBrowser"));
    m_details->setOpenExternalLinks(true);
    m_details->setPlaceholderText(tr("Select a package to inspect its metadata."));
    m_details->setOpenLinks(false);

    splitter->addWidget(m_tabs);
    splitter->addWidget(m_details);
    splitter->setStretchFactor(0, 3);
    splitter->setStretchFactor(1, 2);

    layout->addWidget(splitter, 1);

    connect(m_installedTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::onInstalledSelectionChanged);
    connect(m_marketplaceTable, &QTableWidget::itemSelectionChanged, this, &PackageManagerDialog::onMarketplaceSelectionChanged);
    connect(m_tabs, &QTabWidget::currentChanged, this, [this](int) {
        if (m_tabs->currentWidget() == m_installedTable)
            refreshDetails(m_installedTable);
        else if (m_tabs->currentWidget() == m_marketplaceTable)
            refreshDetails(m_marketplaceTable);
    });
}

void PackageManagerDialog::loadPackages()
{
    applyFilter();
    onInstalledSelectionChanged();
}

void PackageManagerDialog::applyFilter()
{
    const QVector<PackageRow> all = discoverPackages(m_hintWorkdir);
    QVector<PackageRow> filtered;
    for (const PackageRow &row : all) {
        if (packageMatchesFilter(row, m_search ? m_search->text() : QString()))
            filtered.append(row);
    }
    const QVector<PackageRow> installed = selectedRows(filtered, true, false);
    populateTable(m_installedTable, installed, tr("Installed"));
    populateTable(m_marketplaceTable, {}, tr("Available"));
    if (m_installedCount)
        m_installedCount->setText(tr("Installed %1").arg(installed.size()));
    if (m_marketplaceCount)
        m_marketplaceCount->setText(tr("Marketplace 0"));
}

void PackageManagerDialog::refreshDetails(QTableWidget *table)
{
    const QVector<PackageRow> all = discoverPackages(m_hintWorkdir);
    const int rowIndex = table->currentRow();
    if (rowIndex < 0 || !table->item(rowIndex, 0)) {
        m_details->setHtml(tr("<div style=\"font-family:sans-serif;\"><h2>Package details</h2><p>Select a package to inspect it.</p></div>"));
        return;
    }
    const QString packagePath = table->item(rowIndex, 0)->data(Qt::UserRole).toString();
    for (const PackageRow &row : all) {
        if (row.packagePath == packagePath) {
            m_details->setHtml(detailsHtml(row));
            return;
        }
    }
    m_details->setHtml(tr("<div style=\"font-family:sans-serif;\"><p>Package metadata could not be loaded.</p></div>"));
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
            m_details->setHtml(tr("<div style=\"font-family:sans-serif;\"><h2>Marketplace</h2><p>No remote package store is connected yet.</p><p>This tab is intentionally empty until a remote catalog is wired in.</p></div>"));
            return;
        }
        refreshDetails(m_marketplaceTable);
    }
}

void PackageManagerDialog::onSearchChanged(const QString &)
{
    applyFilter();
    if (m_tabs->currentWidget() == m_marketplaceTable)
        onMarketplaceSelectionChanged();
    else
        onInstalledSelectionChanged();
}
