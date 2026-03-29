cmake_minimum_required(VERSION 3.16)
project(pipeon-launcher VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets)

set(PIPEON_LAUNCHER_SOURCES
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
    src/ContextRowWidget.cpp
    src/EditContextDialog.cpp
    src/MainWindow.cpp
    pipeon.qrc
)

qt_add_executable(pipeon-launcher ${PIPEON_LAUNCHER_SOURCES})

target_include_directories(pipeon-launcher PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
target_link_libraries(pipeon-launcher PRIVATE Qt6::Widgets)

if(MSVC)
    target_compile_definitions(pipeon-launcher PRIVATE _CRT_SECURE_NO_WARNINGS)
endif()
