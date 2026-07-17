#pragma once

#include "WorkflowCatalog.h"

#include <QDialog>
#include <QMap>

class QCheckBox;
class QComboBox;
class QFormLayout;
class QStackedWidget;
class QTabWidget;
class QVBoxLayout;

class WorkflowLaunchDialog : public QDialog {
    Q_OBJECT
public:
    explicit WorkflowLaunchDialog(const WorkflowMeta &workflow, const QMap<QString, QString> &currentValues,
                                  QWidget *parent = nullptr);

    QMap<QString, QString> values() const;

private:
    const WorkflowInputMeta *findInputByPath(const QString &path) const;
    void rebuildRoutedPages(const QMap<QString, QString> &currentValues);
    QVector<WorkflowViewPageMeta> routedPagesForSelection(const QString &selectedValue) const;
    void addViewPagesToLayout(QVBoxLayout *layout, const QMap<QString, QString> &currentValues);
    void addViewSectionToLayout(QVBoxLayout *layout, const WorkflowViewSectionMeta &section,
                                const QMap<QString, QString> &currentValues);
    void addInputsToLayout(QVBoxLayout *layout, const QVector<WorkflowInputMeta> &inputs,
                           const QMap<QString, QString> &currentValues, bool topLevel);
    void addLeafInputRow(QFormLayout *form, const WorkflowInputMeta &input, const QString &value, QWidget *groupWidget);
    QString helpTextForInput(const WorkflowInputMeta &input) const;

    WorkflowMeta m_workflow;
    QMap<QString, QWidget *> m_editors;
    QVBoxLayout *m_routedPagesHost = nullptr;
    QWidget *m_routedPagesContainer = nullptr;
    QComboBox *m_entryChoice = nullptr;
};
