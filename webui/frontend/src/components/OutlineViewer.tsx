import { useEffect, useState } from 'react';
import {
  BookOpen,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  Sparkles,
  X,
  Play,
  Wand2,
  CheckCircle,
  AlertCircle,
  Loader2,
  FileText,
  Eye,
} from 'lucide-react';
import { getOutline, createTask, getTask } from '../api';
import type { Outline, Chapter, Task } from '../types';

interface TreeNodeProps {
  title: string;
  children?: React.ReactNode;
  defaultExpanded?: boolean;
  icon?: React.ElementType;
  actions?: React.ReactNode;
  onClick?: () => void;
}

function TreeNode({ title, children, defaultExpanded = false, icon: Icon, actions, onClick }: TreeNodeProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const hasChildren = !!children;

  return (
    <div className="select-none">
      <div
        className="flex items-center gap-2 py-2 px-3 rounded-lg hover:bg-[var(--surface-light)] cursor-pointer group"
        onClick={() => {
          if (hasChildren) setExpanded(!expanded);
          onClick?.();
        }}
      >
        {hasChildren && (
          expanded ? <ChevronDown className="w-4 h-4 text-[var(--text-muted)]" /> : <ChevronRight className="w-4 h-4 text-[var(--text-muted)]" />
        )}
        {!hasChildren && <span className="w-4" />}
        {Icon && <Icon className="w-4 h-4 text-[var(--primary)]" />}
        <span className="flex-1 truncate">{title}</span>
        {actions && (
          <div className="opacity-0 group-hover:opacity-100 transition-opacity">
            {actions}
          </div>
        )}
      </div>
      {hasChildren && expanded && (
        <div className="ml-4 border-l border-[var(--border)]">
          {children}
        </div>
      )}
    </div>
  );
}

interface ChapterDetailProps {
  chapter: Chapter;
  onClose: () => void;
}

function ChapterDetail({ chapter, onClose }: ChapterDetailProps) {
  return (
    <div className="glass rounded-xl p-6 animate-fade-in">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold">{chapter.title}</h3>
        <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
          <X className="w-5 h-5" />
        </button>
      </div>
      <div className="space-y-4">
        <div>
          <label className="block text-sm text-[var(--text-muted)] mb-1">摘要</label>
          <p className="text-sm text-[var(--text)] bg-[var(--surface-light)] p-3 rounded-lg">
            {chapter.summary}
          </p>
        </div>
        {chapter.characters && chapter.characters.length > 0 && (
          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">出场角色</label>
            <div className="flex flex-wrap gap-2">
              {chapter.characters.map((char, idx) => (
                <span key={idx} className="px-2 py-1 bg-[var(--primary)]/10 text-[var(--primary)] rounded text-sm">
                  {char}
                </span>
              ))}
            </div>
          </div>
        )}
        {chapter.location && (
          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">主要地点</label>
            <p className="text-sm">{chapter.location}</p>
          </div>
        )}
        {chapter.events && chapter.events.length > 0 && (
          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">关键事件</label>
            <div className="space-y-2">
              {chapter.events.map((event, idx) => (
                <div key={idx} className="text-sm bg-[var(--surface-light)] p-3 rounded-lg">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="px-2 py-0.5 bg-[var(--primary)]/20 text-[var(--primary)] rounded text-xs">
                      {event.type}
                    </span>
                    <span className="text-[var(--text-muted)]">→</span>
                    <span className="font-medium">{event.change}</span>
                  </div>
                  <p className="text-[var(--text)]">
                    <span className="text-[var(--text-muted)]">对象:</span> {event.subject}
                  </p>
                  {event.details && (
                    <p className="text-[var(--text-muted)] mt-1 text-xs">{event.details}</p>
                  )}
                  {event.characters.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-2">
                      {event.characters.map((char, cidx) => (
                        <span key={cidx} className="text-xs px-1.5 py-0.5 bg-[var(--surface)] rounded">
                          {char}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
        {chapter.beats && chapter.beats.length > 0 && (
          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">情节节拍</label>
            <div className="space-y-2">
              {chapter.beats.map((beat, idx) => (
                <div key={idx} className="flex items-start gap-2 text-sm">
                  <span className="flex-shrink-0 w-5 h-5 rounded-full bg-[var(--primary)]/20 text-[var(--primary)] flex items-center justify-center text-xs">
                    {idx + 1}
                  </span>
                  <span>{beat}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

interface TaskMonitorProps {
  taskId: string | null;
  onComplete: () => void;
  onClose: () => void;
}

function TaskMonitor({ taskId, onComplete, onClose }: TaskMonitorProps) {
  const [task, setTask] = useState<Task | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    if (!taskId) return;

    console.log('Starting task monitor for task:', taskId);
    const interval = setInterval(async () => {
      try {
        const data = await getTask(taskId);
        console.log('Task status:', data.status, 'progress:', data.progress);
        setTask(data);
        
        if (data.output) {
          const newLogs = data.output.split('\n').filter(l => l.trim());
          setLogs(newLogs.slice(-20)); // Keep last 20 lines
        }

        if (data.status === 'completed' || data.status === 'failed') {
          console.log('Task finished with status:', data.status);
          clearInterval(interval);
          if (data.status === 'completed') {
            setTimeout(onComplete, 1000);
          }
        }
      } catch (err) {
        console.error('Failed to fetch task:', err);
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [taskId, onComplete]);

  if (!task) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="glass rounded-xl p-6 w-full max-w-2xl max-h-[80vh] flex flex-col">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            {task.status === 'running' && <Loader2 className="w-5 h-5 text-[var(--primary)] animate-spin" />}
            {task.status === 'completed' && <CheckCircle className="w-5 h-5 text-[var(--success)]" />}
            {task.status === 'failed' && <AlertCircle className="w-5 h-5 text-[var(--error)]" />}
            <h3 className="text-lg font-semibold">
              {task.type === 'compose' ? '大纲生成' : '任务'} - {task.status === 'running' ? '进行中' : task.status === 'completed' ? '完成' : '失败'}
            </h3>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Progress */}
        <div className="mb-4">
          <div className="flex justify-between text-sm mb-1">
            <span className="text-[var(--text-muted)]">{task.message}</span>
            <span className="text-[var(--primary)]">{task.progress}%</span>
          </div>
          <div className="progress-bar">
            <div
              className="progress-bar-fill"
              style={{ width: `${task.progress}%` }}
            />
          </div>
        </div>

        {/* Logs */}
        <div className="flex-1 bg-[var(--background)] rounded-lg p-4 overflow-auto font-mono text-sm">
          {logs.length === 0 ? (
            <p className="text-[var(--text-muted)]">等待输出...</p>
          ) : (
            logs.map((log, idx) => (
              <div key={idx} className="text-[var(--text)] py-0.5">
                {log}
              </div>
            ))
          )}
        </div>

        {task.status !== 'running' && (
          <div className="mt-4 flex justify-end">
            <button onClick={onClose} className="btn btn-primary">
              关闭
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

interface GenerateDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onGenerate: (options: { hierarchical?: boolean }) => void;
  outlineExists: boolean;
}

function GenerateDialog({ isOpen, onClose, onGenerate, outlineExists }: GenerateDialogProps) {
  const [options, setOptions] = useState({
    hierarchical: false,
  });

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="glass rounded-xl p-6 w-full max-w-md">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-[var(--primary)]" />
            生成大纲
          </h3>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          {outlineExists && (
            <div className="p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg text-sm">
              <p className="text-yellow-400 font-medium">⚠️ 大纲已存在</p>
              <p className="text-[var(--text-muted)] mt-1">
                当前项目已有大纲文件。如需重新生成，请先删除或备份 <code className="bg-[var(--surface)] px-1 rounded">story/compose/outline.json</code> 后再生成。
              </p>
              <p className="text-[var(--text-muted)] mt-2">
                或者使用"改进大纲"功能来优化现有大纲。
              </p>
            </div>
          )}

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="hierarchical"
              checked={options.hierarchical}
              onChange={(e) => setOptions({ ...options, hierarchical: e.target.checked })}
              className="w-4 h-4 rounded border-[var(--border)] bg-[var(--surface)]"
            />
            <label htmlFor="hierarchical" className="text-sm">
              分层生成（质量更好，速度较慢）
            </label>
          </div>

          {!outlineExists && (
            <div className="p-3 bg-[var(--surface-light)] rounded-lg text-sm text-[var(--text-muted)]">
              <p>提示：生成大纲需要先在"故事设定"中创建 story_setup.json。</p>
            </div>
          )}
        </div>

        <div className="mt-6 flex gap-2">
          <button onClick={onClose} className="btn btn-secondary flex-1">
            取消
          </button>
          <button
            onClick={() => {
              onGenerate(options);
              onClose();
            }}
            disabled={outlineExists}
            className={`btn flex-1 ${outlineExists ? 'btn-secondary opacity-50 cursor-not-allowed' : 'btn-primary'}`}
          >
            <Play className="w-4 h-4" />
            {outlineExists ? '大纲已存在' : '开始生成'}
          </button>
        </div>
      </div>
    </div>
  );
}

interface ImproveDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onImprove: (options: { maxRounds?: number; hierarchical?: boolean; force?: boolean; prompt?: string }) => void;
}

function ImproveDialog({ isOpen, onClose, onImprove }: ImproveDialogProps) {
  const [options, setOptions] = useState({
    maxRounds: 2,
    hierarchical: false,
    force: false,
    prompt: '',
  });

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="glass rounded-xl p-6 w-full max-w-md">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <Wand2 className="w-5 h-5 text-[var(--primary)]" />
            改进大纲
          </h3>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">最大改进轮数</label>
            <input
              type="number"
              min={1}
              max={5}
              value={options.maxRounds}
              onChange={(e) => setOptions({ ...options, maxRounds: parseInt(e.target.value) })}
              className="input"
            />
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="hierarchical"
              checked={options.hierarchical}
              onChange={(e) => setOptions({ ...options, hierarchical: e.target.checked })}
              className="w-4 h-4 rounded border-[var(--border)] bg-[var(--surface)]"
            />
            <label htmlFor="hierarchical" className="text-sm">
              分层改进（质量更好，速度较慢）
            </label>
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="force"
              checked={options.force}
              onChange={(e) => setOptions({ ...options, force: e.target.checked })}
              className="w-4 h-4 rounded border-[var(--border)] bg-[var(--surface)]"
            />
            <label htmlFor="force" className="text-sm">
              强制改进（即使评分达标也继续改进）
            </label>
          </div>

          <div>
            <label className="block text-sm text-[var(--text-muted)] mb-1">改进建议</label>
            <textarea
              value={options.prompt}
              onChange={(e) => setOptions({ ...options, prompt: e.target.value })}
              className="input min-h-[80px]"
              placeholder="描述你希望改进的重点，如：增强冲突、丰富角色、调整节奏..."
            />
          </div>
        </div>

        <div className="mt-6 flex gap-2">
          <button onClick={onClose} className="btn btn-secondary flex-1">
            取消
          </button>
          <button
            onClick={() => {
              onImprove(options);
              onClose();
            }}
            className="btn btn-primary flex-1"
          >
            <Wand2 className="w-4 h-4" />
            开始改进
          </button>
        </div>
      </div>
    </div>
  );
}

export function OutlineViewer({ projectPath }: { projectPath: string }) {
  const [outline, setOutline] = useState<Outline | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedChapter, setSelectedChapter] = useState<Chapter | null>(null);
  const [error, setError] = useState<string | null>(null);
  
  // Task monitoring
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [showTaskMonitor, setShowTaskMonitor] = useState(false);
  
  // Dialogs
  const [showGenerateDialog, setShowGenerateDialog] = useState(false);
  const [showImproveDialog, setShowImproveDialog] = useState(false);

  useEffect(() => {
    loadOutline();
  }, [projectPath]);

  async function loadOutline() {
    try {
      setLoading(true);
      console.log('Loading outline for project:', projectPath);
      const data = await getOutline(projectPath);
      console.log('Outline data received:', data);
      console.log('Parts count:', data.parts?.length);
      setOutline(data);
      setError(null);
    } catch (err) {
      console.error('Failed to load outline:', err);
      setError('大纲不存在，请先生成大纲');
      setOutline(null);
    } finally {
      setLoading(false);
    }
  }

  async function generateOutline(options: { hierarchical?: boolean }) {
    try {
      console.log('Generating outline with options:', options);
      const args: Record<string, unknown> = {
        project_dir: projectPath,
        subcommand: 'gen',
      };

      // gen command only supports --hierarchical flag
      if (options.hierarchical) args.hierarchical = true;

      console.log('Task args:', args);
      const task = await createTask({
        type: 'compose',
        command: 'compose',
        args,
      });

      console.log('Task created:', task);
      setActiveTaskId(task.id);
      setShowTaskMonitor(true);
    } catch (err) {
      console.error('Failed to generate outline:', err);
    }
  }

  async function improveOutline(options: { maxRounds?: number; hierarchical?: boolean; force?: boolean; prompt?: string }) {
    try {
      const args: Record<string, unknown> = {
        project_dir: projectPath,
        subcommand: 'improve',
        'max-rounds': options.maxRounds || 2,
      };

      if (options.hierarchical) args.hierarchical = true;
      if (options.force) args.force = true;
      if (options.prompt) {
        args.prompt = options.prompt;
      }

      const task = await createTask({
        type: 'compose',
        command: 'compose',
        args,
      });

      setActiveTaskId(task.id);
      setShowTaskMonitor(true);
    } catch (err) {
      console.error('Failed to improve outline:', err);
    }
  }

  function handleTaskComplete() {
    setShowTaskMonitor(false);
    setActiveTaskId(null);
    loadOutline();
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--primary)]"></div>
      </div>
    );
  }

  if (error || !outline) {
    return (
      <div className="text-center py-16">
        <BookOpen className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
        <h2 className="text-xl font-bold mb-2">大纲不存在</h2>
        <p className="text-[var(--text-muted)] mb-6">使用 AI 生成故事大纲</p>
        <button
          onClick={() => setShowGenerateDialog(true)}
          className="btn btn-primary"
        >
          <Sparkles className="w-4 h-4" />
          生成大纲
        </button>

        <GenerateDialog
          isOpen={showGenerateDialog}
          onClose={() => setShowGenerateDialog(false)}
          onGenerate={generateOutline}
          outlineExists={false}
        />

        {showTaskMonitor && activeTaskId && (
          <TaskMonitor
            taskId={activeTaskId}
            onComplete={handleTaskComplete}
            onClose={() => setShowTaskMonitor(false)}
          />
        )}
      </div>
    );
  }

  const totalVolumes = outline.parts?.reduce((acc, p) => acc + (p.volumes?.length || 0), 0) || 0;
  const totalChapters = outline.parts?.reduce((acc, p) => 
    acc + p.volumes?.reduce((vacc, v) => vacc + (v.chapters?.length || 0), 0) || 0, 0
  ) || 0;

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold mb-1">故事大纲</h1>
          <p className="text-[var(--text-muted)] text-sm">
            {outline.parts?.length || 0} 部 · {totalVolumes} 卷 · {totalChapters} 章
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadOutline}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          <button
            onClick={() => setShowGenerateDialog(true)}
            className="btn btn-secondary"
          >
            <Play className="w-4 h-4" />
            重新生成
          </button>
          <button
            onClick={() => setShowImproveDialog(true)}
            className="btn btn-primary"
          >
            <Wand2 className="w-4 h-4" />
            改进大纲
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="glass rounded-xl p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-[var(--primary)]/20 flex items-center justify-center">
              <BookOpen className="w-5 h-5 text-[var(--primary)]" />
            </div>
            <div>
              <p className="text-2xl font-bold">{outline.parts?.length || 0}</p>
              <p className="text-sm text-[var(--text-muted)]">部</p>
            </div>
          </div>
        </div>
        <div className="glass rounded-xl p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-[var(--secondary)]/20 flex items-center justify-center">
              <FileText className="w-5 h-5 text-[var(--secondary)]" />
            </div>
            <div>
              <p className="text-2xl font-bold">{totalVolumes}</p>
              <p className="text-sm text-[var(--text-muted)]">卷</p>
            </div>
          </div>
        </div>
        <div className="glass rounded-xl p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-green-500/20 flex items-center justify-center">
              <Eye className="w-5 h-5 text-green-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">{totalChapters}</p>
              <p className="text-sm text-[var(--text-muted)]">章</p>
            </div>
          </div>
        </div>
      </div>

      {/* Outline Tree */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 glass rounded-xl p-4">
          <div className="space-y-1">
            {outline.parts?.map((part, partIdx) => (
              <TreeNode
                key={part.id}
                title={`部 ${partIdx + 1}: ${part.title}`}
                defaultExpanded={partIdx === 0}
                icon={BookOpen}
              >
                {part.volumes?.map((volume, volIdx) => (
                  <TreeNode
                    key={volume.id}
                    title={`卷 ${volIdx + 1}: ${volume.title}`}
                    defaultExpanded={partIdx === 0 && volIdx === 0}
                  >
                    {volume.chapters?.map((chapter, chapIdx) => (
                      <TreeNode
                        key={chapter.id}
                        title={`章 ${chapIdx + 1}: ${chapter.title}`}
                        onClick={() => setSelectedChapter(chapter)}
                        actions={
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              setSelectedChapter(chapter);
                            }}
                            className="p-1 hover:bg-[var(--primary)]/20 rounded"
                          >
                            <Eye className="w-4 h-4" />
                          </button>
                        }
                      />
                    ))}
                  </TreeNode>
                ))}
              </TreeNode>
            ))}
          </div>
        </div>

        {/* Chapter Detail */}
        <div>
          {selectedChapter ? (
            <ChapterDetail
              chapter={selectedChapter}
              onClose={() => setSelectedChapter(null)}
            />
          ) : (
            <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)]">
              <Eye className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>选择一个章节查看详情</p>
            </div>
          )}
        </div>
      </div>

      {/* Dialogs */}
      <GenerateDialog
        isOpen={showGenerateDialog}
        onClose={() => setShowGenerateDialog(false)}
        onGenerate={generateOutline}
        outlineExists={!!outline}
      />

      <ImproveDialog
        isOpen={showImproveDialog}
        onClose={() => setShowImproveDialog(false)}
        onImprove={improveOutline}
      />

      {/* Task Monitor */}
      {showTaskMonitor && activeTaskId && (
        <TaskMonitor
          taskId={activeTaskId}
          onComplete={handleTaskComplete}
          onClose={() => setShowTaskMonitor(false)}
        />
      )}
    </div>
  );
}
