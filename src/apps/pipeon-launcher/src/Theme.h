#pragma once

class QApplication;

/// Loads base + light/dark companion stylesheets from system color scheme (with fallbacks).
void applyPipeonTheme(QApplication &app);

/// Re-applies when the platform color scheme changes (Qt 6.5+).
void connectPipeonThemeUpdates(QApplication &app);
