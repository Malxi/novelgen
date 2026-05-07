import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  AlertCircle,
  CheckCircle,
  FileJson,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react';
import { createTask, getStorySetup, getTask, getTemplates, saveJSONFile } from '../api';
import { VersionHistory } from './VersionHistory';
import type { Premise, PremiseProgression, StorySetup, Storyline, Task, TemplateLibrary, WorldResource } from '../types';

interface StorySetupWorkbenchProps {
  projectPath: string;
}

type SetupMode = 'components' | 'json';

const emptySetup: StorySetup = {
  project_name: '',
  genres: [],
  premise: '',
  theme: '',
  rules: [],
  target_audience: '',
  tone: '',
  tense: '',
  pov_style: '',
  storylines: [],
  premises: [],
  world_timeline: [],
  world_resources: [],
};

const createStoryline = (): Storyline => ({
  name: '新故事线',
  description: '',
  type: 'subplot',
  importance: 7,
  scope: 'book',
  payoff_style: 'staged_reveal',
  setup_role: '',
  desire: '',
  opposition: '',
  stakes: '',
  open_question: '',
  pressure_points: [],
  key_characters: [],
  must_include: [],
  must_avoid: [],
  required_costs: [],
  antagonist_moves: [],
});

const createPremise = (): Premise => ({
  name: '新升级体系',
  description: '',
  category: '修炼体系',
  progression: [],
});

const createProgressionStage = (level: number): PremiseProgression => ({
  level,
  name: `第 ${level} 阶`,
  description: '',
  requirements: '',
});

const createWorldResource = (): WorldResource => ({
  name: '新资源',
  category: '资源',
  scarcity: '常见',
  description: '',
});

function splitList(value: string): string[] {
  return value
    .split(/\n|;|；|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value?: string[]): string {
  return (value || []).join('\n');
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm text-[var(--text-muted)]">{label}</span>
      {children}
    </label>
  );
}

function ConfirmButton({
  className,
  children,
  confirmText = '确认执行',
  onConfirm,
  title,
}: {
  className: string;
  children: ReactNode;
  confirmText?: string;
  onConfirm: () => void;
  title?: string;
}) {
  const [armed, setArmed] = useState(false);

  return (
    <button
      className={className}
      title={title}
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

export function StorySetupWorkbench({ projectPath }: StorySetupWorkbenchProps) {
  const [setup, setSetup] = useState<StorySetup>(emptySetup);
  const [mode, setMode] = useState<SetupMode>('components');
  const [jsonDraft, setJsonDraft] = useState('');
  const [idea, setIdea] = useState('');
  const [prompt, setPrompt] = useState('');
  const [templates, setTemplates] = useState<TemplateLibrary | null>(null);
  const [selectedProgressionTemplate, setSelectedProgressionTemplate] = useState('');
  const [selectedResourceTierTemplate, setSelectedResourceTierTemplate] = useState('');
  const [taskId, setTaskId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const storylines = setup.storylines || [];
  const premises = setup.premises || [];
  const worldResources = setup.world_resources || [];
  const progressionTemplates = (templates?.templates || []).filter((item) => item.kind === 'progression');
  const resourceTierTemplates = (templates?.templates || []).filter((item) => item.kind === 'resource_tier');

  const texture = useMemo(() => {
    const important = storylines.filter((item) => (item.importance || 0) >= 8).length;
    const contracted = storylines.filter(
      (item) => (item.must_include?.length || 0) > 0 && (item.must_avoid?.length || 0) > 0 && (item.pressure_points?.length || 0) > 0
    ).length;
    return { important, contracted };
  }, [storylines]);

  const loadSetup = useCallback(async () => {
    setError(null);
    try {
      const data = await getStorySetup(projectPath);
      const next = {
        ...emptySetup,
        ...data,
        storylines: data.storylines || [],
        premises: data.premises || [],
        world_resources: data.world_resources || [],
      };
      setSetup(next);
      setJsonDraft(JSON.stringify(next, null, 2));
      setDirty(false);
    } catch (err) {
      setSetup(emptySetup);
      setJsonDraft(JSON.stringify(emptySetup, null, 2));
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [projectPath]);

  const loadTemplates = useCallback(async () => {
    try {
      const data = await getTemplates(projectPath);
      setTemplates(data);
      const firstProgression = data.templates.find((item) => item.kind === 'progression');
      const firstResourceTier = data.templates.find((item) => item.kind === 'resource_tier');
      setSelectedProgressionTemplate((current) => (data.templates.some((item) => item.id === current) ? current : firstProgression?.id || ''));
      setSelectedResourceTierTemplate((current) => (data.templates.some((item) => item.id === current) ? current : firstResourceTier?.id || ''));
    } catch {
      setTemplates(null);
      setSelectedProgressionTemplate('');
      setSelectedResourceTierTemplate('');
    }
  }, [projectPath]);

  useEffect(() => {
    loadSetup();
    loadTemplates();
  }, [loadSetup, loadTemplates]);

  const reloadWorkbench = useCallback(() => {
    loadSetup();
    loadTemplates();
  }, [loadSetup, loadTemplates]);

  async function saveSetup(next = setup) {
    setSaving(true);
    setError(null);
    try {
      await saveJSONFile('story/setup/story_setup.json', next, projectPath);
      setSetup(next);
      setJsonDraft(JSON.stringify(next, null, 2));
      setDirty(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  async function runSetupTask(subcommand: string, args: Record<string, unknown>) {
    setError(null);
    const task = await createTask({
      type: 'setup',
      command: 'setup',
      args: { project_dir: projectPath, subcommand, ...args },
    });
    setTaskId(task.id);
  }

  async function runTemplateTask(subcommand: string, args: Record<string, unknown>) {
    setError(null);
    const task = await createTask({
      type: 'template',
      command: 'template',
      args: { project_dir: projectPath, subcommand, ...args },
    });
    setTaskId(task.id);
  }

  function updateSetupField<K extends keyof StorySetup>(field: K, value: StorySetup[K]) {
    setDirty(true);
    setSetup((current) => ({ ...current, [field]: value }));
  }

  function updateStoryline(index: number, patch: Partial<Storyline>) {
    setDirty(true);
    setSetup((current) => {
      const next = [...(current.storylines || [])];
      next[index] = { ...next[index], ...patch };
      return { ...current, storylines: next };
    });
  }

  function updatePremise(index: number, patch: Partial<Premise>) {
    setDirty(true);
    setSetup((current) => {
      const next = [...(current.premises || [])];
      next[index] = { ...next[index], ...patch };
      return { ...current, premises: next };
    });
  }

  function addPremise() {
    setDirty(true);
    setSetup((current) => ({ ...current, premises: [...(current.premises || []), createPremise()] }));
  }

  function addPremiseWithAI() {
    runSetupTask('improve', {
      prompt: `只在 premises 中添加或完善一套升级体系。用户要求：${prompt.trim() || '补一套能被 outline、craft、RPG-DSL 追踪的升级体系，包含阶段、门槛、代价、资源消耗。'}。不要重写 storylines 和其他设定。`,
      force: true,
      'max-rounds': 1,
    });
  }

  function applyProgressionTemplate() {
    if (!selectedProgressionTemplate) return;
    runTemplateTask('apply', { _positional: selectedProgressionTemplate });
  }

  function applyResourceTierTemplate() {
    if (!selectedResourceTierTemplate) return;
    runTemplateTask('apply', { _positional: selectedResourceTierTemplate });
  }

  function syncDefaultTemplates() {
    runTemplateTask('sync-defaults', {});
  }

  function extractTemplatesFromSetup() {
    runTemplateTask('extract', {});
  }

  function removePremise(index: number) {
    setDirty(true);
    setSetup((current) => ({ ...current, premises: (current.premises || []).filter((_, itemIndex) => itemIndex !== index) }));
  }

  function updateProgressionStage(premiseIndex: number, stageIndex: number, patch: Partial<PremiseProgression>) {
    setDirty(true);
    setSetup((current) => {
      const premises = [...(current.premises || [])];
      const premise = premises[premiseIndex] || createPremise();
      const progression = [...(premise.progression || [])];
      progression[stageIndex] = { ...progression[stageIndex], ...patch };
      premises[premiseIndex] = { ...premise, progression };
      return { ...current, premises };
    });
  }

  function addProgressionStage(premiseIndex: number) {
    setDirty(true);
    setSetup((current) => {
      const premises = [...(current.premises || [])];
      const premise = premises[premiseIndex] || createPremise();
      const progression = [...(premise.progression || [])];
      const nextLevel = progression.reduce((max, stage) => Math.max(max, stage.level || 0), 0) + 1;
      progression.push(createProgressionStage(nextLevel));
      premises[premiseIndex] = { ...premise, progression };
      return { ...current, premises };
    });
  }

  function removeProgressionStage(premiseIndex: number, stageIndex: number) {
    setDirty(true);
    setSetup((current) => {
      const premises = [...(current.premises || [])];
      const premise = premises[premiseIndex] || createPremise();
      premises[premiseIndex] = {
        ...premise,
        progression: (premise.progression || []).filter((_, itemIndex) => itemIndex !== stageIndex),
      };
      return { ...current, premises };
    });
  }

  function updateWorldResource(index: number, patch: Partial<WorldResource>) {
    setDirty(true);
    setSetup((current) => {
      const next = [...(current.world_resources || [])];
      next[index] = { ...next[index], ...patch };
      return { ...current, world_resources: next };
    });
  }

  function addWorldResource() {
    setDirty(true);
    setSetup((current) => ({ ...current, world_resources: [...(current.world_resources || []), createWorldResource()] }));
  }

  function removeWorldResource(index: number) {
    setDirty(true);
    setSetup((current) => ({ ...current, world_resources: (current.world_resources || []).filter((_, itemIndex) => itemIndex !== index) }));
  }

  function addStorylineManually() {
    setDirty(true);
    setSetup((current) => {
      const next = [...(current.storylines || []), createStoryline()];
      return { ...current, storylines: next };
    });
  }

  function addStorylineWithAI() {
    runSetupTask('improve', {
      prompt: `只在 storylines 中添加一条新故事线。用户要求：${prompt.trim() || '补一条能持续影响 outline 的高压故事线'}。不要重写其他设定。`,
      force: true,
      'max-rounds': 1,
    });
  }

  function removeStoryline(index: number) {
    setDirty(true);
    setSetup((current) => {
      const next = (current.storylines || []).filter((_, itemIndex) => itemIndex !== index);
      return { ...current, storylines: next };
    });
  }

  function scrollToSection(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  async function saveJSONDraft() {
    try {
      const parsed = JSON.parse(jsonDraft) as StorySetup;
      await saveSetup({
        ...emptySetup,
        ...parsed,
        storylines: parsed.storylines || [],
        premises: parsed.premises || [],
        world_resources: parsed.world_resources || [],
      });
      setMode('components');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="animate-fade-in space-y-6">
      <div className="workbench-header">
        <div>
          <p className="eyebrow mb-2">Human in the loop</p>
          <h1 className="mb-1 text-2xl font-bold">设定协作台</h1>
          <p className="max-w-3xl text-sm text-[var(--text-muted)]">先让人维护故事契约，再让 AI 生成、审查、局部改进。所有全量覆盖动作都需要二次确认。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {dirty && <span className="badge badge-warning">有未保存修改</span>}
          <button onClick={loadSetup} className="btn btn-secondary">
            <RefreshCw className="h-4 w-4" />
            刷新
          </button>
          <button onClick={() => saveSetup()} disabled={saving} className="btn btn-primary">
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            保存
          </button>
          <button onClick={() => setMode(mode === 'json' ? 'components' : 'json')} className="btn btn-secondary">
            <FileJson className="h-4 w-4" />
            {mode === 'json' ? '组件编辑' : 'JSON 编辑'}
          </button>
        </div>
      </div>

      {error && <div className="rounded-lg border border-[var(--error)]/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
      <TaskStrip taskId={taskId} onDone={reloadWorkbench} />

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <aside className="sticky-rail space-y-4">
          <section className="panel panel-pad space-y-3">
            <h2 className="panel-title">
              <Sparkles className="h-4 w-4 text-[var(--primary)]" />
              全量 AI 操作
            </h2>
            <Field label="Idea / 全局提示">
              <textarea
                className="input min-h-28"
                value={idea}
                onChange={(event) => setIdea(event.target.value)}
                placeholder="从一句创意生成设定。重新生成会覆盖当前设定。"
              />
            </Field>
            <div className="grid grid-cols-2 gap-2">
              <ConfirmButton className="btn btn-secondary" confirmText="确认生成" onConfirm={() => runSetupTask('gen', { _positional: idea || '请基于当前项目方向生成设定' })}>
                生成
              </ConfirmButton>
              <ConfirmButton className="btn btn-secondary" confirmText="确认重写" onConfirm={() => runSetupTask('regen', { prompt: idea })}>
                重新生成
              </ConfirmButton>
            </div>
            <Field label="Improve / Review 提示">
              <textarea
                className="input min-h-24"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="例如：增强三哥能动性；补足主角死亡复活代价；减少巧合和听话角色。"
              />
            </Field>
            <div className="grid grid-cols-1 gap-2">
              <button className="btn btn-primary" onClick={() => runSetupTask('improve', { prompt, force: true, 'max-rounds': 1 })}>
                <Wand2 className="h-4 w-4" />
                根据提示改进
              </button>
              <button className="btn btn-secondary" onClick={() => runSetupTask('storyline-review', { prompt, apply: true })}>
                AI/DSL Review 后应用
              </button>
              <button className="btn btn-secondary" onClick={() => runSetupTask('check', {})}>
                只运行确定性检查
              </button>
            </div>
          </section>

          <section className="grid grid-cols-2 gap-3">
            <div className="metric-card">
              <p className="text-2xl font-bold">{storylines.length}</p>
              <p className="text-xs text-[var(--text-muted)]">故事线</p>
            </div>
            <div className="metric-card">
              <p className="text-2xl font-bold">{texture.contracted}/{texture.important}</p>
              <p className="text-xs text-[var(--text-muted)]">重要线有契约</p>
            </div>
            <div className="metric-card">
              <p className="text-2xl font-bold">{premises.length}</p>
              <p className="text-xs text-[var(--text-muted)]">设定体系</p>
            </div>
            <div className="metric-card">
              <p className="text-2xl font-bold">{worldResources.length}</p>
              <p className="text-xs text-[var(--text-muted)]">核心资源</p>
            </div>
          </section>

          <VersionHistory
            projectPath={projectPath}
            filePath="story/setup/story_setup.json"
            label="设定 / story_setup.json"
            onRestored={loadSetup}
          />
        </aside>

        <main className="min-w-0">
          {mode === 'json' ? (
            <section className="panel panel-pad space-y-3">
              <h2 className="font-semibold">全量 JSON 编辑</h2>
              <textarea className="input min-h-[680px] font-mono text-sm" value={jsonDraft} onChange={(event) => setJsonDraft(event.target.value)} />
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
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('setup-basic')}>基础设定</button>
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('setup-storylines')}>故事线 {storylines.length}</button>
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('setup-premises')}>升级体系 {premises.length}</button>
                  <button className="btn btn-secondary whitespace-nowrap text-sm" onClick={() => scrollToSection('setup-resources')}>核心资源 {worldResources.length}</button>
                </div>
              </div>

              <div id="setup-basic" className="scroll-mt-20 panel panel-pad grid grid-cols-1 gap-4 lg:grid-cols-2">
                <Field label="项目名">
                  <input className="input" value={setup.project_name || ''} onChange={(event) => updateSetupField('project_name', event.target.value)} />
                </Field>
                <ListField label="类型" value={setup.genres} onChange={(value) => updateSetupField('genres', value)} minHeight="min-h-20" />
                <Field label="核心设定">
                  <textarea className="input min-h-28" value={setup.premise || ''} onChange={(event) => updateSetupField('premise', event.target.value)} />
                </Field>
                <Field label="主题">
                  <textarea className="input min-h-28" value={setup.theme || ''} onChange={(event) => updateSetupField('theme', event.target.value)} />
                </Field>
                <ListField label="规则" value={setup.rules} onChange={(value) => updateSetupField('rules', value)} minHeight="min-h-32" />
                <div className="grid grid-cols-1 gap-4">
                  <Field label="读者">
                    <input className="input" value={setup.target_audience || ''} onChange={(event) => updateSetupField('target_audience', event.target.value)} />
                  </Field>
                  <Field label="基调">
                    <input className="input" value={setup.tone || ''} onChange={(event) => updateSetupField('tone', event.target.value)} />
                  </Field>
                  <Field label="视角">
                    <input className="input" value={setup.pov_style || ''} onChange={(event) => updateSetupField('pov_style', event.target.value)} />
                  </Field>
                </div>
              </div>

              <div id="setup-storylines" className="scroll-mt-20 panel panel-pad space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">故事线</h2>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">每条故事线独立折叠编辑，避免左侧列表和右侧预览互相抢位置。</p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button className="btn btn-secondary" onClick={addStorylineManually}>
                      <Plus className="h-4 w-4" />
                      手动添加
                    </button>
                    <button className="btn btn-secondary" onClick={addStorylineWithAI}>
                      <Sparkles className="h-4 w-4" />
                      AI 添加
                    </button>
                  </div>
                </div>

                {storylines.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-[var(--border)] p-6 text-center text-sm text-[var(--text-muted)]">
                    暂无故事线。可以手动添加，或用左侧提示让 AI 添加。
                  </div>
                ) : (
                  <div className="space-y-3">
                    {storylines.map((storyline, storylineIndex) => (
                      <details key={`${storyline.name}-${storylineIndex}`} className="group rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/60">
                        <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-4 transition-colors hover:bg-[var(--surface-light)]/60">
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium">{storyline.name || `故事线 ${storylineIndex + 1}`}</span>
                              <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{storyline.type || '未分类'}</span>
                              <span className="rounded bg-[var(--primary)]/10 px-2 py-0.5 text-xs text-[var(--primary)]">{storyline.importance || 0}/10</span>
                              {storyline.scope && <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{storyline.scope}</span>}
                            </div>
                            <p className="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{storyline.description || '未填写描述'}</p>
                          </div>
                          <span className="mt-1 text-xs text-[var(--text-muted)] group-open:hidden">展开编辑</span>
                          <span className="mt-1 hidden text-xs text-[var(--text-muted)] group-open:inline">收起</span>
                        </summary>

                        <div className="space-y-4 border-t border-[var(--border)]/70 p-4">
                          <div className="flex justify-end gap-2">
                            <button
                              className="btn btn-secondary"
                              onClick={() =>
                                runSetupTask('improve', {
                                  prompt: `只改进 storylines[${storylineIndex}]，名称：${storyline.name}。${prompt || '补足代价、能动性、反派动作、线索契约和 must-avoid。'} 不要重写其他设定。`,
                                  force: true,
                                  'max-rounds': 1,
                                })
                              }
                            >
                              <Wand2 className="h-4 w-4" />
                              AI 改进此线
                            </button>
                            <ConfirmButton className="btn btn-secondary" confirmText="确认移除" onConfirm={() => removeStoryline(storylineIndex)}>
                              <Trash2 className="h-4 w-4" />
                              移除
                            </ConfirmButton>
                          </div>

                          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                            <Field label="名称">
                              <input className="input" value={storyline.name || ''} onChange={(event) => updateStoryline(storylineIndex, { name: event.target.value })} />
                            </Field>
                            <div className="grid grid-cols-2 gap-3">
                              <Field label="类型">
                                <input className="input" value={storyline.type || ''} onChange={(event) => updateStoryline(storylineIndex, { type: event.target.value })} />
                              </Field>
                              <Field label="重要度">
                                <input
                                  className="input"
                                  type="number"
                                  min={1}
                                  max={10}
                                  value={storyline.importance || 1}
                                  onChange={(event) => updateStoryline(storylineIndex, { importance: Number(event.target.value) })}
                                />
                              </Field>
                            </div>
                            <Field label="描述">
                              <textarea className="input min-h-28" value={storyline.description || ''} onChange={(event) => updateStoryline(storylineIndex, { description: event.target.value })} />
                            </Field>
                            <Field label="欲望 / 阻力 / 代价">
                              <textarea
                                className="input min-h-28"
                                value={[storyline.desire, storyline.opposition, storyline.stakes].filter(Boolean).join('\n')}
                                onChange={(event) => {
                                  const [desire = '', opposition = '', stakes = ''] = event.target.value.split('\n');
                                  updateStoryline(storylineIndex, { desire, opposition, stakes });
                                }}
                              />
                            </Field>
                            <ListField label="关键角色" value={storyline.key_characters} onChange={(value) => updateStoryline(storylineIndex, { key_characters: value })} />
                            <ListField label="Pressure Points" value={storyline.pressure_points} onChange={(value) => updateStoryline(storylineIndex, { pressure_points: value })} />
                            <ListField label="Must Include" value={storyline.must_include} onChange={(value) => updateStoryline(storylineIndex, { must_include: value })} minHeight="min-h-28" />
                            <ListField label="Must Avoid" value={storyline.must_avoid} onChange={(value) => updateStoryline(storylineIndex, { must_avoid: value })} minHeight="min-h-28" />
                            <ListField label="Required Costs" value={storyline.required_costs} onChange={(value) => updateStoryline(storylineIndex, { required_costs: value })} />
                            <ListField label="Antagonist Moves" value={storyline.antagonist_moves} onChange={(value) => updateStoryline(storylineIndex, { antagonist_moves: value })} />
                          </div>
                        </div>
                      </details>
                    ))}
                  </div>
                )}
              </div>

              <div id="setup-premises" className="scroll-mt-20 panel panel-pad space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">升级体系 / 设定体系</h2>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">
                      来自 `premises`，包括修炼境界、金手指、阵营等级、敌人等级等可被 outline / craft / RPG-DSL 追踪的体系。
                    </p>
                  </div>
                  <div className="flex flex-wrap justify-end gap-2">
                    <button className="btn btn-secondary" onClick={addPremise}>
                      <Plus className="h-4 w-4" />
                      手动添加
                    </button>
                    <button className="btn btn-secondary" onClick={addPremiseWithAI}>
                      <Sparkles className="h-4 w-4" />
                      AI 添加
                    </button>
                    <button className="btn btn-secondary" onClick={extractTemplatesFromSetup}>
                      扫描提取模板
                    </button>
                    {templates ? (
                      <div className="flex min-w-0 flex-wrap gap-2">
                        <select
                          className="input h-10 min-w-56 py-0 text-sm"
                          value={selectedProgressionTemplate}
                          onChange={(event) => setSelectedProgressionTemplate(event.target.value)}
                        >
                          {progressionTemplates.map((template) => (
                            <option key={template.id} value={template.id}>
                              {template.name}
                            </option>
                          ))}
                        </select>
                        <button className="btn btn-primary" onClick={applyProgressionTemplate} disabled={!selectedProgressionTemplate}>
                          应用模板
                        </button>
                      </div>
                    ) : (
                      <button className="btn btn-secondary" onClick={() => runTemplateTask('init', {})}>
                        初始化模板
                      </button>
                    )}
                  </div>
                </div>

                {premises.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-[var(--border)] p-6 text-center text-sm text-[var(--text-muted)]">
                    暂无升级体系。可以从数值化系统的模板页应用升级体系，或在这里手动添加。
                  </div>
                ) : (
                  <div className="space-y-3">
                    {premises.map((premise, premiseIndex) => (
                      <details key={`${premise.name}-${premiseIndex}`} className="group rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/60">
                        <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-4 transition-colors hover:bg-[var(--surface-light)]/60">
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium">{premise.name || `体系 ${premiseIndex + 1}`}</span>
                              <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{premise.category || '未分类'}</span>
                              <span className="rounded bg-[var(--primary)]/10 px-2 py-0.5 text-xs text-[var(--primary)]">{(premise.progression || []).length} 阶</span>
                            </div>
                            <p className="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{premise.description || '未填写体系描述'}</p>
                          </div>
                          <span className="mt-1 text-xs text-[var(--text-muted)] group-open:hidden">展开编辑</span>
                          <span className="mt-1 hidden text-xs text-[var(--text-muted)] group-open:inline">收起</span>
                        </summary>

                        <div className="border-t border-[var(--border)]/70 p-4">
                          <div className="mb-4 flex items-start justify-between gap-3">
                          <div className="grid flex-1 grid-cols-1 gap-3 lg:grid-cols-[1fr_180px]">
                            <Field label="体系名称">
                              <input className="input" value={premise.name || ''} onChange={(event) => updatePremise(premiseIndex, { name: event.target.value })} />
                            </Field>
                            <Field label="分类">
                              <input className="input" value={premise.category || ''} onChange={(event) => updatePremise(premiseIndex, { category: event.target.value })} />
                            </Field>
                          </div>
                          <ConfirmButton className="rounded-lg p-2 hover:bg-[var(--surface-light)]" confirmText="确认" onConfirm={() => removePremise(premiseIndex)} title="移除此体系">
                            <Trash2 className="h-4 w-4 text-[var(--text-muted)]" />
                          </ConfirmButton>
                          </div>

                          <Field label="体系描述">
                            <textarea className="input min-h-24" value={premise.description || ''} onChange={(event) => updatePremise(premiseIndex, { description: event.target.value })} />
                          </Field>

                          <div className="mt-4 flex items-center justify-between">
                            <h3 className="text-sm font-semibold">阶段 / 等级</h3>
                            <button className="btn btn-secondary text-sm" onClick={() => addProgressionStage(premiseIndex)}>
                              <Plus className="h-4 w-4" />
                              添加阶段
                            </button>
                          </div>

                          <div className="mt-3 space-y-3">
                            {(premise.progression || []).map((stage, stageIndex) => (
                              <details key={`${stage.level}-${stage.name}-${stageIndex}`} className="group/stage rounded-lg bg-[var(--surface-light)]/50">
                                <summary className="flex cursor-pointer list-none items-center justify-between gap-3 p-3 hover:bg-[var(--surface-light)]">
                                  <div className="min-w-0">
                                    <span className="font-medium">Lv.{stage.level || 0} {stage.name || `阶段 ${stageIndex + 1}`}</span>
                                    <p className="mt-1 line-clamp-1 text-xs text-[var(--text-muted)]">{stage.description || stage.requirements || '未填写阶段描述'}</p>
                                  </div>
                                  <span className="text-xs text-[var(--text-muted)] group-open/stage:hidden">编辑</span>
                                  <span className="hidden text-xs text-[var(--text-muted)] group-open/stage:inline">收起</span>
                                </summary>
                                <div className="grid grid-cols-1 gap-3 border-t border-[var(--border)]/60 p-3 xl:grid-cols-[90px_180px_minmax(0,1fr)_minmax(0,1fr)_44px]">
                                  <Field label="等级">
                                    <input
                                      className="input"
                                      type="number"
                                      value={stage.level || 0}
                                      onChange={(event) => updateProgressionStage(premiseIndex, stageIndex, { level: Number(event.target.value) })}
                                    />
                                  </Field>
                                  <Field label="名称">
                                    <input className="input" value={stage.name || ''} onChange={(event) => updateProgressionStage(premiseIndex, stageIndex, { name: event.target.value })} />
                                  </Field>
                                  <Field label="描述">
                                    <textarea className="input min-h-20" value={stage.description || ''} onChange={(event) => updateProgressionStage(premiseIndex, stageIndex, { description: event.target.value })} />
                                  </Field>
                                  <Field label="要求 / 代价">
                                    <textarea className="input min-h-20" value={stage.requirements || ''} onChange={(event) => updateProgressionStage(premiseIndex, stageIndex, { requirements: event.target.value })} />
                                  </Field>
                                  <button className="mt-6 rounded-lg p-2 hover:bg-[var(--surface)]" onClick={() => removeProgressionStage(premiseIndex, stageIndex)} title="移除此阶段">
                                    <Trash2 className="h-4 w-4 text-[var(--text-muted)]" />
                                  </button>
                                </div>
                              </details>
                            ))}
                          </div>
                        </div>
                      </details>
                    ))}
                  </div>
                )}
              </div>

              <div id="setup-resources" className="scroll-mt-20 panel panel-pad space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">核心资源 / 物品等级标记</h2>
                    <p className="mt-1 text-sm text-[var(--text-muted)]">来自 `world_resources`，包括灵石、寿元、丹药、法器、仙宝和物品等级模板等。</p>
                  </div>
                  <div className="flex flex-wrap justify-end gap-2">
                    <button className="btn btn-secondary" onClick={addWorldResource}>
                      <Plus className="h-4 w-4" />
                      添加资源
                    </button>
                    {templates ? (
                      <>
                        <button className="btn btn-secondary" onClick={syncDefaultTemplates}>
                          同步默认模板
                        </button>
                        <div className="flex min-w-0 flex-wrap gap-2">
                          <select
                            className="input h-10 min-w-56 py-0 text-sm"
                            value={selectedResourceTierTemplate}
                            onChange={(event) => setSelectedResourceTierTemplate(event.target.value)}
                          >
                            {resourceTierTemplates.length === 0 ? (
                              <option value="">暂无资源等级模板</option>
                            ) : (
                              resourceTierTemplates.map((template) => (
                                <option key={template.id} value={template.id}>
                                  {template.name}
                                </option>
                              ))
                            )}
                          </select>
                          <button className="btn btn-primary" onClick={applyResourceTierTemplate} disabled={!selectedResourceTierTemplate}>
                            应用模板
                          </button>
                        </div>
                      </>
                    ) : (
                      <button className="btn btn-secondary" onClick={() => runTemplateTask('init', {})}>
                        初始化模板
                      </button>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                  {worldResources.map((resource, resourceIndex) => (
                    <details key={`${resource.name}-${resourceIndex}`} className="group rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/60">
                      <summary className="flex cursor-pointer list-none items-start justify-between gap-3 p-4 transition-colors hover:bg-[var(--surface-light)]/60">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-medium">{resource.name || `资源 ${resourceIndex + 1}`}</span>
                            <span className="rounded bg-[var(--surface-light)] px-2 py-0.5 text-xs text-[var(--text-muted)]">{resource.category || '未分类'}</span>
                            <span className="rounded bg-[var(--primary)]/10 px-2 py-0.5 text-xs text-[var(--primary)]">{resource.scarcity || '未标记'}</span>
                          </div>
                          <p className="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{resource.description || '未填写资源描述'}</p>
                        </div>
                        <span className="mt-1 text-xs text-[var(--text-muted)] group-open:hidden">展开编辑</span>
                        <span className="mt-1 hidden text-xs text-[var(--text-muted)] group-open:inline">收起</span>
                      </summary>
                      <div className="border-t border-[var(--border)]/70 p-4">
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div className="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2">
                            <Field label="名称">
                              <input className="input" value={resource.name || ''} onChange={(event) => updateWorldResource(resourceIndex, { name: event.target.value })} />
                            </Field>
                            <Field label="稀缺度">
                              <input className="input" value={resource.scarcity || ''} onChange={(event) => updateWorldResource(resourceIndex, { scarcity: event.target.value })} />
                            </Field>
                          </div>
                          <button className="rounded-lg p-2 hover:bg-[var(--surface-light)]" onClick={() => removeWorldResource(resourceIndex)} title="移除此资源">
                            <Trash2 className="h-4 w-4 text-[var(--text-muted)]" />
                          </button>
                        </div>
                        <Field label="分类">
                          <input className="input" value={resource.category || ''} onChange={(event) => updateWorldResource(resourceIndex, { category: event.target.value })} />
                        </Field>
                        <div className="mt-3">
                          <Field label="描述">
                            <textarea className="input min-h-24" value={resource.description || ''} onChange={(event) => updateWorldResource(resourceIndex, { description: event.target.value })} />
                          </Field>
                        </div>
                      </div>
                    </details>
                  ))}
                </div>
              </div>

            </section>
          )}
        </main>
      </div>
    </div>
  );
}
