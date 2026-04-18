#include "MainWindow.h"
#include "SingleInstanceGuard.h"
#include "Theme.h"

#include <QApplication>
#include <QIcon>
#include <QProcessEnvironment>
#include <QSize>
#include <QStyleFactory>
#include <cstring>

static bool allowSecondInstance(int argc, char *argv[])
{
    for (int i = 1; i < argc; ++i) {
        if (std::strcmp(argv[i], "--allow-second-instance") == 0)
            return true;
    }
    return false;
}

static QIcon dockpipeLauncherIcon()
{
    QIcon icon;
    const struct {
        const char *path;
        int size;
    } sizes[] = {
        {":/icons/hicolor/16x16/apps/dockpipe-launcher.png", 16},
        {":/icons/hicolor/24x24/apps/dockpipe-launcher.png", 24},
        {":/icons/hicolor/32x32/apps/dockpipe-launcher.png", 32},
        {":/icons/hicolor/48x48/apps/dockpipe-launcher.png", 48},
        {":/icons/hicolor/64x64/apps/dockpipe-launcher.png", 64},
        {":/icons/hicolor/96x96/apps/dockpipe-launcher.png", 96},
        {":/icons/hicolor/128x128/apps/dockpipe-launcher.png", 128},
        {":/icons/hicolor/256x256/apps/dockpipe-launcher.png", 256},
        {":/icons/hicolor/512x512/apps/dockpipe-launcher.png", 512},
    };
    for (const auto &entry : sizes)
        icon.addFile(QString::fromUtf8(entry.path), QSize(entry.size, entry.size));
    if (icon.isNull())
        icon = QIcon(QStringLiteral(":/icon.png"));
    return icon;
}

int main(int argc, char *argv[])
{
    const QProcessEnvironment startupEnv = QProcessEnvironment::systemEnvironment();
#if defined(Q_OS_LINUX)
    if (startupEnv.value(QStringLiteral("XDG_SESSION_TYPE")).compare(QStringLiteral("x11"), Qt::CaseInsensitive) == 0) {
        qunsetenv("DESKTOP_STARTUP_ID");
        qunsetenv("XDG_ACTIVATION_TOKEN");
    }
#endif

    QApplication app(argc, argv);
    QApplication::setApplicationName(QStringLiteral("dockpipe-launcher"));
    QApplication::setApplicationDisplayName(QStringLiteral("DockPipe Launcher"));
    QApplication::setOrganizationName(QStringLiteral("dockpipe"));
    app.setDesktopFileName(QStringLiteral("dockpipe-launcher"));
    app.setWindowIcon(dockpipeLauncherIcon());

    app.setStyle(QStyleFactory::create(QStringLiteral("Fusion")));

    applyDockpipeLauncherTheme(app);
    connectDockpipeLauncherThemeUpdates(app);

    app.setQuitOnLastWindowClosed(false);

    const bool startHome = app.arguments().contains(QStringLiteral("--start-home"));
    const bool allowSecond = allowSecondInstance(argc, argv);
    SingleInstanceGuard singleGuard;
    if (!allowSecond && !singleGuard.tryRunPrimaryInstance(startHome))
        return 0;

    MainWindow w;
    if (!allowSecond)
        singleGuard.setActivationTarget(&w);
    w.show();
    return app.exec();
}
