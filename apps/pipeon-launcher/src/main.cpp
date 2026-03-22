#include "MainWindow.h"
#include "Theme.h"

#include <QApplication>
#include <QStyleFactory>

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    QApplication::setApplicationName(QStringLiteral("Pipeon"));
    QApplication::setApplicationDisplayName(QStringLiteral("Pipeon"));
    QApplication::setOrganizationName(QStringLiteral("pipeon"));

    app.setStyle(QStyleFactory::create(QStringLiteral("Fusion")));

    applyPipeonTheme(app);
    connectPipeonThemeUpdates(app);

    app.setQuitOnLastWindowClosed(false);

    MainWindow w;
    w.show();
    return app.exec();
}
