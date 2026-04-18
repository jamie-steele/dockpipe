#include "MainWindow.h"
#include "SingleInstanceGuard.h"
#include "Theme.h"

#include <QApplication>
#include <QIcon>
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

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    QApplication::setApplicationName(QStringLiteral("dockpipe-launcher"));
    QApplication::setApplicationDisplayName(QStringLiteral("DockPipe Launcher"));
    QApplication::setOrganizationName(QStringLiteral("dockpipe"));
    app.setDesktopFileName(QStringLiteral("dockpipe-launcher"));
    app.setWindowIcon(QIcon(QStringLiteral(":/icon.png")));

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
