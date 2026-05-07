import { useEffect, useMemo, useState } from 'react';
import { AlertCircle, Bot, ChevronDown, FileText, Loader2, RefreshCw, Search } from 'lucide-react';
import { getAICall, listAICalls } from '../api';
import type { AICallDetail, AICallSummary } from '../types';

type DetailTab = 'system' | 'user' | 'output';

type StageInfo = {
  id: string;
  label: string;
  hint: string;
  order: number;
};

type StageGroup = StageInfo & {
  calls: AICallSummary[];
  totalTokens: number;
  inputChars: number;
  outputChars: number;
  latestStartedAt: string;
};

const stageCatalog: StageInfo[] = [
  { id: 'setup', label: '设定', hint: 'Story setup / storyline review', order: 10 },
  { id: 'compose', label: '大纲', hint: 'Skeleton / volume / outline improve', order: 20 },
  { id: 'craft', label: 'Craft', hint: 'Characters / locations / items', order: 30 },
  { id: 'write', label: '写作', hint: 'Draft / chapter / recap / translate', order: 40 },
  { id: 'rpg', label: '数值化系统', hint: 'RPG-DSL / simulation repair', order: 50 },
  { id: 'template', label: '模板', hint: 'Template extraction / application', order: 60 },
  { id: 'legacy', label: 'Legacy', hint: '旧日志格式', order: 90 },
  { id: 'other', label: '其他', hint: '未识别 stage', order: 100 },
];

const stageById = new Map(stageCatalog.map((stage) => [stage.id, stage]));

function formatTime(value: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatNumber(value?: number) {
  return typeof value === 'number' && value > 0 ? value.toLocaleString() : '-';
}

function previewText(call: AICallSummary) {
  if (call.command) return call.command;
  if (call.legacy) return 'legacy log';
  return 'AI call';
}

function inferStage(call: AICallSummary): StageInfo {
  if (call.legacy) return stageById.get('legacy')!;
  const text = [call.agent, call.command, call.id].filter(Boolean).join(' ').toLowerCase();
  const matches: Array<[string, RegExp]> = [
    ['setup', /\bsetup\b|storyline|story_setup|设定/],
    ['compose', /\bcompose\b|outline|skeleton|volume|大纲/],
    ['craft', /\bcraft\b|character|location|item|角色|地点|物品/],
    ['write', /\bwrite\b|chapter|recap|draft|translate|章节|写作|翻译/],
    ['rpg', /\brpg\b|dsl|simulate|simulation|数值/],
    ['template', /\btemplate\b|模板/],
  ];
  const found = matches.find(([, pattern]) => pattern.test(text));
  return stageById.get(found?.[0] || 'other')!;
}

function createStageGroups(calls: AICallSummary[]): StageGroup[] {
  const groups = new Map<string, StageGroup>();
  for (const call of calls) {
    const stage = inferStage(call);
    const current =
      groups.get(stage.id) ||
      ({
        ...stage,
        calls: [],
        totalTokens: 0,
        inputChars: 0,
        outputChars: 0,
        latestStartedAt: '',
      } satisfies StageGroup);
    current.calls.push(call);
    current.totalTokens += call.total_tokens || 0;
    current.inputChars += call.input_chars || 0;
    current.outputChars += call.output_chars || 0;
    if (!current.latestStartedAt || new Date(call.started_at).getTime() > new Date(current.latestStartedAt).getTime()) {
      current.latestStartedAt = call.started_at;
    }
    groups.set(stage.id, current);
  }
  return Array.from(groups.values()).sort((a, b) => a.order - b.order || b.calls.length - a.calls.length);
}

export function AICallsViewer({ projectPath }: { projectPath: string }) {
  const [calls, setCalls] = useState<AICallSummary[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<AICallDetail | null>(null);
  const [tab, setTab] = useState<DetailTab>('user');
  const [query, setQuery] = useState('');
  const [collapsedStages, setCollapsedStages] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadCalls();
  }, [projectPath]);

  useEffect(() => {
    if (!selectedId) {
      setDetail(null);
      return;
    }
    loadDetail(selectedId);
  }, [selectedId, projectPath]);

  const filteredCalls = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return calls;
    return calls.filter((call) =>
      [call.agent, call.command, call.model, call.id, inferStage(call).label, inferStage(call).hint]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(needle))
    );
  }, [calls, query]);

  const groupedCalls = useMemo(() => createStageGroups(filteredCalls), [filteredCalls]);
  const allStageCount = useMemo(() => createStageGroups(calls).length, [calls]);

  async function loadCalls() {
    setLoading(true);
    setError(null);
    try {
      const next = await listAICalls(projectPath);
      setCalls(next);
      setSelectedId((current) => (next.some((call) => call.id === current) ? current : next[0]?.id || null));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  function toggleStage(stageId: string) {
    setCollapsedStages((current) => {
      const next = new Set(current);
      if (next.has(stageId)) {
        next.delete(stageId);
      } else {
        next.add(stageId);
      }
      return next;
    });
  }

  async function loadDetail(id: string) {
    setDetailLoading(true);
    setError(null);
    try {
      const next = await getAICall(id, projectPath);
      setDetail(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  }

  const activeContent =
    tab === 'system'
      ? detail?.system_prompt || ''
      : tab === 'user'
        ? detail?.user_prompt || ''
        : detail?.response || '';

  return (
    <div className="animate-fade-in space-y-6">
      <div className="workbench-header">
        <div>
          <p className="eyebrow mb-2">AI observability</p>
          <h1 className="mb-1 text-2xl font-bold">AI 调用</h1>
          <p className="max-w-3xl text-sm text-[var(--text-muted)]">
            查看每次 AI 调用的 context 输入、用户输入和模型输出，用来排查生成偏移、上下文过长或提示不稳定的问题。
          </p>
        </div>
        <button className="btn btn-secondary" onClick={loadCalls}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </button>
      </div>

      {error && (
        <div className="flex items-center gap-2 rounded-lg border border-[var(--error)]/40 bg-red-500/10 p-3 text-sm text-red-300">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[380px_minmax(0,1fr)]">
        <aside className="sticky-rail space-y-4">
          <section className="panel panel-pad space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="panel-title">
                <Bot className="h-4 w-4 text-[var(--primary)]" />
                Stage 分组
              </h2>
              <span className="text-xs text-[var(--text-muted)]">
                {groupedCalls.length}/{allStageCount} 组 · {filteredCalls.length}/{calls.length}
              </span>
            </div>
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
              <input
                className="input pl-9"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="搜索 agent / model / command"
              />
            </label>
            <div className="max-h-[680px] space-y-2 overflow-auto">
              {loading ? (
                <div className="flex items-center justify-center py-12 text-[var(--text-muted)]">
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  读取日志中
                </div>
              ) : filteredCalls.length === 0 ? (
                <div className="rounded-lg border border-dashed border-[var(--border)]/80 p-6 text-center text-sm text-[var(--text-muted)]">
                  暂无 AI 调用日志
                </div>
              ) : (
                groupedCalls.map((group) => {
                  const collapsed = collapsedStages.has(group.id);
                  const selectedInGroup = group.calls.some((call) => call.id === selectedId);
                  return (
                    <div key={group.id} className="rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/50">
                      <button
                        className="flex w-full items-start justify-between gap-3 p-3 text-left transition-colors hover:bg-[var(--surface-light)]/60"
                        onClick={() => toggleStage(group.id)}
                      >
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <ChevronDown className={`h-4 w-4 text-[var(--text-muted)] transition-transform ${collapsed ? '-rotate-90' : ''}`} />
                            <span className="font-semibold">{group.label}</span>
                            <span className={`badge ${selectedInGroup ? 'badge-success' : 'badge-info'}`}>{group.calls.length}</span>
                          </div>
                          <p className="mt-1 truncate pl-6 text-xs text-[var(--text-muted)]">{group.hint}</p>
                        </div>
                        <div className="shrink-0 text-right text-xs text-[var(--text-muted)]">
                          <p>{formatNumber(group.totalTokens)} tokens</p>
                          <p>{formatTime(group.latestStartedAt)}</p>
                        </div>
                      </button>
                      {!collapsed && (
                        <div className="space-y-2 border-t border-[var(--border)]/60 p-2">
                          {group.calls.map((call) => (
                            <button
                              key={call.id}
                              className={`w-full rounded-lg border p-3 text-left transition-colors ${
                                selectedId === call.id
                                  ? 'border-[var(--primary)] bg-[var(--primary)]/10 shadow-sm'
                                  : 'border-[var(--border)]/60 bg-[var(--surface)]/70 hover:bg-[var(--surface-light)]'
                              }`}
                              onClick={() => {
                                setSelectedId(call.id);
                                setTab(call.has_output ? 'output' : 'user');
                              }}
                            >
                              <div className="flex items-center gap-2">
                                <span className="truncate font-medium">{call.agent || 'Unknown agent'}</span>
                                {call.legacy && <span className="badge badge-info">legacy</span>}
                              </div>
                              <p className="mt-1 truncate text-xs text-[var(--text-muted)]">{previewText(call)}</p>
                              <div className="mt-2 flex items-center justify-between text-xs text-[var(--text-muted)]">
                                <span>{formatTime(call.started_at)}</span>
                                <span>{formatNumber(call.total_tokens)} tokens</span>
                              </div>
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          </section>
        </aside>

        <main className="min-w-0 space-y-4">
          <section className="panel panel-pad">
            {detailLoading ? (
              <div className="flex min-h-[420px] items-center justify-center text-[var(--text-muted)]">
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                读取调用详情中
              </div>
            ) : detail ? (
              <div className="space-y-4">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <span className="badge badge-info">{inferStage(detail).label}</span>
                      {detail.command && <span className="rounded bg-[var(--surface-light)] px-2 py-1 text-xs text-[var(--text-muted)]">{detail.command}</span>}
                    </div>
                    <h2 className="text-xl font-bold">{detail.agent}</h2>
                    <p className="text-sm text-[var(--text-muted)]">{formatTime(detail.started_at)}</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {detail.model && <span className="badge badge-info">{detail.model}</span>}
                    {detail.legacy && <span className="badge badge-warning">legacy pairing</span>}
                    {detail.total_tokens ? <span className="badge badge-success">{detail.total_tokens.toLocaleString()} tokens</span> : null}
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
                  <div className="metric-card">
                    <p className="text-xl font-bold">{formatNumber(detail.input_chars)}</p>
                    <p className="text-xs text-[var(--text-muted)]">输入字符</p>
                  </div>
                  <div className="metric-card">
                    <p className="text-xl font-bold">{formatNumber(detail.output_chars)}</p>
                    <p className="text-xs text-[var(--text-muted)]">输出字符</p>
                  </div>
                  <div className="metric-card">
                    <p className="text-xl font-bold">{formatNumber(detail.prompt_tokens)}</p>
                    <p className="text-xs text-[var(--text-muted)]">Prompt Tokens</p>
                  </div>
                  <div className="metric-card">
                    <p className="text-xl font-bold">{formatNumber(detail.completion_tokens)}</p>
                    <p className="text-xs text-[var(--text-muted)]">Output Tokens</p>
                  </div>
                </div>

                {detail.skills?.length ? (
                  <div className="flex flex-wrap gap-2">
                    {detail.skills.map((skill) => (
                      <span key={skill} className="rounded-lg border border-[var(--border)]/70 px-2 py-1 text-xs text-[var(--text-muted)]">
                        {skill}
                      </span>
                    ))}
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2 border-b border-[var(--border)]/70 pb-3">
                  {[
                    ['system', 'Context 输入'],
                    ['user', '用户输入'],
                    ['output', 'AI 输出'],
                  ].map(([id, label]) => (
                    <button
                      key={id}
                      className={`btn ${tab === id ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setTab(id as DetailTab)}
                    >
                      {label}
                    </button>
                  ))}
                </div>

                <pre className="max-h-[720px] overflow-auto whitespace-pre-wrap rounded-lg border border-[var(--border)]/70 bg-black/25 p-4 text-sm leading-relaxed text-[var(--text)]">
                  {activeContent || '这一段日志为空。'}
                </pre>

                {(detail.prompt_path || detail.response_path) && (
                  <div className="space-y-1 text-xs text-[var(--text-muted)]">
                    {detail.prompt_path && <p>Prompt: {detail.prompt_path}</p>}
                    {detail.response_path && <p>Response: {detail.response_path}</p>}
                  </div>
                )}
              </div>
            ) : (
              <div className="flex min-h-[420px] flex-col items-center justify-center text-center text-[var(--text-muted)]">
                <FileText className="mb-3 h-12 w-12 opacity-60" />
                <p>选择一次 AI 调用查看详情</p>
              </div>
            )}
          </section>
        </main>
      </div>
    </div>
  );
}
