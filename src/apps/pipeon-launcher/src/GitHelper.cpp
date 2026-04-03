#include "GitHelper.h"

#include <QDir>
#include <QProcess>
#include <QStringList>

QString GitHelper::repoRoot(const QString &dir)
{
    QProcess p;
    p.setProgram(QStringLiteral("git"));
    p.setArguments({QStringLiteral("-C"), QDir::toNativeSeparators(dir), QStringLiteral("rev-parse"),
                    QStringLiteral("--show-toplevel")});
    p.setProcessChannelMode(QProcess::MergedChannels);
    p.start();
    if (!p.waitForFinished(10000) || p.exitCode() != 0)
        return {};
    QString out = QString::fromUtf8(p.readAllStandardOutput()).trimmed();
    if (out.isEmpty())
        return {};
    return QDir::cleanPath(out);
}

QVector<WorktreeRow> GitHelper::listWorktrees(const QString &repoRoot)
{
    QVector<WorktreeRow> rows;
    QProcess p;
    p.setProgram(QStringLiteral("git"));
    p.setArguments({QStringLiteral("-C"), QDir::toNativeSeparators(repoRoot), QStringLiteral("worktree"),
                    QStringLiteral("list"), QStringLiteral("--porcelain")});
    p.setProcessChannelMode(QProcess::MergedChannels);
    p.start();
    if (!p.waitForFinished(15000) || p.exitCode() != 0)
        return rows;

    const QString text = QString::fromUtf8(p.readAllStandardOutput());
    const QStringList blocks = text.split(QStringLiteral("\n\n"), Qt::SkipEmptyParts);
    for (const QString &block : blocks) {
        QString path;
        QString branch;
        QString commit;
        const QStringList lines = block.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            if (line.startsWith(QStringLiteral("worktree "))) {
                path = QDir::cleanPath(line.mid(9).trimmed());
            } else if (line.startsWith(QStringLiteral("HEAD "))) {
                commit = line.mid(5).trimmed();
            } else if (line.startsWith(QStringLiteral("branch "))) {
                branch = line.mid(7).trimmed();
                if (branch.startsWith(QStringLiteral("refs/heads/")))
                    branch = branch.mid(11);
            } else if (line.startsWith(QStringLiteral("detached"))) {
                branch = QStringLiteral("(detached)");
            }
        }
        if (!path.isEmpty()) {
            WorktreeRow r;
            r.path = path;
            r.branch = branch;
            r.commit = commit;
            rows.append(r);
        }
    }
    return rows;
}
