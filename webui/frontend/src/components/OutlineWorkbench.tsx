import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  AlertCircle,
  BookOpen,
  CheckCircle,
  ChevronRight,
  FileJson,
  Layers,
  ListTree,
  Loader2,
  Plus,
  Play,
  RefreshCw,
  Save,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react';
import { createTask, getOutline, getTask, saveJSONFile } from '../api';
import { VersionHistory } from './VersionHistory';
import type { Chapter, Outline, OutlineSelection, Part, Task, Volume } from '../types';

interface OutlineWorkbenchProps {
  projectPath: string;
  view?: OutlineView;
}

type OutlineMode = 'structure' | 'json';
type OutlineView = 'skeleton' | 'volumes';
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
    .split(/\r?\n|;|；|,|，/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value?: string[]): string {
  return (value || []).join('\n');
}

function chapterBeats(chapter: Chapter): string[] {
  const sceneBeats = (chapter.scenes || [])
    .flatMap((scene) => scene.beats || [])
    .map((beat) => beat.trim())
    .filter(Boolean);
  if (sceneBeats.length > 0) return sceneBeats;
  return (chapter.beats || []).map((beat) => beat.trim()).filter(Boolean);
}

function patchChapterBeats(chapter: Chapter, nextBeats: string[]): Partial<Chapter> {
  const beats = nextBeats.map((beat) => beat.trim()).filter(Boolean);
  const scenes = chapter.scenes && chapter.scenes.length > 0
    ? chapter.scenes.map((scene) => ({ ...scene, beats: [...(scene.beats || [])] }))
    : [];

  if (scenes.length > 0) {
    const counts = scenes.map((scene) => Math.max(scene.beats?.length || 0, 1));
    let cursor = 0;
    for (let i = 0; i < scenes.length; i += 1) {
      const remainingScenes = scenes.length - i - 1;
      const remainingBeats = Math.max(beats.length - cursor, 0);
      const count = i === scenes.length - 1
        ? remainingBeats
        : Math.min(counts[i], Math.max(remainingBeats - remainingScenes, 0));
      scenes[i] = { ...scenes[i], beats: beats.slice(cursor, cursor + count) };
      cursor += count;
    }
  } else if (beats.length > 0) {
    scenes.push({ order: 1, beats });
  }

  return {
    beats,
    scenes,
    opening_beat: beats[0] || '',
    closing_beat: beats[beats.length - 1] || '',
  };
}

function isVolumeSelected(selection: OutlineSelection, partIndex: number, volumeIndex: number): boolean {
  return (
    (selection.type === 'volume' || selection.type === 'chapter') &&
    selection.partIndex === partIndex &&
    selection.volumeIndex === volumeIndex
  );
}

function selectionLabel(selection: OutlineSelection, volumes: VolumeRef[]): string {
  if (selection.type === 'skeleton') return '骨架';
  const volume = volumes.find((item) => item.partIndex === selection.partIndex && item.volumeIndex === selection.volumeIndex);
  if (!volume) return '未选择';
  if (selection.type === 'volume') return `V${volume.globalIndex} ${volume.volume.title || '未命名卷'}`;
  return `V${volume.globalIndex} / C${selection.chapterIndex + 1}`;
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

function EditableListField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value?: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
}) {
  const [draftItems, setDraftItems] = useState<string[]>(value && value.length > 0 ? value : ['']);
  const valueKey = (value || []).join('\n');

  useEffect(() => {
    setDraftItems(value && value.length > 0 ? value : ['']);
  }, [valueKey]);

  function commit(next: string[]) {
    onChange(next.map((item) => item.trim()).filter(Boolean));
  }

  return (
    <div className="block">
      <div className="mb-2 flex items-center justify-between gap-3">
        <span className="block text-sm text-[var(--text-muted)]">{label}</span>
        <button
          type="button"
          className="btn btn-secondary text-sm"
          onClick={() => setDraftItems((current) => [...current, ''])}
        >
          <Plus className="h-4 w-4" />
          添加
        </button>
      </div>
      <div className="space-y-2">
        {draftItems.map((item, index) => (
          <div key={index} className="grid grid-cols-[2rem_minmax(0,1fr)_2rem] items-start gap-2">
            <div className="mt-2 flex h-7 w-7 items-center justify-center rounded bg-[var(--surface)] text-xs text-[var(--text-muted)]">
              {index + 1}
            </div>
            <textarea
              className="input min-h-20"
              value={item}
              placeholder={placeholder}
              onChange={(event) => {
                const next = [...draftItems];
                next[index] = event.target.value;
                setDraftItems(next);
                commit(next);
              }}
            />
            <button
              type="button"
              className="mt-1 flex h-8 w-8 items-center justify-center rounded border border-[var(--border)] text-[var(--text-muted)] hover:bg-[var(--surface-light)] hover:text-[var(--error)]"
              aria-label={`删除 ${label} ${index + 1}`}
              onClick={() => {
                const next = draftItems.filter((_, itemIndex) => itemIndex !== index);
                setDraftItems(next.length > 0 ? next : ['']);
                commit(next);
              }}
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
    </div>
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

export function OutlineWorkbench({ projectPath, view = 'skeleton' }: OutlineWorkbenchProps) {
  const [outline, setOutline] = useState<Outline>(emptyOutline);
  const [mode, setMode] = useState<OutlineMode>('structure');
  const [selection, setSelection] = useState<OutlineSelection>({ type: 'skeleton' });
  const [jsonDraft, setJsonDraft] = useState('');
  const [prompt, setPrompt] = useState('');
  const [taskId, setTaskId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);

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

  const selectedVolume = useMemo(() => {
    if (selection.type === 'skeleton') return null;
    return volumes.find((item) => item.partIndex === selection.partIndex && item.volumeIndex === selection.volumeIndex) || null;
  }, [selection, volumes]);

  const selectedChapter = useMemo(() => {
    if (selection.type !== 'chapter' || !selectedVolume) return null;
    return selectedVolume.volume.chapters?.[selection.chapterIndex] || null;
  }, [selection, selectedVolume]);

  useEffect(() => {
    if (view === 'skeleton') {
      if (selection.type !== 'skeleton') setSelection({ type: 'skeleton' });
      return;
    }
    if (view === 'volumes' && selection.type === 'skeleton' && volumes.length > 0) {
      const first = volumes[0];
      setSelection({ type: 'volume', partIndex: first.partIndex, volumeIndex: first.volumeIndex });
    }
  }, [selection.type, view, volumes]);

  useEffect(() => {
    if (selection.type === 'skeleton') return;
    const volume = volumes.find((item) => item.partIndex === selection.partIndex && item.volumeIndex === selection.volumeIndex);
    if (!volume) {
      setSelection({ type: 'skeleton' });
      return;
    }
    if (selection.type === 'chapter' && !volume.volume.chapters?.[selection.chapterIndex]) {
      setSelection({ type: 'volume', partIndex: selection.partIndex, volumeIndex: selection.volumeIndex });
    }
  }, [selection, volumes]);

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

  function switchMode(nextMode: OutlineMode) {
    if (nextMode === 'json') setJsonDraft(JSON.stringify(outline, null, 2));
    setMode(nextMode);
  }

  function renderSkeletonEditor() {
    return (
      <section className="panel panel-pad space-y-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="eyebrow mb-2">Skeleton</p>
            <h2 className="text-xl font-semibold">骨架</h2>
            <p className="mt-1 text-sm text-[var(--text-muted)]">这里维护整本书的部级承诺和每一卷的骨架摘要；章节细节放到“卷”页面处理。</p>
          </div>
          <button className="btn btn-secondary" onClick={() => runComposeTask('storyline-plan', { force: true })}>
            生成卷级 Storyline Plan
          </button>
        </div>

        {outline.parts?.length ? (
          <div className="space-y-4">
            {outline.parts.map((part, partIndex) => (
              <div key={part.id || partIndex} className="soft-card p-4">
                <div className="mb-4 flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-xs text-[var(--text-muted)]">P{partIndex + 1}</p>
                    <h3 className="font-semibold">{part.title || '未命名部'}</h3>
                  </div>
                  <ConfirmButton className="btn btn-secondary text-sm" confirmText="确认重写此部" onConfirm={() => runRegen({ type: 'part', partIndex })}>
                    <Sparkles className="h-4 w-4" />
                    AI 重写
                  </ConfirmButton>
                </div>
                <div className="space-y-4">
                  <Field label="部标题">
                    <input className="input" value={part.title || ''} onChange={(event) => updatePart(partIndex, { title: event.target.value })} />
                  </Field>
                  <Field label="部摘要 / 骨架契约">
                    <textarea className="input min-h-36" value={part.summary || ''} onChange={(event) => updatePart(partIndex, { summary: event.target.value })} />
                  </Field>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between gap-3">
                      <h4 className="font-semibold">卷级骨架</h4>
                      <span className="text-sm text-[var(--text-muted)]">{part.volumes?.length || 0} 卷</span>
                    </div>
                    <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
                      {(part.volumes || []).map((volume, volumeIndex) => (
                        <div key={volume.id || volumeIndex} className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
                          <div className="mb-3 flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <p className="text-xs text-[var(--text-muted)]">
                                V{volumes.find((item) => item.partIndex === partIndex && item.volumeIndex === volumeIndex)?.globalIndex || volumeIndex + 1}
                              </p>
                              <h5 className="truncate font-semibold">{volume.title || '未命名卷'}</h5>
                            </div>
                            <ConfirmButton className="btn btn-secondary text-sm" confirmText="确认重写此卷" onConfirm={() => runRegen({ type: 'volume', partIndex, volumeIndex })}>
                              <Sparkles className="h-4 w-4" />
                              AI
                            </ConfirmButton>
                          </div>
                          <div className="space-y-3">
                            <Field label="卷标题">
                              <input className="input" value={volume.title || ''} onChange={(event) => updateVolume(partIndex, volumeIndex, { title: event.target.value })} />
                            </Field>
                            <Field label="卷骨架 / 章节生成契约">
                              <textarea className="input min-h-32" value={volume.summary || ''} onChange={(event) => updateVolume(partIndex, volumeIndex, { summary: event.target.value })} />
                            </Field>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="hidden flex-wrap gap-2">
                    {(part.volumes || []).map((volume, volumeIndex) => (
                      <button
                        key={volume.id || volumeIndex}
                        type="button"
                        className="rounded border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-left text-sm hover:border-[var(--primary)] hover:bg-[var(--surface-light)]"
                        onClick={() => setSelection({ type: 'volume', partIndex, volumeIndex })}
                      >
                        V{volumes.find((item) => item.partIndex === partIndex && item.volumeIndex === volumeIndex)?.globalIndex || volumeIndex + 1}
                        {' '}
                        {volume.title || '未命名卷'}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-[var(--border)] p-8 text-center text-sm text-[var(--text-muted)]">暂无大纲骨架。</div>
        )}
      </section>
    );
  }

  function renderVolumeEditor() {
    if (!selectedVolume) return null;
    const { partIndex, volumeIndex, globalIndex, part, volume } = selectedVolume;

    return (
      <section className="space-y-4">
        <div className="panel panel-pad space-y-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div className="min-w-0">
              <p className="eyebrow mb-2">{part.title || `第 ${partIndex + 1} 部`}</p>
              <h2 className="text-xl font-semibold">V{globalIndex} {volume.title || '未命名卷'}</h2>
              <p className="mt-1 text-sm text-[var(--text-muted)]">卷级目标、生成约束和本卷章节入口。</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <ConfirmButton
                className="btn btn-secondary"
                confirmText="确认重写此卷"
                onConfirm={() => runRegen({ type: 'volume', partIndex, volumeIndex })}
              >
                <Sparkles className="h-4 w-4" />
                AI 重写此卷
              </ConfirmButton>
              <button
                className="btn btn-secondary"
                onClick={() =>
                  runComposeTask('pipeline', {
                    'from-volume': globalIndex,
                    'to-volume': globalIndex,
                    'skip-improve': true,
                    force: true,
                  })
                }
              >
                <Play className="h-4 w-4" />
                生成章节
              </button>
              <button className="btn btn-primary" onClick={() => runComposeTask('improve', { volume: globalIndex, prompt, force: true, 'max-rounds': 1 })}>
                <Wand2 className="h-4 w-4" />
                Improve
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Field label="卷标题">
              <input className="input" value={volume.title || ''} onChange={(event) => updateVolume(partIndex, volumeIndex, { title: event.target.value })} />
            </Field>
            <Field label="卷 ID">
              <input className="input" value={volume.id || ''} onChange={(event) => updateVolume(partIndex, volumeIndex, { id: event.target.value })} />
            </Field>
            <div className="lg:col-span-2">
              <Field label="卷摘要 / 章节生成契约">
                <textarea className="input min-h-44" value={volume.summary || ''} onChange={(event) => updateVolume(partIndex, volumeIndex, { summary: event.target.value })} />
              </Field>
            </div>
          </div>
        </div>

        <div className="panel panel-pad space-y-3">
          <div className="flex items-center justify-between gap-3">
            <h3 className="font-semibold">本卷章节</h3>
            <span className="text-sm text-[var(--text-muted)]">{volume.chapters?.length || 0} 章</span>
          </div>
          {volume.chapters?.length ? (
            <>
            <div className="space-y-3">
              {volume.chapters.map((chapter, chapterIndex) => (
                <details key={chapter.id || chapterIndex} className="soft-card group/chapter">
                  <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-4 transition-colors hover:bg-[var(--surface-light)]/70">
                    <div className="min-w-0">
                      <p className="text-xs text-[var(--text-muted)]">C{chapterIndex + 1}</p>
                      <h4 className="font-semibold">{chapter.title || chapter.id || '未命名章'}</h4>
                      <p className="mt-2 line-clamp-2 text-sm text-[var(--text-muted)]">{chapter.summary || '未填写章节摘要'}</p>
                      <div className="mt-3 flex flex-wrap gap-2 text-xs">
                        {chapter.location && <span className="rounded bg-[var(--surface)] px-2 py-1 text-[var(--text-muted)]">{chapter.location}</span>}
                        {chapter.pacing && <span className="rounded bg-[var(--primary)]/10 px-2 py-1 text-[var(--primary)]">{chapter.pacing}</span>}
                        <span className="rounded bg-[var(--surface)] px-2 py-1 text-[var(--text-muted)]">{chapterBeats(chapter).length} beats</span>
                      </div>
                    </div>
                    <ChevronRight className="mt-1 h-4 w-4 flex-none text-[var(--text-muted)] transition-transform group-open/chapter:rotate-90" />
                  </summary>
                  <div className="space-y-4 border-t border-[var(--border)]/70 p-4">
                    <div className="flex justify-end">
                      <ConfirmButton
                        className="btn btn-secondary"
                        confirmText="确认重写此章"
                        onConfirm={() => runRegen({ type: 'chapter', partIndex, volumeIndex, chapterIndex })}
                      >
                        <Sparkles className="h-4 w-4" />
                        AI 重写此章
                      </ConfirmButton>
                    </div>
                    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                      <Field label="标题">
                        <input className="input" value={chapter.title || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { title: event.target.value })} />
                      </Field>
                      <Field label="地点">
                        <input className="input" value={chapter.location || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { location: event.target.value })} />
                      </Field>
                      <div className="lg:col-span-2">
                        <Field label="摘要">
                          <textarea className="input min-h-28" value={chapter.summary || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { summary: event.target.value })} />
                        </Field>
                      </div>
                      <div className="lg:col-span-2">
                        <ListField label="登场角色" value={chapter.characters} onChange={(value) => updateChapter(partIndex, volumeIndex, chapterIndex, { characters: value })} minHeight="min-h-24" />
                      </div>
                      <div className="lg:col-span-2">
                        <EditableListField
                          label="Beats"
                          value={chapterBeats(chapter)}
                          placeholder="输入一个独立节拍"
                          onChange={(value) => updateChapter(partIndex, volumeIndex, chapterIndex, patchChapterBeats(chapter, value))}
                        />
                      </div>
                      <Field label="冲突">
                        <textarea className="input min-h-20" value={chapter.conflict || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { conflict: event.target.value })} />
                      </Field>
                      <Field label="节奏">
                        <input className="input" value={chapter.pacing || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { pacing: event.target.value })} />
                      </Field>
                    </div>
                  </div>
                </details>
              ))}
            </div>
            <div className="hidden grid-cols-1 gap-3 xl:grid-cols-2">
              {volume.chapters.map((chapter, chapterIndex) => (
                <button
                  key={chapter.id || chapterIndex}
                  type="button"
                  className="soft-card p-4 text-left transition-colors hover:border-[var(--primary)] hover:bg-[var(--surface-light)]/70"
                  onClick={() => setSelection({ type: 'chapter', partIndex, volumeIndex, chapterIndex })}
                >
                  <div className="mb-2 flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="text-xs text-[var(--text-muted)]">C{chapterIndex + 1}</p>
                      <h4 className="font-semibold">{chapter.title || chapter.id || '未命名章'}</h4>
                    </div>
                    <ChevronRight className="mt-1 h-4 w-4 flex-none text-[var(--text-muted)]" />
                  </div>
                  <p className="line-clamp-2 text-sm text-[var(--text-muted)]">{chapter.summary || '未填写章节摘要'}</p>
                  <div className="mt-3 flex flex-wrap gap-2 text-xs">
                    {chapter.location && <span className="rounded bg-[var(--surface)] px-2 py-1 text-[var(--text-muted)]">{chapter.location}</span>}
                    {chapter.pacing && <span className="rounded bg-[var(--primary)]/10 px-2 py-1 text-[var(--primary)]">{chapter.pacing}</span>}
                    <span className="rounded bg-[var(--surface)] px-2 py-1 text-[var(--text-muted)]">{chapterBeats(chapter).length} beats</span>
                  </div>
                </button>
              ))}
            </div>
            </>
          ) : (
            <div className="rounded-lg border border-dashed border-[var(--border)]/80 bg-[var(--surface)]/40 p-8 text-center text-sm text-[var(--text-muted)]">
              此卷还没有章节。先确认卷摘要，再点击“生成章节”。
            </div>
          )}
        </div>
      </section>
    );
  }

  function renderChapterEditor() {
    if (!selectedVolume || !selectedChapter || selection.type !== 'chapter') return null;
    const { partIndex, volumeIndex, globalIndex, volume } = selectedVolume;
    const chapter = selectedChapter;
    const chapterIndex = selection.chapterIndex;

    return (
      <section className="panel panel-pad space-y-5">
        <div className="flex flex-col gap-3 border-b border-[var(--border)]/70 pb-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0">
            <button className="mb-2 text-sm text-[var(--primary)] hover:underline" onClick={() => setSelection({ type: 'volume', partIndex, volumeIndex })}>
              返回 V{globalIndex} {volume.title || '未命名卷'}
            </button>
            <h2 className="text-xl font-semibold">C{chapterIndex + 1} {chapter.title || chapter.id || '未命名章'}</h2>
            <p className="mt-1 text-sm text-[var(--text-muted)]">章节单独编辑页。Beats、冲突和节奏都在这里处理。</p>
          </div>
          <ConfirmButton
            className="btn btn-secondary"
            confirmText="确认重写此章"
            onConfirm={() => runRegen({ type: 'chapter', partIndex, volumeIndex, chapterIndex })}
          >
            <Sparkles className="h-4 w-4" />
            AI 重写此章
          </ConfirmButton>
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Field label="标题">
            <input className="input" value={chapter.title || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { title: event.target.value })} />
          </Field>
          <Field label="地点">
            <input className="input" value={chapter.location || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { location: event.target.value })} />
          </Field>
          <div className="lg:col-span-2">
            <Field label="摘要">
              <textarea className="input min-h-32" value={chapter.summary || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { summary: event.target.value })} />
            </Field>
          </div>
        </div>

        <div className="space-y-4">
          <ListField label="登场角色" value={chapter.characters} onChange={(value) => updateChapter(partIndex, volumeIndex, chapterIndex, { characters: value })} minHeight="min-h-28" />
          <EditableListField
            label="Beats"
            value={chapterBeats(chapter)}
            placeholder="输入一个独立节拍"
            onChange={(value) => updateChapter(partIndex, volumeIndex, chapterIndex, patchChapterBeats(chapter, value))}
          />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Field label="冲突">
            <textarea className="input min-h-24" value={chapter.conflict || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { conflict: event.target.value })} />
          </Field>
          <Field label="节奏">
            <input className="input" value={chapter.pacing || ''} onChange={(event) => updateChapter(partIndex, volumeIndex, chapterIndex, { pacing: event.target.value })} />
          </Field>
        </div>
      </section>
    );
  }

  function renderOutlineTabs() {
    return (
      <>
      <section className="panel panel-pad space-y-3">
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_18rem] lg:items-end">
          <div>
            <h2 className="font-semibold">卷</h2>
            <p className="text-sm text-[var(--text-muted)]">{outline.parts?.length || 0} 部 · {volumes.length} 卷 · {totalChapters} 章</p>
          </div>
          <Field label="选择卷">
            <select
              className="input"
              value={selectedVolume ? `${selectedVolume.partIndex}:${selectedVolume.volumeIndex}` : ''}
              onChange={(event) => {
                const [partIndex, volumeIndex] = event.target.value.split(':').map((item) => Number(item));
                if (Number.isFinite(partIndex) && Number.isFinite(volumeIndex)) {
                  setSelection({ type: 'volume', partIndex, volumeIndex });
                }
              }}
            >
              {volumes.map((item) => (
                <option key={item.volume.id || `${item.partIndex}-${item.volumeIndex}`} value={`${item.partIndex}:${item.volumeIndex}`}>
                  V{item.globalIndex} {item.volume.title || '未命名卷'} ({item.volume.chapters?.length || 0}章)
                </option>
              ))}
            </select>
          </Field>
        </div>
      </section>
      <div className="hidden">
      <section className="panel panel-pad space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="font-semibold">卷</h2>
            <p className="text-sm text-[var(--text-muted)]">选择当前要编辑的卷；章节从卷页面进入。</p>
          </div>
          <span className="text-sm text-[var(--text-muted)]">{outline.parts?.length || 0} 部 · {volumes.length} 卷 · {totalChapters} 章</span>
        </div>
        <div className="flex gap-2 overflow-x-auto pb-1">
          {volumes.map((item) => {
            const active =
              (selection.type === 'volume' || selection.type === 'chapter') &&
              selection.partIndex === item.partIndex &&
              selection.volumeIndex === item.volumeIndex;
            return (
              <button
                key={item.volume.id || `${item.partIndex}-${item.volumeIndex}`}
                type="button"
                className={`btn whitespace-nowrap ${active ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setSelection({ type: 'volume', partIndex: item.partIndex, volumeIndex: item.volumeIndex })}
                title={item.volume.title || `V${item.globalIndex}`}
              >
                <BookOpen className="h-4 w-4" />
                V{item.globalIndex}
                <span className="max-w-44 truncate">{item.volume.title || '未命名卷'}</span>
                <span className="rounded bg-black/20 px-1.5 text-xs">{item.volume.chapters?.length || 0}</span>
              </button>
            );
          })}
        </div>
      </section>
      </div>
      </>
    );
  }

  return (
    <div className="animate-fade-in space-y-6">
      <div className="workbench-header">
        <div>
          <p className="eyebrow mb-2">Structured collaboration</p>
          <h1 className="mb-1 text-2xl font-bold">大纲协作台</h1>
          <p className="max-w-3xl text-sm text-[var(--text-muted)]">左侧选择骨架、卷或章节；右侧只编辑当前目标，避免章节表单挤在卷列表里。</p>
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
          <button onClick={() => switchMode(mode === 'json' ? 'structure' : 'json')} className="btn btn-secondary">
            <FileJson className="h-4 w-4" />
            {mode === 'json' ? '结构编辑' : 'JSON 编辑'}
          </button>
        </div>
      </div>

      {error && <div className="rounded-lg border border-[var(--error)]/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      <TaskStrip taskId={taskId} onDone={loadOutline} />

      {view === 'skeleton' && (
      <section className="panel panel-pad space-y-4">
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_auto]">
          <Field label="AI 提示">
            <textarea
              className="input min-h-20"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="例如：第一卷强化主角主动选择；每章都要有真实代价；减少巧合。"
            />
          </Field>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:w-[34rem]">
            <ConfirmButton className="btn btn-secondary" confirmText="确认全量生成" onConfirm={() => runComposeTask('gen', { hierarchical: true })}>
              <Play className="h-4 w-4" />
              全量生成
            </ConfirmButton>
            <button className="btn btn-primary" onClick={() => runComposeTask('improve', { prompt, force: true, 'max-rounds': 1 })}>
              <Wand2 className="h-4 w-4" />
              Improve
            </button>
            <button className="btn btn-secondary" onClick={() => runComposeTask('review', { prompt, apply: true })}>
              Review
            </button>
            <button className="btn btn-secondary" onClick={() => runComposeTask('check', {})}>
              检查
            </button>
            <button
              className="btn btn-secondary sm:col-span-2"
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
          </div>
        </div>
        <div className="grid grid-cols-3 gap-3">
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
        </div>
        {emptyVolumes > 0 && <p className="text-sm text-[var(--warning)]">还有 {emptyVolumes} 个空卷等待生成章节。</p>}
      </section>
      )}

      {mode !== 'json' && view === 'volumes' && renderOutlineTabs()}

      <div className="space-y-4">
        <aside className="hidden">
          <section className="panel panel-pad space-y-3">
            <h2 className="panel-title">
              <Sparkles className="h-4 w-4 text-[var(--primary)]" />
              AI / Skeleton
            </h2>
            <Field label="AI 提示">
              <textarea
                className="input min-h-24"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="例如：第一卷强化主角主动选择；每章都要有真实代价；减少巧合。"
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
              <button className="btn btn-secondary" onClick={() => runComposeTask('review', { prompt, apply: true })}>
                Review 后应用
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
          {emptyVolumes > 0 && <p className="text-sm text-[var(--warning)]">还有 {emptyVolumes} 个空卷等待生成章节。</p>}

          <section className="hidden">
            <div className="flex items-center justify-between gap-3">
              <h2 className="panel-title">
                <ListTree className="h-4 w-4 text-[var(--primary)]" />
                大纲导航
              </h2>
              <span className="text-xs text-[var(--text-muted)]">{selectionLabel(selection, volumes)}</span>
            </div>
            <button
              type="button"
              className={`nav-item justify-between ${selection.type === 'skeleton' ? 'nav-item-active' : ''}`}
              onClick={() => setSelection({ type: 'skeleton' })}
            >
              <span className="flex min-w-0 items-center gap-2">
                <Layers className="h-4 w-4 flex-none" />
                <span className="truncate">骨架</span>
              </span>
              <span className="text-xs">{outline.parts?.length || 0} 部</span>
            </button>

            <div className="max-h-[52vh] space-y-3 overflow-auto pr-1">
              {outline.parts?.map((part, partIndex) => (
                <div key={part.id || partIndex} className="space-y-1">
                  <div className="px-2 pt-2 text-xs font-semibold text-[var(--text-muted)]">
                    P{partIndex + 1} {part.title || '未命名部'}
                  </div>
                  {(part.volumes || []).map((volume, volumeIndex) => {
                    const volumeRef = volumes.find((item) => item.partIndex === partIndex && item.volumeIndex === volumeIndex);
                    const activeVolume = isVolumeSelected(selection, partIndex, volumeIndex);
                    return (
                      <div key={volume.id || volumeIndex} className="space-y-1">
                        <button
                          type="button"
                          className={`nav-item justify-between text-left ${selection.type === 'volume' && activeVolume ? 'nav-item-active' : ''}`}
                          onClick={() => setSelection({ type: 'volume', partIndex, volumeIndex })}
                        >
                          <span className="flex min-w-0 items-center gap-2">
                            <BookOpen className="h-4 w-4 flex-none" />
                            <span className="truncate">V{volumeRef?.globalIndex || volumeIndex + 1} {volume.title || '未命名卷'}</span>
                          </span>
                          <span className="text-xs">{volume.chapters?.length || 0}</span>
                        </button>
                        {activeVolume && volume.chapters?.length ? (
                          <div className="ml-5 space-y-1 border-l border-[var(--border)]/70 pl-2">
                            {volume.chapters.map((chapter, chapterIndex) => (
                              <button
                                key={chapter.id || chapterIndex}
                                type="button"
                                className={`flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm transition-colors hover:bg-[var(--surface-light)] ${
                                  selection.type === 'chapter' && selection.chapterIndex === chapterIndex
                                    ? 'bg-[var(--primary)]/14 text-[var(--text)]'
                                    : 'text-[var(--text-muted)]'
                                }`}
                                onClick={() => setSelection({ type: 'chapter', partIndex, volumeIndex, chapterIndex })}
                              >
                                <span className="flex-none text-xs">C{chapterIndex + 1}</span>
                                <span className="truncate">{chapter.title || chapter.id || '未命名章'}</span>
                              </button>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              ))}
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
            <div className="space-y-4">
              {view === 'skeleton' && renderSkeletonEditor()}
              {view === 'volumes' && selection.type === 'volume' && renderVolumeEditor()}
              {view === 'volumes' && selection.type === 'chapter' && renderChapterEditor()}
              {view === 'volumes' && selection.type === 'skeleton' && (
                <section className="panel panel-pad text-sm text-[var(--text-muted)]">暂无卷。请先生成或补全大纲骨架。</section>
              )}
            </div>
          )}
        </main>
        <VersionHistory
          projectPath={projectPath}
          filePath="story/compose/outline.json"
          label="大纲 / outline.json"
          onRestored={loadOutline}
        />
      </div>
    </div>
  );
}
