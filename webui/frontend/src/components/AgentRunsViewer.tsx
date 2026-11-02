import { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Gauge,
  RefreshCw,
  Terminal,
  XCircle,
} from 'lucide-react';
import { getAgentRun, listAgentRuns } from '../api';
import type { AgentRunDetail, AgentRunEvent, AgentRunSummary } from '../types';

interface AgentRunsViewerProps {
  projectPath: string;
}

export function AgentRunsViewer({ projectPath }: AgentRunsViewerProps) {
  const [runs, setRuns] = useState<AgentRunSummary[]>([]);
  const [detail, setDetail] = useState<AgentRunDetail | null>(null);
  const [stuckOnly, setStuckOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState('');
  const [refreshToken, setRefreshToken] = useState(0);
  const totals = useMemo(() => ({
    runs: runs.length,
    stuck: runs.filter((run) => (run.stuck_events || 0) > 0).length,
    denied: runs.reduce((sum, run) => sum + (run.tool_denied || 0), 0),
    tokens: runs.reduce((sum, run) => sum + (run.total_tokens || 0), 0),
  }), [runs]);

  useEffect(() => {
    let cancelled = false;
    listAgentRuns(projectPath, 50, stuckOnly)
      .then((data) => {
        if (cancelled) return;
        setRuns(data);
        setDetail((current) =>
          current && data.some((run) => run.run_id === current.summary.run_id) ? current : null
        );
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectPath, stuckOnly, refreshToken]);

  async function openRun(runID: string) {
    setDetailLoading(true);
    setError('');
    try {
      setDetail(await getAgentRun(runID, projectPath));
    } catch (err) {
      setError(String(err));
    } finally {
      setDetailLoading(false);
    }
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="mb-1 flex items-center gap-2 text-2xl font-bold">
            <Gauge className="h-6 w-6 text-[var(--primary)]" />
            Agent 运行
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            查看模型调用、skill、工具命令、拒绝原因和卡住信号。
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => {
              setLoading(true);
              setError('');
              setStuckOnly((value) => !value);
            }}
            className={`btn ${stuckOnly ? 'btn-primary' : 'btn-secondary'}`}
          >
            <AlertTriangle className="h-4 w-4" />
            只看卡住
          </button>
          <button
            type="button"
            onClick={() => {
              setLoading(true);
              setError('');
              setRefreshToken((value) => value + 1);
            }}
            className="btn btn-secondary"
          >
            <RefreshCw className="h-4 w-4" />
            刷新
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-[var(--error)]/40 bg-[var(--error)]/10 px-4 py-3 text-sm text-[var(--error)]">
          {error}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <SummaryCard label="运行数" value={String(totals.runs)} />
        <SummaryCard label="卡住运行" value={String(totals.stuck)} warning={totals.stuck > 0} />
        <SummaryCard label="工具拒绝" value={String(totals.denied)} warning={totals.denied > 0} />
        <SummaryCard label="Tokens" value={totals.tokens.toLocaleString()} />
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(18rem,0.85fr)_minmax(30rem,1.5fr)]">
        <div className="glass rounded-xl p-3">
          {loading ? (
            <div className="flex h-48 items-center justify-center text-[var(--text-muted)]">
              <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
              加载运行记录…
            </div>
          ) : runs.length === 0 ? (
            <div className="flex h-48 flex-col items-center justify-center text-center text-[var(--text-muted)]">
              <Clock3 className="mb-3 h-10 w-10 opacity-50" />
              <p>暂无运行记录</p>
              <p className="mt-1 text-xs">执行一次 setup、compose、write 或 tool 命令后出现。</p>
            </div>
          ) : (
            <div className="max-h-[70vh] space-y-2 overflow-auto pr-1">
              {runs.map((run) => (
                <button
                  key={run.run_id}
                  type="button"
                  onClick={() => void openRun(run.run_id)}
                  className={`w-full rounded-lg border p-3 text-left transition ${
                    detail?.summary.run_id === run.run_id
                      ? 'border-[var(--primary)]/60 bg-[var(--primary)]/10'
                      : 'border-[var(--border)]/60 bg-[var(--surface)]/50 hover:bg-[var(--surface-light)]'
                  }`}
                >
                  <div className="flex items-start gap-2">
                    <RunStatusIcon status={run.status} />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-semibold">{run.command || run.run_id}</div>
                      <div className="mt-1 text-xs text-[var(--text-muted)]">
                        {formatTime(run.started_at)} · {formatDuration(run.duration_ms)} · {run.total_tokens || 0} tokens
                      </div>
                      <div className="mt-2 flex flex-wrap gap-1.5 text-[11px] text-[var(--text-muted)]">
                        <span>{run.agent_calls} calls</span>
                        <span>{run.tool_calls} tools</span>
                        {(run.stuck_events || 0) > 0 && (
                          <span className="rounded bg-[var(--warning)]/15 px-1.5 py-0.5 text-[var(--warning)]">
                            {run.stuck_events} stuck
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="glass min-h-[28rem] rounded-xl p-4">
          {detailLoading ? (
            <div className="flex h-64 items-center justify-center text-[var(--text-muted)]">
              <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
              加载时间线…
            </div>
          ) : detail ? (
            <RunDetailView detail={detail} />
          ) : (
            <div className="flex h-64 flex-col items-center justify-center text-center text-[var(--text-muted)]">
              <Terminal className="mb-3 h-10 w-10 opacity-50" />
              <p>选择左侧运行记录查看事件时间线</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function RunDetailView({ detail }: { detail: AgentRunDetail }) {
  const { summary } = detail;
  return (
    <div>
      <div className="border-b border-[var(--border)]/60 pb-4">
        <div className="flex items-start gap-2">
          <RunStatusIcon status={summary.status} />
          <div className="min-w-0">
            <h2 className="break-words text-lg font-bold">{summary.command || summary.run_id}</h2>
            <p className="mt-1 break-all text-xs text-[var(--text-muted)]">{summary.run_id}</p>
          </div>
        </div>
        <div className="mt-4 grid grid-cols-2 gap-2 text-xs text-[var(--text-muted)] md:grid-cols-4">
          <Metric label="模型调用" value={String(summary.agent_calls)} />
          <Metric label="工具调用" value={String(summary.tool_calls)} />
          <Metric label="拒绝" value={String(summary.tool_denied || 0)} />
          <Metric label="Tokens" value={String(summary.total_tokens || 0)} />
        </div>
        {!!summary.skills?.length && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {summary.skills.map((skill) => (
              <span key={skill} className="rounded bg-[var(--primary)]/10 px-2 py-0.5 text-xs text-[var(--primary)]">
                {skill}
              </span>
            ))}
          </div>
        )}
        {!!summary.stuck?.length && (
          <div className="mt-3 rounded-lg border border-[var(--warning)]/30 bg-[var(--warning)]/10 p-2 text-xs text-[var(--warning)]">
            {summary.stuck.map((signal) => <div key={signal.reason}>⚠ {signal.reason}</div>)}
          </div>
        )}
      </div>

      <div className="mt-4 max-h-[58vh] space-y-2 overflow-auto pr-1">
        {detail.events.map((event, index) => (
          <RunEventRow key={`${event.span_id || 'event'}-${index}`} event={event} />
        ))}
      </div>
    </div>
  );
}

function RunEventRow({ event }: { event: AgentRunEvent }) {
  const isProblem = event.event === 'stuck' || event.status === 'error' || event.allowed === false;
  return (
    <div className={`rounded-lg border px-3 py-2 ${
      isProblem
        ? 'border-[var(--warning)]/35 bg-[var(--warning)]/10'
        : 'border-[var(--border)]/50 bg-[var(--surface)]/45'
    }`}>
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="font-mono text-[var(--text-muted)]">{formatClock(event.time)}</span>
        <span className="font-semibold">{event.event}</span>
        {event.status && <span className="badge">{event.status}</span>}
        {event.agent && <span className="text-[var(--text-muted)]">{event.agent}</span>}
        {event.model && <span className="text-[var(--primary)]">{event.model}</span>}
        {!!event.duration_ms && <span className="text-[var(--text-muted)]">{event.duration_ms}ms</span>}
      </div>
      {event.command && <div className="mt-1 break-words font-mono text-xs text-[var(--text-muted)]">{event.command}</div>}
      {!!event.loaded_skills?.length && (
        <div className="mt-1 text-xs text-[var(--text-muted)]">loaded: {event.loaded_skills.join(', ')}</div>
      )}
      {event.reason && <div className="mt-1 text-xs text-[var(--warning)]">{event.reason}</div>}
      {event.error && <div className="mt-1 text-xs text-[var(--error)]">{event.error}</div>}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-[var(--surface)]/70 px-3 py-2">
      <div className="text-[10px] uppercase tracking-wide">{label}</div>
      <div className="mt-0.5 text-base font-semibold text-[var(--text)]">{value}</div>
    </div>
  );
}

function SummaryCard({ label, value, warning = false }: { label: string; value: string; warning?: boolean }) {
  return (
    <div className="glass rounded-xl px-4 py-3">
      <div className="text-xs text-[var(--text-muted)]">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${warning ? 'text-[var(--warning)]' : 'text-[var(--text)]'}`}>
        {value}
      </div>
    </div>
  );
}

function RunStatusIcon({ status }: { status: string }) {
  if (status === 'completed') return <CheckCircle2 className="h-4 w-4 flex-none text-[var(--success)]" />;
  if (status === 'failed') return <XCircle className="h-4 w-4 flex-none text-[var(--error)]" />;
  return <Clock3 className="h-4 w-4 flex-none text-[var(--primary)]" />;
}

function formatDuration(ms?: number) {
  if (!ms) return '0s';
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

function formatTime(value?: string) {
  if (!value) return '未开始';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function formatClock(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString();
}
