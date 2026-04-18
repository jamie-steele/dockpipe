#pragma once

#include <QString>
#include <QVector>

struct WorktreeRow {
    QString path;
    QString branch;
    QString commit;
};

class GitHelper {
public:
    /// Empty if not inside a git work tree.
    static QString repoRoot(const QString &dir);
    /// Lines from `git worktree list` (excluding parse errors).
    static QVector<WorktreeRow> listWorktrees(const QString &repoRoot);
};
