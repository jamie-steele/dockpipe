#include "Theme.h"

#include <QApplication>
#include <QColor>
#include <QDir>
#include <QFile>
#include <QGuiApplication>
#include <QIODevice>
#include <QPalette>
#include <QProcess>
#include <QString>
#include <QStyle>
#include <QTextStream>
#include <QtGlobal>
#include <cstdlib>
#include <optional>

#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
#include <QStyleHints>
#endif

namespace {

QString loadResource(const QString &path)
{
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return {};
    return QString::fromUtf8(f.readAll());
}

/// Fusion-friendly dark palette so palette(base/window/…) match pipeon-dark.qss.
QPalette fusionDarkPalette()
{
    QPalette p;
    const QColor win(48, 48, 48);
    const QColor base(40, 40, 40);
    const QColor alt(56, 56, 56);
    const QColor text(235, 235, 235);
    const QColor mid(120, 120, 120);
    const QColor highlight(42, 130, 218);

    p.setColor(QPalette::Window, win);
    p.setColor(QPalette::WindowText, text);
    p.setColor(QPalette::Base, base);
    p.setColor(QPalette::AlternateBase, alt);
    p.setColor(QPalette::Text, text);
    p.setColor(QPalette::Button, win);
    p.setColor(QPalette::ButtonText, text);
    p.setColor(QPalette::BrightText, QColor(255, 100, 100));
    p.setColor(QPalette::Link, highlight);
    p.setColor(QPalette::Highlight, highlight);
    p.setColor(QPalette::HighlightedText, Qt::white);
    p.setColor(QPalette::PlaceholderText, QColor(160, 160, 160));
    p.setColor(QPalette::ToolTipBase, alt);
    p.setColor(QPalette::ToolTipText, text);
    p.setColor(QPalette::Light, alt.lighter(120));
    p.setColor(QPalette::Midlight, QColor(80, 80, 80));
    p.setColor(QPalette::Mid, mid);
    p.setColor(QPalette::Dark, QColor(30, 30, 30));
    p.setColor(QPalette::Shadow, QColor(20, 20, 20));
    return p;
}

std::optional<bool> tryGnomeColorScheme()
{
    QProcess proc;
    proc.start(QStringLiteral("gsettings"),
               {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"), QStringLiteral("color-scheme")});
    if (!proc.waitForFinished(400) || proc.exitCode() != 0)
        return std::nullopt;
    QString s = QString::fromUtf8(proc.readAllStandardOutput()).trimmed();
    s.remove(QLatin1Char('\''));
    if (s == QStringLiteral("prefer-dark"))
        return true;
    if (s == QStringLiteral("prefer-light"))
        return false;
    return std::nullopt;
}

std::optional<bool> tryGnomeGtkThemeDark()
{
    QProcess proc;
    proc.start(QStringLiteral("gsettings"),
               {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"), QStringLiteral("gtk-theme")});
    if (!proc.waitForFinished(400) || proc.exitCode() != 0)
        return std::nullopt;
    const QString t = QString::fromUtf8(proc.readAllStandardOutput()).trimmed().remove(QLatin1Char('\'')).toLower();
    if (t.contains(QStringLiteral("dark")) || t.endsWith(QStringLiteral("-dark")))
        return true;
    return std::nullopt;
}

bool gtkSettingsIniPreferDark()
{
    const QString path = QDir::homePath() + QStringLiteral("/.config/gtk-3.0/settings.ini");
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;
    QTextStream in(&f);
    bool inSettings = false;
    while (!in.atEnd()) {
        QString line = in.readLine().trimmed();
        if (line == QStringLiteral("[Settings]"))
            inSettings = true;
        else if (line.startsWith(QLatin1Char('[')))
            inSettings = false;
        if (inSettings && line.startsWith(QStringLiteral("gtk-application-prefer-dark-theme=")))
            return line.section(QLatin1Char('='), 1).trimmed() == QStringLiteral("1");
    }
    return false;
}

bool kdeGlobalsPreferDark()
{
    const QString path = QDir::homePath() + QStringLiteral("/.config/kdeglobals");
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;
    QTextStream in(&f);
    bool inGeneral = false;
    while (!in.atEnd()) {
        QString line = in.readLine().trimmed();
        if (line == QStringLiteral("[General]"))
            inGeneral = true;
        else if (line.startsWith(QLatin1Char('[')))
            inGeneral = false;
        if (inGeneral && line.startsWith(QStringLiteral("ColorScheme="))) {
            const QString scheme = line.section(QLatin1Char('='), 1).trimmed().toLower();
            return scheme.contains(QStringLiteral("dark"));
        }
    }
    return false;
}

bool gtkThemeEnvDark()
{
    const char *gtk = std::getenv("GTK_THEME");
    if (!gtk)
        return false;
    const QString t = QString::fromLocal8Bit(gtk).toLower();
    return t.contains(QStringLiteral("dark"));
}

bool isDarkUi()
{
#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
    switch (QGuiApplication::styleHints()->colorScheme()) {
    case Qt::ColorScheme::Dark:
        return true;
    case Qt::ColorScheme::Light:
        return false;
    default:
        break;
    }
#endif

#ifdef Q_OS_LINUX
    if (const auto g = tryGnomeColorScheme(); g.has_value())
        return *g;
    if (gtkSettingsIniPreferDark())
        return true;
    if (kdeGlobalsPreferDark())
        return true;
    if (const auto g = tryGnomeGtkThemeDark(); g.has_value() && *g)
        return true;
    if (gtkThemeEnvDark())
        return true;
#endif

    // Last resort (e.g. qt6ct / custom platform theme with a dark palette). Fusion alone often stays light at startup on Linux.
    const QColor bg = QGuiApplication::palette().color(QPalette::Window);
    return bg.lightnessF() < 0.45;
}

} // namespace

void applyPipeonTheme(QApplication &app)
{
    QStyle *st = app.style();
    const bool dark = isDarkUi();

    if (dark)
        app.setPalette(fusionDarkPalette());
    else if (st)
        app.setPalette(st->standardPalette());
    else
        app.setPalette(QPalette());

    QString sheet = loadResource(QStringLiteral(":/theme/pipeon.qss"));
    sheet += loadResource(dark ? QStringLiteral(":/theme/pipeon-dark.qss")
                               : QStringLiteral(":/theme/pipeon-light.qss"));
    app.setStyleSheet(sheet);
}

void connectPipeonThemeUpdates(QApplication &app)
{
#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
    QObject::connect(QGuiApplication::styleHints(), &QStyleHints::colorSchemeChanged, &app,
                       [&app]() { applyPipeonTheme(app); });
#else
    Q_UNUSED(app);
#endif
}
