cmake_minimum_required(VERSION 3.16)
project(dockpipe-launcher VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets Network)

set(DOCKPIPE_LAUNCHER_SOURCES
    src/main.cpp
    src/SingleInstanceGuard.cpp
    src/Theme.cpp
    src/Context.cpp
    src/ContextStore.cpp
    src/GitHelper.cpp
    src/SessionManager.cpp
    src/DockpipeChoices.cpp
    src/ContextDiscovery.cpp
    src/WorkflowCatalog.cpp
    src/LauncherSettings.cpp
    src/BasicModeWidget.cpp
    src/PackageManagerDialog.cpp
    src/SettingsDialog.cpp
    src/LogViewerDialog.cpp
    src/PromptDialog.cpp
    src/DockerObservabilityWidget.cpp
    src/ContextRowWidget.cpp
    src/EditContextDialog.cpp
    src/MainWindow.cpp
    dockpipe-launcher.qrc
)

qt_add_executable(dockpipe-launcher ${DOCKPIPE_LAUNCHER_SOURCES})

target_include_directories(dockpipe-launcher PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
target_link_libraries(dockpipe-launcher PRIVATE Qt6::Widgets Qt6::Network)

if(MSVC)
    target_compile_definitions(dockpipe-launcher PRIVATE _CRT_SECURE_NO_WARNINGS)
endif()
