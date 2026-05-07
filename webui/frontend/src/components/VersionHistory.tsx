import { useEffect, useState } from 'react';
import { Clock, History, Loader2, RefreshCw, RotateCcw } from 'lucide-react';
import { listFileVersions, restoreFileVersion } from '../api';
import type { FileVersion } from '../types';

interface VersionHistoryProps {
  projectPath: string;
  filePath: string;
  label: string;
  onRestored: () => void;
}

export function VersionHistory({ projectPath, filePath, label, onRestored }: VersionHistoryProps) {
  const [open, setOpen] = useState(false);
  const [versions, setVersions] = useState<FileVersion[]>([]);
  const [loading, setLoading] = useState(false);
  const [restoring, setRestoring] = useState<string | null>(null);
  const [armed, setArmed] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      loadVersions();
    }
  }, [open, filePath, projectPath]);

  async function loadVersions() {
    setLoading(true);
    setError(null);
    try {
      setVersions(await listFileVersions(filePath, projectPath));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  async function restore(version: FileVersion) {
    if (armed !== version.filename) {
      setArmed(version.filename);
      window.setTimeout(() => setArmed((current) => (current === version.filename ? null : current)), 3000);
      return;
    }

    setRestoring(version.filename);
    setError(null);
    try {
      await restoreFileVersion(filePath, version.filename, projectPath);
      setArmed(null);
      await loadVersions();
      onRestored();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setRestoring(null);
    }
  }

  return (
    <section className="panel panel-pad space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="panel-title">
            <History className="h-4 w-4 text-[var(--primary)]" />
            版本历史
          </h2>
          <p className="mt-1 text-xs text-[var(--text-muted)]">{label}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {open && (
            <button className="btn btn-secondary text-sm" onClick={loadVersions} disabled={loading}>
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              刷新
            </button>
          )}
          <button className="btn btn-secondary text-sm" onClick={() => setOpen((current) => !current)}>
            {open ? '收起版本' : '查看版本'}
          </button>
        </div>
      </div>

      {open && (
        <div className="space-y-2">
          {error && <div className="rounded-lg border border-[var(--error)]/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
          {loading ? (
            <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
              <Loader2 className="h-4 w-4 animate-spin" />
              正在读取版本...
            </div>
          ) : versions.length ? (
            versions.map((version) => (
              <div key={version.filename} className="flex flex-col gap-3 rounded-lg border border-[var(--border)]/70 bg-[var(--surface-light)]/35 p-3 lg:flex-row lg:items-center lg:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    <Clock className="h-4 w-4 text-[var(--text-muted)]" />
                    <span>{version.created_at || version.filename}</span>
                    <span className="rounded bg-[var(--surface)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{formatSize(version.size)}</span>
                  </div>
                  <p className="mt-1 truncate text-xs text-[var(--text-muted)]">{version.filename}</p>
                </div>
                <button className="btn btn-secondary text-sm" onClick={() => restore(version)} disabled={restoring === version.filename}>
                  {restoring === version.filename ? <Loader2 className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
                  {armed === version.filename ? '确认回滚' : '回滚到此版'}
                </button>
              </div>
            ))
          ) : (
            <div className="rounded-lg border border-dashed border-[var(--border)] p-4 text-sm text-[var(--text-muted)]">
              暂无历史版本。下次保存或 AI 覆盖前会自动生成备份。
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function formatSize(size: number) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
