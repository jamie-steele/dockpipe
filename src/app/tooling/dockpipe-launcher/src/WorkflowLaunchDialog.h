#pragma once

#include "WorkflowCatalog.h"

#include <QDialog>
#include <QMap>

class QCheckBox;

class WorkflowLaunchDialog : public QDialog {
    Q_OBJECT
public:
    explicit WorkflowLaunchDialog(const WorkflowMeta &workflow, const QMap<QString, QString> &currentValues,
                                  QWidget *parent = nullptr);

    QMap<QString, QString> values() const;

private:
    WorkflowMeta m_workflow;
    QMap<QString, QWidget *> m_editors;
};
