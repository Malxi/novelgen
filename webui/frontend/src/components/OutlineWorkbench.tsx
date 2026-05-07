import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  AlertCircle,
  ChevronRight,
  CheckCircle,
  FileJson,
  Loader2,
  Play,
  RefreshCw,
  Save,
  Sparkles,
  Wand2,
} from 'lucide-react';
import { createTask, getOutline, getTask, saveJSONFile } from '../api';
import { VersionHistory } from './VersionHistory';
import type { Chapter, Outline, Part, Task, Volume } from '../types';

interface OutlineWorkbenchProps {
  projectPath: string;
}

type OutlineMode = 'structure' | 'json';
type OutlineTarget =
  | { type: 'part'; partIndex: number }
  | { type: 'volume'; partIndex: number; volumeIndex: number }
  | { type: 'chapter'; partIndex: number; volumeIndex: number; chapterIndex: number };

interface VolumeRef {
  partIndex: number;
  volumeIndex: number;
  globalIndex: number;
  part: Part;
  volume: Volume;
}

const emptyOutline: Outline = { parts: [] };

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm text-[var(--text-muted)]">{label}</span>
      {children}
    </label>
  );
}

function splitList(value: string): string[] {
  return value
    .split(/\n|;|；|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value?: string[]): string {
  return (value || []).join('\n');
}

function volumeKey(item: VolumeRef): string {
  return item.volume.id || `${item.partIndex}-${item.volumeIndex}`;
}

function ListField({
  label,
  value,
  onChange,
  minHeight = 'min-h-24',
}: {
  label: string;
  value?: string[];
  onChange: (value: string[]) => void;
  minHeight?: string;
}) {
  return (
    <Field label={label}>
      <textarea className={`input ${minHeight}`} value={joinList(value)} onChange={(event) => onChange(splitList(event.target.value))} />
    </Field>
  );
}

function ConfirmButton({
  className,
  children,
  confirmText = '确认执行',
  onConfirm,
}: {
  className: string;
  children: ReactNode;
  confirmText?: string;
  onConfirm: () => void;
}) {
  const [armed, setArmed] = useState(false);

  return (
    <button
      className={className}
      onClick={() => {
        if (!armed) {
          setArmed(true);
          window.setTimeout(() => setArmed(false), 3000);
          return;
        }
        setArmed(false);
        onConfirm();
      }}
    >
      {armed ? confirmText : children}
    </button>
  );
}

function TaskStrip({ taskId, onDone }: { taskId: string | null; onDone: () => void }) {
  const [task, setTask] = useState<Task | null>(null);

  useEffect(() => {
    if (!taskId) {
      setTask(null);
      return;
    }

    const poll = async () => {
      const next = await getTask(taskId);
      setTask(next);
      if (next.status === 'completed') onDone();
      return next.status === 'completed' || next.status === 'failed';
    };

    let stopped = false;
    poll().catch(() => undefined);
    const timer = window.setInterval(async () => {
      if (stopped) return;
      const done = await poll().catch(() => false);
      if (done) {
        stopped = true;
        window.clearInterval(timer);
      }
    }, 1600);

    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [taskId, onDone]);

  if (!task) return null;

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
      <div className="flex items-center gap-2 text-sm">
        {task.status === 'running' && <Loader2 className="h-4 w-4 animate-spin text-[var(--primary)]" />}
        {task.status === 'completed' && <CheckCircle className="h-4 w-4 text-[var(--success)]" />}
        {task.status === 'failed' && <AlertCircle className="h-4 w-4 text-[var(--error)]" />}
        <span className="font-medium">{task.message}</span>
        <span className="text-[var(--text-muted)]">{task.progress}%</span>
      </div>
      {task.output && (
        <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-xs text-[var(--text-muted)]">
          {task.output.split('\n').slice(-10).join('\n')}
        </pre>
      )}
      {task.error && <p className="mt-2 text-sm text-[var(--error)]">{task.error}</p>}
    </div>
  );
}

export function OutlineWorkbench({ projectPath }: OutlineWorkbenchProps) {
  const [outline, setOutline] = useState<Outline>(emptyOutline);
  const [mode, setMode] = useState<OutlineMode>('structure');
  const [jsonDraft, setJsonDraft] = useState('');
  const [prompt, setPrompt] = useState('');
  const [taskId, setTaskId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [openVolumeKeys, setOpenVolumeKeys] = useState<Set<string>>(new Set());

  const volumes = useMemo<VolumeRef[]>(() => {
    const list: VolumeRef[] = [];
    let globalIndex = 0;
    outline.parts?.forEach((part, partIndex) => {
      part.volumes?.forEach((volume, volumeIndex) => {
        globalIndex += 1;
        list.push({ partIndex, volumeIndex, globalIndex, part, volume });
      });
    });
    return list;
  }, [outline]);

  const totalChapters = volumes.reduce((sum, item) => sum + (item.volume.chapters?.length || 0), 0);
  const emptyVolumes = volumes.filter((item) => !item.volume.chapters || item.volume.chapters.length === 0).length;
  useEffect(() => {
    setOpenVolumeKeys((current) => {
      const knownKeys = new Set(volumes.map(volumeKey));
      const next = new Set([...current].filter((key) => knownKeys.has(key)));
      return next.size === current.size ? current : next;
    });
  }, [volumes]);

  const loadOutline = useCallback(async () => {
    setError(null);
    try {
      const data = await getOutline(projectPath);
      setOutline(data);
      setJsonDraft(JSON.stringify(data, null, 2));
      setDirty(false);
    } catch (err) {
      setOutline(emptyOutline);
      setJsonDraft(JSON.stringify(emptyOutline, null, 2));
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [projectPath]);

  useEffect(() => {
    loadOutline();
  }, [loadOutline]);

  async function saveOutline(next = outline) {
    setSaving(true);
    setError(null);
    try {
      await saveJSONFile('story/compose/outline.json', next, projectPath);
      setOutline(next);
      setJsonDraft(JSON.stringify(next, null, 2));
      setDirty(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  async function saveJSONDraft() {
    try {
      const parsed = JSON.parse(jsonDraft) as Outline;
      await saveOutline(parsed);
      setMode('structure');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function runComposeTask(subcommand: string, args: Record<string, unknown>) {
    setError(null);
    const task = await createTask({
      type: 'compose',
      command: 'compose',
      args: { project_dir: projectPath, subcommand, ...args },
    });
    setTaskId(task.id);
  }

  function updatePart(index: number, patch: Partial<Part>) {
    setDirty(true);
    setOutline((current) => {
      const parts = [...(current.parts || [])];
      parts[index] = { ...parts[index], ...patch };
      return { ...current, parts };
    });
  }

  function updateVolume(partIndex: number, volumeIndex: number, patch: Partial<Volume>) {
    setDirty(true);
    setOutline((current) => {
      const parts = [...(current.parts || [])];
      const volumesForPart = [...(parts[partIndex].volumes || [])];
      volumesForPart[volumeIndex] = { ...volumesForPart[volumeIndex], ...patch };
      parts[partIndex] = { ...parts[partIndex], volumes: volumesForPart };
      return { ...current, parts };
    });
  }

  function updateChapter(partIndex: number, volumeIndex: number, chapterIndex: number, patch: Partial<Chapter>) {
    setDirty(true);
    setOutline((current) => {
      const parts = [...(current.parts || [])];
      const volumesForPart = [...(parts[partIndex].volumes || [])];
      const chapters = [...(volumesForPart[volumeIndex].chapters || [])];
      chapters[chapterIndex] = { ...chapters[chapterIndex], ...patch };
      volumesForPart[volumeIndex] = { ...volumesForPart[volumeIndex], chapters };
      parts[partIndex] = { ...parts[partIndex], volumes: volumesForPart };
      return { ...current, parts };
    });
  }

  function regenID(target: OutlineTarget) {
    if (target.type === 'part') return String(target.partIndex + 1);
    if (target.type === 'volume') return `${target.partIndex + 1}_${target.volumeIndex + 1}`;
    return `${target.partIndex + 1}_${target.volumeIndex + 1}_${target.chapterIndex + 1}`;
  }

  function runRegen(target: OutlineTarget) {
    runComposeTask('regen', { _positional: regenID(target), prompt });
  }

  function scrollToSection(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function toggleVolume(key: string) {
    setOpenVolumeKeys((current) => {
      const next = new Set(current);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }

  function setAllVolumesOpen(open: boolean) {
    setOpenVolumeKeys(open ? new Set(volumes.map(volumeKey)) : new Set());
  }

  return (
    <div className="animate-fade-in space-y-6">
      <div className="workbench-header">
        <div>
          <p className="eyebrow mb-2">Structured collaboration</p>
          <h1 className="mb-1 text-2xl font-bold">大纲协作台</h1>
          <p className="max-w-3xl text-sm text-[var(--text-muted)]">按 skeleton、volume、chapter 分层协作：先由人锁定局部契约，再让 AI 生成或改进选中的范围。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {dirty && <span className="badge badge-warning">有未保存修改</span>}
          <button onClick={loadOutline} className="btn btn-secondary">
            <RefreshCw className="h-4 w-4" />
            刷新
          </button>
          <button onClick={() => saveOutline()} disabled={saving} className="btn btn-primary">
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            保存
          </button>
          <button onClick={() => setMode(mode === 'json' ? 'structure' : 'json')} className="btn btn-secondary">
            <FileJson className="h-4 w-4" />
            {mode === 'json' ? '结构编辑' : 'JSON 编辑'}
          </button>
        </div>
      </div>

      {error && <div className="rounded-lg border border-[var(--error)]/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      <TaskStrip taskId={taskId} onDone={loadOutline} />

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <aside className="sticky-rail space-y-4">
          <section className="panel panel-pad space-y-3">
            <h2 className="panel-title">
              <Sparkles className="h-4 w-4 text-[var(--primary)]" />
              全量 / Skeleton
            </h2>
            <Field label="AI 提示">
              <textarea
                className="input min-h-24"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="例如：三哥必须进入卷级契约；每卷都要有真实代价；减少被动角色和巧合。"
              />
            </Field>
            <div className="grid grid-cols-1 gap-2">
              <ConfirmButton className="btn btn-secondary" confirmText="确认全量生成" onConfirm={() => runComposeTask('gen', { hierarchical: true })}>
                <Play className="h-4 w-4" />
                全量生成大纲
              </ConfirmButton>
              <button
                className="btn btn-secondary"
                onClick={() =>
                  runComposeTask('pipeline', {
                    'from-volume': 1,
                    'to-volume': 1,
                    'skip-gen': true,
                    'skip-improve': true,
                    'skip-cross': true,
                    force: true,
                  })
                }
              >
                生成缺失 Skeleton
              </button>
              <button className="btn btn-primary" onClick={() => runComposeTask('improve', { prompt, force: true, 'max-rounds': 1 })}>
                <Wand2 className="h-4 w-4" />
                全量改进已生成卷
              </button>
              <button className="btn btn-secondary" onClick={() => runComposeTask('storyline-plan', { force: true })}>
                生成卷级 Storyline Plan
              </button>
              <button className="btn btn-secondary" onClick={() => runComposeTask('review', { prompt, apply: true })}>
                Rationality Review 后应用
              </button>
              <button className="btn btn-secondary" onClick={() => runComposeTask('check', {})}>
                只运行确定性检查
              </button>
            </div>
          </section>

          <section className="grid grid-cols-3 gap-3">
            <div className="metric-card">
              <p className="text-2xl font-bold">{outline.parts?.length || 0}</p>
              <p className="text-xs text-[var(--text-muted)]">部</p>
            </div>
            <div className="metric-card">
              <p className="text-2xl font-bold">{volumes.length}</p>
              <p className="text-xs text-[var(--text-muted)]">卷</p>
            </div>
            <div className="metric-card">
              <p className="text-2xl font-bold">{totalChapters}</p>
              <p className="text-xs text-[var(--text-muted)]">章</p>
            </div>
          </section>
          {emptyVolumes > 0 && <p className="text-sm text-[var(--warning)]">还有 {emptyVolumes} 个空卷等待人类确认摘要后生成。</p>}

          <section className="panel panel-pad space-y-3">
            <h2 className="panel-title">快速定位</h2>
            <div className="grid grid-cols-2 gap-2">
              <button className="btn btn-secondary text-sm" onClick={() => scrollToSection('outline-parts')}>部级骨架</button>
              <button className="btn btn-secondary text-sm" onClick={() => scrollToSection('outline-volumes')}>卷工作台</button>
            </div>
          </section>

          <VersionHistory
            projectPath={projectPath}
            filePath="story/compose/outline.json"
            label="大纲 / outline.json"
            onRestored={loadOutline}
          />
        </aside>

        <main className="min-w-0">
          {mode === 'json' ? (
            <section className="panel panel-pad space-y-3">
              <h2 className="font-semibold">全量 JSON 编辑</h2>
              <textarea className="input min-h-[720px] font-mono text-sm" value={jsonDraft} onChange={(event) => setJsonDraft(event.target.value)} />
              <div className="flex justify-end">
                <button className="btn btn-primary" onClick={saveJSONDraft}>
                  <Save className="h-4 w-4" />
                  保存 JSON
                </button>
              </div>
            </section>
          ) : (
            <section className="space-y-4">
              <div className="sticky top-0 z-20 rounded-xl border border-[var(--border)]/70 bg-[var(--background)]/90 p-2 shadow-sm backdrop-blur">
                <div className="flex gap-2 overflow-x-auto">
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('outline-parts')}>
                    部级骨架 {outline.parts?.length || 0}
                  </button>
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('outline-volumes')}>
                    卷工作台 {volumes.length}/{totalChapters}
                  </button>
                </div>
              </div>

              <div id="outline-parts" className="scroll-mt-20 panel panel-pad space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">Part Skeleton / 部级骨架</h2>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">这里只维护部级功能。卷级骨架放在下面每个卷卡片的左侧。</p>
                  </div>
                </div>

                {outline.parts?.length ? (
                  <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                    {outline.parts.map((part, partIndex) => (
                      <details key={part.id || partIndex} className="group rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/60">
                        <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-4 transition-colors hover:bg-[var(--surface-light)]/60">
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium">部 {partIndex + 1}: {part.title || '未命名'}</span>
                              <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{part.volumes?.length || 0} 卷</span>
                            </div>
                            <p className="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{part.summary || '未填写部级摘要'}</p>
                          </div>
                          <span className="mt-1 text-xs text-[var(--text-muted)] group-open:hidden">展开编辑</span>
                          <span className="mt-1 hidden text-xs text-[var(--text-muted)] group-open:inline">收起</span>
                        </summary>
                        <div className="space-y-4 border-t border-[var(--border)]/70 p-4">
                          <div className="flex justify-end">
                            <ConfirmButton className="btn btn-secondary" confirmText="确认重写此部" onConfirm={() => runRegen({ type: 'part', partIndex })}>
                              <Sparkles className="h-4 w-4" />
                              AI 重写此部骨架
                            </ConfirmButton>
                          </div>
                          <Field label="部标题">
                            <input className="input" value={part.title || ''} onChange={(event) => updatePart(partIndex, { title: event.target.value })} />
                          </Field>
                          <Field label="部摘要 / 骨架契约">
                            <textarea className="input min-h-28" value={part.summary || ''} onChange={(event) => updatePart(partIndex, { summary: event.target.value })} />
                          </Field>
                        </div>
                      </details>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-lg border border-dashed border-[var(--border)] p-6 text-center text-sm text-[var(--text-muted)]">暂无大纲骨架。</div>
                )}
              </div>

              <div id="outline-volumes" className="scroll-mt-20 panel panel-pad space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">Volume Workbench / 卷工作台</h2>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">每个卷只出现一次：左侧维护卷骨架，右侧维护本卷章节。</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button className="btn btn-secondary text-sm" onClick={() => setAllVolumesOpen(true)}>
                      全部展开
                    </button>
                    <button className="btn btn-secondary text-sm" onClick={() => setAllVolumesOpen(false)} disabled={!openVolumeKeys.size}>
                      全部收起
                    </button>
                  </div>
                </div>

                <div className="space-y-3">
                  {volumes.map((item) => {
                    const key = volumeKey(item);
                    const isOpen = openVolumeKeys.has(key);

                    return (
                      <section key={key} className="rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/60">
                        <button
                          type="button"
                          className="flex w-full items-start justify-between gap-3 p-4 text-left transition-colors hover:bg-[var(--surface-light)]/45"
                          aria-expanded={isOpen}
                          onClick={() => toggleVolume(key)}
                        >
                          <div className="flex min-w-0 flex-1 gap-3">
                            <ChevronRight className={`mt-1 h-4 w-4 flex-none text-[var(--text-muted)] transition-transform ${isOpen ? 'rotate-90' : ''}`} />
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="font-medium">V{item.globalIndex} {item.volume.title || '未命名卷'}</span>
                                <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{item.part.title || `部 ${item.partIndex + 1}`}</span>
                                <span className={`rounded px-2 py-0.5 text-xs ${item.volume.chapters?.length ? 'bg-[var(--primary)]/10 text-[var(--primary)]' : 'bg-yellow-500/15 text-yellow-300'}`}>
                                  {item.volume.chapters?.length || 0} 章
                                </span>
                              </div>
                              <p className="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{item.volume.summary || '未填写卷摘要'}</p>
                            </div>
                          </div>
                          <span className="mt-1 flex-none text-xs text-[var(--text-muted)]">{isOpen ? '收起' : '展开编辑'}</span>
                        </button>
                        {isOpen && (
                        <div className="grid grid-cols-1 gap-4 border-t border-[var(--border)]/70 p-4 2xl:grid-cols-[minmax(300px,0.8fr)_minmax(0,1.2fr)]">
                          <div className="space-y-4 rounded-lg border border-[var(--border)]/60 bg-[var(--surface-light)]/35 p-3">
                            <div className="flex items-center justify-between gap-3">
                              <h3 className="text-sm font-semibold">卷骨架</h3>
                              <ConfirmButton
                                className="btn btn-secondary text-sm"
                                confirmText="确认重写"
                                onConfirm={() => runRegen({ type: 'volume', partIndex: item.partIndex, volumeIndex: item.volumeIndex })}
                              >
                                <Sparkles className="h-4 w-4" />
                                AI 重写
                              </ConfirmButton>
                            </div>
                            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-1">
                              <Field label="卷标题">
                                <input className="input" value={item.volume.title || ''} onChange={(event) => updateVolume(item.partIndex, item.volumeIndex, { title: event.target.value })} />
                              </Field>
                              <Field label="卷 ID">
                                <input className="input" value={item.volume.id || ''} onChange={(event) => updateVolume(item.partIndex, item.volumeIndex, { id: event.target.value })} />
                              </Field>
                            </div>
                            <Field label="卷摘要 / 章节生成契约">
                              <textarea className="input min-h-36" value={item.volume.summary || ''} onChange={(event) => updateVolume(item.partIndex, item.volumeIndex, { summary: event.target.value })} />
                            </Field>
                          </div>

                          <div className="space-y-3">
                            <div className="flex flex-col gap-3 rounded-lg border border-[var(--border)]/60 bg-[var(--surface-light)]/25 p-3 lg:flex-row lg:items-center lg:justify-between">
                              <h3 className="text-sm font-semibold">本卷章节</h3>
                              <div className="flex flex-wrap gap-2">
                                <button
                                  className="btn btn-secondary text-sm"
                                  onClick={() =>
                                    runComposeTask('pipeline', {
                                      'from-volume': item.globalIndex,
                                      'to-volume': item.globalIndex,
                                      'skip-improve': true,
                                      force: true,
                                    })
                                  }
                                >
                                  <Play className="h-4 w-4" />
                                  生成章节
                                </button>
                                <button className="btn btn-primary text-sm" onClick={() => runComposeTask('improve', { volume: item.globalIndex, prompt, force: true, 'max-rounds': 1 })}>
                                  <Wand2 className="h-4 w-4" />
                                  Improve
                                </button>
                              </div>
                            </div>
                            {item.volume.chapters?.length ? (
                              item.volume.chapters.map((chapter, chapterIndex) => (
                                <details key={chapter.id || chapterIndex} className="group/chapter rounded-lg bg-[var(--surface-light)]/50">
                                  <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-3 hover:bg-[var(--surface-light)]">
                                    <div className="min-w-0 flex-1">
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium">C{chapterIndex + 1} {chapter.title || chapter.id || '未命名章'}</span>
                                        {chapter.location && <span className="rounded bg-[var(--surface)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{chapter.location}</span>}
                                        {chapter.pacing && <span className="rounded bg-[var(--primary)]/10 px-2 py-0.5 text-xs text-[var(--primary)]">{chapter.pacing}</span>}
                                      </div>
                                      <p className="mt-1 line-clamp-2 text-xs text-[var(--text-muted)]">{chapter.summary || '未填写章节摘要'}</p>
                                    </div>
                                    <span className="mt-1 text-xs text-[var(--text-muted)] group-open/chapter:hidden">编辑</span>
                                    <span className="mt-1 hidden text-xs text-[var(--text-muted)] group-open/chapter:inline">收起</span>
                                  </summary>
                                  <div className="space-y-4 border-t border-[var(--border)]/60 p-3">
                                    <div className="flex justify-end">
                                      <ConfirmButton
                                        className="btn btn-secondary"
                                        confirmText="确认重写此章"
                                        onConfirm={() => runRegen({ type: 'chapter', partIndex: item.partIndex, volumeIndex: item.volumeIndex, chapterIndex })}
                                      >
                                        <Sparkles className="h-4 w-4" />
                                        AI 重写此章
                                      </ConfirmButton>
                                    </div>
                                    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                                      <Field label="标题">
                                        <input className="input" value={chapter.title || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { title: event.target.value })} />
                                      </Field>
                                      <Field label="地点">
                                        <input className="input" value={chapter.location || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { location: event.target.value })} />
                                      </Field>
                                      <div className="lg:col-span-2">
                                        <Field label="摘要">
                                          <textarea className="input min-h-24" value={chapter.summary || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { summary: event.target.value })} />
                                        </Field>
                                      </div>
                                      <ListField label="登场角色" value={chapter.characters} onChange={(value) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { characters: value })} />
                                      <ListField label="Beats" value={chapter.beats} onChange={(value) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { beats: value })} />
                                      <Field label="开场 Beat">
                                        <textarea className="input min-h-20" value={chapter.opening_beat || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { opening_beat: event.target.value })} />
                                      </Field>
                                      <Field label="收束 Beat">
                                        <textarea className="input min-h-20" value={chapter.closing_beat || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { closing_beat: event.target.value })} />
                                      </Field>
                                      <Field label="冲突">
                                        <textarea className="input min-h-20" value={chapter.conflict || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { conflict: event.target.value })} />
                                      </Field>
                                      <Field label="节奏">
                                        <input className="input" value={chapter.pacing || ''} onChange={(event) => updateChapter(item.partIndex, item.volumeIndex, chapterIndex, { pacing: event.target.value })} />
                                      </Field>
                                    </div>
                                  </div>
                                </details>
                              ))
                            ) : (
                              <div className="rounded-lg border border-dashed border-[var(--border)]/80 bg-[var(--surface)]/40 p-6 text-center text-sm text-[var(--text-muted)]">
                                此卷还没有章节。先确认卷摘要，再点击“生成此卷章节”。
                              </div>
                            )}
                          </div>
                        </div>
                        )}
                      </section>
                    );
                  })}
                </div>
              </div>
            </section>
          )}
        </main>
      </div>
    </div>
  );
}
