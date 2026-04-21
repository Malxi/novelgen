import { useEffect, useState, useMemo } from 'react';
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
  Gamepad2,
  BarChart3,
} from 'lucide-react';
import { getOutline, createTask, getTask, listSimulationReports, getSimulationReport, listOutlineVersions, restoreOutlineVersion } from '../api';
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
  onComplete: (task?: Task) => void;
  onClose: () => void;
  isVolumeSimulating?: boolean;
  volumeSimComplete?: boolean;
  currentSimIndex?: number;
  totalChapters?: number;
  volumeReports?: Array<{chapterId: string; success: boolean; summary: string}>;
}

function TaskMonitor({ taskId, onComplete, onClose, isVolumeSimulating, volumeSimComplete, currentSimIndex, totalChapters }: TaskMonitorProps) {
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
          // Call onComplete to trigger next chapter in volume simulation
          // But don't close the monitor - let user see results
          if (data.status === 'completed') {
            onComplete(data);
          }
        }
      } catch (err) {
        console.error('Failed to fetch task:', err);
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [taskId, onComplete]);

  if (!task) return null;

  // Calculate volume simulation progress
  const volumeProgress = isVolumeSimulating && totalChapters && totalChapters > 0
    ? Math.round(((currentSimIndex || 0) + (task.status === 'completed' ? 1 : 0)) / totalChapters * 100)
    : null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="glass rounded-xl p-6 w-full max-w-2xl max-h-[80vh] flex flex-col">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            {task.status === 'running' && <Loader2 className="w-5 h-5 text-[var(--primary)] animate-spin" />}
            {task.status === 'completed' && <CheckCircle className="w-5 h-5 text-[var(--success)]" />}
            {task.status === 'failed' && <AlertCircle className="w-5 h-5 text-[var(--error)]" />}
            <h3 className="text-lg font-semibold">
              {isVolumeSimulating ? '整卷RPG模拟' : task.type === 'compose' ? '大纲生成' : '任务'}
              {' - '}
              {volumeSimComplete ? '全部完成' : task.status === 'running' ? '进行中' : task.status === 'completed' ? '完成' : '失败'}
            </h3>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Volume Simulation Progress */}
        {isVolumeSimulating && totalChapters && totalChapters > 0 && (
          <div className="mb-4 p-3 bg-[var(--surface-light)] rounded-lg">
            <div className="flex justify-between text-sm mb-2">
              <span className="text-[var(--text)] font-medium">
                整卷进度: {currentSimIndex !== undefined ? currentSimIndex + 1 : 0} / {totalChapters} 章
              </span>
              <span className="text-[var(--primary)]">{volumeProgress}%</span>
            </div>
            <div className="progress-bar">
              <div
                className="progress-bar-fill"
                style={{ width: `${volumeProgress}%` }}
              />
            </div>
            {volumeSimComplete && (
              <div className="mt-2 text-center text-[var(--success)] font-medium">
                ✅ 整卷模拟全部完成！
              </div>
            )}
          </div>
        )}

        {/* Current Task Progress */}
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
              {volumeSimComplete ? '完成' : '关闭'}
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
            {outlineExists ? '重新生成大纲' : '生成大纲'}
          </h3>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          {outlineExists && (
            <div className="p-3 bg-blue-500/10 border border-blue-500/30 rounded-lg text-sm">
              <p className="text-blue-400 font-medium">ℹ️ 将自动备份</p>
              <p className="text-[var(--text-muted)] mt-1">
                当前项目已有大纲文件。重新生成前会自动备份现有大纲到 <code className="bg-[var(--surface)] px-1 rounded">story/compose/backups/</code> 目录。
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

          <div className="p-3 bg-[var(--surface-light)] rounded-lg text-sm text-[var(--text-muted)]">
            <p>提示：生成大纲需要先在"故事设定"中创建 story_setup.json。</p>
          </div>
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
            className="btn btn-primary flex-1"
          >
            <Play className="w-4 h-4" />
            {outlineExists ? '重新生成' : '开始生成'}
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

interface RPGSimulationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSimulate: (options: { chapterId?: string; volumeId?: string; generateReport?: boolean }) => void;
  outline: Outline | null;
}

function RPGSimulationDialog({ isOpen, onClose, onSimulate, outline }: RPGSimulationDialogProps) {
  const [simType, setSimType] = useState<'chapter' | 'volume'>('chapter');
  const [options, setOptions] = useState({
    chapterId: '',
    volumeId: '',
    generateReport: true,
  });

  // Get all volumes from outline
  const volumes = useMemo(() => {
    if (!outline) return [];
    const list: { id: string; title: string; chapterCount: number }[] = [];
    outline.parts.forEach((part) => {
      part.volumes.forEach((vol) => {
        list.push({
          id: vol.id,
          title: `${part.title} > ${vol.title}`,
          chapterCount: vol.chapters.length,
        });
      });
    });
    return list;
  }, [outline]);

  // Get all chapters from outline
  const chapters = useMemo(() => {
    if (!outline) return [];
    const list: { id: string; title: string; volumeId: string }[] = [];
    outline.parts.forEach((part) => {
      part.volumes.forEach((vol) => {
        vol.chapters.forEach((chap, cIdx) => {
          list.push({
            id: chap.id,
            title: `${part.title} > ${vol.title} > 章${cIdx + 1}: ${chap.title}`,
            volumeId: vol.id,
          });
        });
      });
    });
    return list;
  }, [outline]);

  // Filter chapters by selected volume
  const filteredChapters = useMemo(() => {
    if (!options.volumeId) return chapters;
    return chapters.filter((ch) => ch.volumeId === options.volumeId);
  }, [chapters, options.volumeId]);

  const canSimulate = simType === 'chapter' ? !!options.chapterId : !!options.volumeId;

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="glass rounded-xl p-6 w-full max-w-lg">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <Gamepad2 className="w-5 h-5 text-[var(--primary)]" />
            RPG 模拟验证
          </h3>
          <button onClick={onClose} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div className="p-3 bg-[var(--surface-light)] rounded-lg text-sm">
            <p className="text-[var(--text)]">
              <strong>工作原理：</strong>
            </p>
            <ol className="list-decimal list-inside mt-2 space-y-1 text-[var(--text-muted)]">
              <li>将大纲章节转换为 RPG 游戏场景</li>
              <li>运行 RPG 模拟，验证剧情逻辑</li>
              <li>发现潜在的逻辑漏洞或不合理之处</li>
              <li>生成模拟报告，用于改进大纲</li>
            </ol>
          </div>

          {/* Simulation Type Selection */}
          <div className="flex gap-2">
            <button
              onClick={() => setSimType('chapter')}
              className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                simType === 'chapter'
                  ? 'bg-[var(--primary)] text-white'
                  : 'bg-[var(--surface-light)] text-[var(--text-muted)] hover:bg-[var(--surface)]'
              }`}
            >
              单章模拟
            </button>
            <button
              onClick={() => setSimType('volume')}
              className={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                simType === 'volume'
                  ? 'bg-[var(--primary)] text-white'
                  : 'bg-[var(--surface-light)] text-[var(--text-muted)] hover:bg-[var(--surface)]'
              }`}
            >
              整卷模拟
            </button>
          </div>

          {simType === 'volume' ? (
            <div>
              <label className="block text-sm text-[var(--text-muted)] mb-1">
                选择要模拟的卷 <span className="text-red-400">*</span>
              </label>
              <select
                value={options.volumeId}
                onChange={(e) => setOptions({ ...options, volumeId: e.target.value, chapterId: '' })}
                className="input w-full"
                required
              >
                <option value="">请选择卷...</option>
                {volumes.map((vol) => (
                  <option key={vol.id} value={vol.id}>
                    {vol.title} ({vol.chapterCount} 章)
                  </option>
                ))}
              </select>
              {options.volumeId && (
                <p className="text-xs text-[var(--text-muted)] mt-1">
                  将连续模拟该卷的所有章节
                </p>
              )}
            </div>
          ) : (
            <>
              <div>
                <label className="block text-sm text-[var(--text-muted)] mb-1">
                  选择卷（可选，用于筛选章节）
                </label>
                <select
                  value={options.volumeId}
                  onChange={(e) => setOptions({ ...options, volumeId: e.target.value, chapterId: '' })}
                  className="input w-full"
                >
                  <option value="">全部卷</option>
                  {volumes.map((vol) => (
                    <option key={vol.id} value={vol.id}>
                      {vol.title}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm text-[var(--text-muted)] mb-1">
                  选择要模拟的章节 <span className="text-red-400">*</span>
                </label>
                <select
                  value={options.chapterId}
                  onChange={(e) => setOptions({ ...options, chapterId: e.target.value })}
                  className="input w-full"
                  required
                >
                  <option value="">请选择章节...</option>
                  {filteredChapters.map((chap) => (
                    <option key={chap.id} value={chap.id}>
                      {chap.title}
                    </option>
                  ))}
                </select>
                {chapters.length === 0 && (
                  <p className="text-xs text-yellow-400 mt-1">
                    警告：当前大纲没有章节数据，请先生成大纲
                  </p>
                )}
              </div>
            </>
          )}

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="generateReport"
              checked={options.generateReport}
              onChange={(e) => setOptions({ ...options, generateReport: e.target.checked })}
              className="w-4 h-4 rounded border-[var(--border)] bg-[var(--surface)]"
            />
            <label htmlFor="generateReport" className="text-sm">
              生成详细模拟报告
            </label>
          </div>
        </div>

        <div className="mt-6 flex gap-2">
          <button onClick={onClose} className="btn btn-secondary flex-1">
            取消
          </button>
          <button
            onClick={() => {
              onSimulate(options);
              onClose();
            }}
            disabled={!canSimulate}
            className={`btn flex-1 ${!canSimulate ? 'btn-secondary opacity-50 cursor-not-allowed' : 'btn-primary'}`}
          >
            <Play className="w-4 h-4" />
            {!canSimulate
              ? simType === 'volume'
                ? '请选择卷'
                : '请选择章节'
              : simType === 'volume'
                ? '开始整卷模拟'
                : '开始模拟'}
          </button>
        </div>
      </div>
    </div>
  );
}

interface SimulationReport {
  filename: string;
  chapter_id: string;
  chapter_name: string;
  success: boolean;
}

interface SimulationReportDetail {
  chapter_id: string;
  chapter_name: string;
  success: boolean;
  steps: Array<{
    step_number: number;
    type: string;
    description: string;
    characters?: string[];
    location?: string;
    actions?: string[];
    results?: string[];
  }>;
  player_stats: {
    initial: {
      level: number;
      hp: number;
      max_hp: number;
      mp: number;
      max_mp: number;
      attack: number;
      defense: number;
      exp: number;
    };
    final: {
      level: number;
      hp: number;
      max_hp: number;
      mp: number;
      max_mp: number;
      attack: number;
      defense: number;
      exp: number;
    };
  };
  rewards: Array<{
    name: string;
    type: string;
    quantity?: number;
  }>;
  full_log: Array<{
    timestamp: number;
    type: string;
    message: string;
    details?: Record<string, unknown>;
  }>;
}

export function OutlineViewer({ projectPath }: { projectPath: string }) {
  const [outline, setOutline] = useState<Outline | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedChapter, setSelectedChapter] = useState<Chapter | null>(null);
  const [error, setError] = useState<string | null>(null);
  
  // Task monitoring
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [showTaskMonitor, setShowTaskMonitor] = useState(false);
  
  // Volume simulation queue
  const [volumeSimQueue, setVolumeSimQueue] = useState<string[]>([]);
  const [currentSimIndex, setCurrentSimIndex] = useState(0);
  const [isVolumeSimulating, setIsVolumeSimulating] = useState(false);
  const [volumeSimComplete, setVolumeSimComplete] = useState(false);
  
  // Simulation reports
  const [simulationReports, setSimulationReports] = useState<SimulationReport[]>([]);
  const [showReportsDialog, setShowReportsDialog] = useState(false);
  const [selectedReport, setSelectedReport] = useState<SimulationReportDetail | null>(null);
  const [showReportDetail, setShowReportDetail] = useState(false);
  
  // Dialogs
  const [showGenerateDialog, setShowGenerateDialog] = useState(false);
  const [showImproveDialog, setShowImproveDialog] = useState(false);
  const [showRPGSimDialog, setShowRPGSimDialog] = useState(false);

  // Version management
  const [outlineVersions, setOutlineVersions] = useState<Array<{ filename: string; created_at: string; size: number }>>([]);
  const [showVersionsDialog, setShowVersionsDialog] = useState(false);
  const [restoringVersion, setRestoringVersion] = useState(false);

  useEffect(() => {
    loadOutline();
    loadSimulationReports();
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

  async function loadSimulationReports() {
    try {
      const reports = await listSimulationReports(projectPath);
      setSimulationReports(reports);
    } catch (err) {
      console.error('Failed to load simulation reports:', err);
    }
  }

  async function loadOutlineVersions() {
    try {
      const versions = await listOutlineVersions(projectPath);
      setOutlineVersions(versions);
    } catch (err) {
      console.error('Failed to load outline versions:', err);
    }
  }

  async function handleRestoreVersion(filename: string) {
    if (!confirm(`确定要恢复到版本 ${filename} 吗？当前大纲将被自动备份。`)) {
      return;
    }

    try {
      setRestoringVersion(true);
      await restoreOutlineVersion(filename, projectPath);
      await loadOutline();
      setShowVersionsDialog(false);
      alert('版本恢复成功！');
    } catch (err) {
      console.error('Failed to restore version:', err);
      alert('版本恢复失败：' + (err as Error).message);
    } finally {
      setRestoringVersion(false);
    }
  }

  async function viewReportDetail(chapterId: string) {
    try {
      const report = await getSimulationReport(chapterId, projectPath);
      setSelectedReport(report as SimulationReportDetail);
      setShowReportDetail(true);
    } catch (err) {
      console.error('Failed to load report detail:', err);
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

  async function runRPGSimulation(options: { chapterId?: string; volumeId?: string; generateReport?: boolean }) {
    try {
      // If volumeId is provided, simulate all chapters in the volume sequentially
      if (options.volumeId && outline) {
        // Find all chapters in the selected volume
        const chaptersToSimulate: string[] = [];
        outline.parts.forEach((part) => {
          part.volumes.forEach((vol) => {
            if (vol.id === options.volumeId) {
              vol.chapters.forEach((chap) => {
                chaptersToSimulate.push(chap.id);
              });
            }
          });
        });

        if (chaptersToSimulate.length === 0) {
          console.error('No chapters found in selected volume');
          return;
        }

        console.log(`Starting volume simulation for ${options.volumeId}, chapters:`, chaptersToSimulate);

        // Set up the queue
        setVolumeSimQueue(chaptersToSimulate);
        setCurrentSimIndex(0);
        setIsVolumeSimulating(true);
        setVolumeSimComplete(false);

        // Start with the first chapter
        await simulateChapter(chaptersToSimulate[0], options.generateReport ?? true);
        return;
      }

      // Single chapter simulation
      setIsVolumeSimulating(false);
      setVolumeSimQueue([]);
      if (options.chapterId) {
        await simulateChapter(options.chapterId, options.generateReport ?? true);
      }
    } catch (err) {
      console.error('Failed to run RPG simulation:', err);
    }
  }

  async function simulateChapter(chapterId: string, generateReport: boolean) {
    const args: Record<string, unknown> = {
      project_dir: projectPath,
      subcommand: 'simulate',
      _positional: chapterId,
    };
    if (generateReport) {
      args.report = true;
    }

    const task = await createTask({
      type: 'rpg',
      command: 'rpg',
      args,
    });

    setActiveTaskId(task.id);
    setShowTaskMonitor(true);
  }

  async function handleTaskComplete() {
    // Check if we're in the middle of a volume simulation
    if (isVolumeSimulating && volumeSimQueue.length > 0) {
      const nextIndex = currentSimIndex + 1;
      if (nextIndex < volumeSimQueue.length) {
        // Continue to next chapter
        console.log(`Continuing volume simulation: chapter ${nextIndex + 1}/${volumeSimQueue.length}`);
        setCurrentSimIndex(nextIndex);
        await simulateChapter(volumeSimQueue[nextIndex], true);
        return;
      } else {
        // All chapters completed - don't close monitor, show completion status
        console.log('Volume simulation completed!');
        setVolumeSimComplete(true);
        // Keep isVolumeSimulating true so monitor stays open
        return;
      }
    }
    
    // Only close for single chapter simulation
    setShowTaskMonitor(false);
    setActiveTaskId(null);
    loadOutline();
  }

  function closeVolumeSimulation() {
    setShowTaskMonitor(false);
    setActiveTaskId(null);
    setIsVolumeSimulating(false);
    setVolumeSimQueue([]);
    setCurrentSimIndex(0);
    setVolumeSimComplete(false);
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
            onClick={() => setShowRPGSimDialog(true)}
            className="btn btn-secondary"
          >
            <Gamepad2 className="w-4 h-4" />
            RPG模拟
          </button>
          {simulationReports.length > 0 && (
            <button
              onClick={() => setShowReportsDialog(true)}
              className="btn btn-secondary"
            >
              <BarChart3 className="w-4 h-4" />
              模拟报告 ({simulationReports.length})
            </button>
          )}
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
          <button
            onClick={() => {
              loadOutlineVersions();
              setShowVersionsDialog(true);
            }}
            className="btn btn-secondary"
          >
            <FileText className="w-4 h-4" />
            版本历史
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

      <RPGSimulationDialog
        isOpen={showRPGSimDialog}
        onClose={() => setShowRPGSimDialog(false)}
        onSimulate={runRPGSimulation}
        outline={outline}
      />

      {/* Versions History Dialog */}
      {showVersionsDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="glass rounded-xl p-6 w-full max-w-2xl max-h-[80vh] flex flex-col">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold flex items-center gap-2">
                <FileText className="w-5 h-5 text-[var(--primary)]" />
                大纲版本历史
              </h3>
              <button onClick={() => setShowVersionsDialog(false)} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="flex-1 overflow-auto">
              {outlineVersions.length === 0 ? (
                <div className="text-center py-8 text-[var(--text-muted)]">
                  <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                  <p>暂无备份版本</p>
                  <p className="text-sm mt-2">每次重新生成大纲时会自动创建备份</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {outlineVersions.map((version, index) => (
                    <div
                      key={version.filename}
                      className="p-4 bg-[var(--surface-light)] rounded-lg flex items-center justify-between hover:bg-[var(--surface)] transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-[var(--text)]">
                            版本 {outlineVersions.length - index}
                          </span>
                          {index === 0 && (
                            <span className="px-2 py-0.5 bg-[var(--primary)]/20 text-[var(--primary)] rounded text-xs">
                              最新
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-[var(--text-muted)] mt-1">
                          备份时间: {version.created_at || '未知'}
                        </p>
                        <p className="text-xs text-[var(--text-muted)]">
                          文件大小: {(version.size / 1024).toFixed(2)} KB
                        </p>
                      </div>
                      <button
                        onClick={() => handleRestoreVersion(version.filename)}
                        disabled={restoringVersion}
                        className="btn btn-secondary text-sm"
                      >
                        {restoringVersion ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <RefreshCw className="w-4 h-4" />
                        )}
                        恢复
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="mt-4 pt-4 border-t border-[var(--border)] text-sm text-[var(--text-muted)]">
              <p>💡 提示：恢复版本前，当前大纲会自动备份。恢复后请刷新页面查看更新。</p>
            </div>
          </div>
        </div>
      )}

      {/* Simulation Reports Dialog */}
      {showReportsDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="glass rounded-xl p-6 w-full max-w-2xl max-h-[80vh] flex flex-col">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold flex items-center gap-2">
                <BarChart3 className="w-5 h-5 text-[var(--primary)]" />
                模拟报告列表
              </h3>
              <button onClick={() => setShowReportsDialog(false)} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="flex-1 overflow-auto">
              {simulationReports.length === 0 ? (
                <p className="text-[var(--text-muted)] text-center py-8">暂无模拟报告</p>
              ) : (
                <div className="space-y-2">
                  {simulationReports.map((report) => (
                    <div
                      key={report.chapter_id}
                      className="p-4 bg-[var(--surface-light)] rounded-lg hover:bg-[var(--surface)] cursor-pointer transition-colors"
                      onClick={() => viewReportDetail(report.chapter_id)}
                    >
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="font-medium text-[var(--text)]">{report.chapter_name}</p>
                          <p className="text-sm text-[var(--text-muted)]">章节ID: {report.chapter_id}</p>
                        </div>
                        <div className="flex items-center gap-2">
                          {report.success ? (
                            <span className="px-2 py-1 bg-green-500/20 text-green-500 rounded text-xs">成功</span>
                          ) : (
                            <span className="px-2 py-1 bg-red-500/20 text-red-500 rounded text-xs">失败</span>
                          )}
                          <ChevronRight className="w-4 h-4 text-[var(--text-muted)]" />
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Report Detail Dialog */}
      {showReportDetail && selectedReport && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="glass rounded-xl p-6 w-full max-w-4xl max-h-[90vh] flex flex-col">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  <BarChart3 className="w-5 h-5 text-[var(--primary)]" />
                  模拟报告: {selectedReport.chapter_name}
                </h3>
                <p className="text-sm text-[var(--text-muted)]">章节ID: {selectedReport.chapter_id}</p>
              </div>
              <button onClick={() => setShowReportDetail(false)} className="p-2 hover:bg-[var(--surface-light)] rounded-lg">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="flex-1 overflow-auto space-y-4">
              {/* Success Status */}
              <div className="flex items-center gap-2">
                <span className="text-sm text-[var(--text-muted)]">模拟结果:</span>
                {selectedReport.success ? (
                  <span className="px-3 py-1 bg-green-500/20 text-green-500 rounded-full text-sm font-medium">成功</span>
                ) : (
                  <span className="px-3 py-1 bg-red-500/20 text-red-500 rounded-full text-sm font-medium">失败</span>
                )}
              </div>

              {/* Player Stats */}
              {selectedReport.player_stats && (
                <div className="p-4 bg-[var(--surface-light)] rounded-lg">
                  <h4 className="font-medium mb-3">角色属性变化</h4>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm text-[var(--text-muted)] mb-2">初始状态</p>
                      <div className="space-y-1 text-sm">
                        <p>等级: {selectedReport.player_stats.initial.level}</p>
                        <p>HP: {selectedReport.player_stats.initial.hp}/{selectedReport.player_stats.initial.max_hp}</p>
                        <p>MP: {selectedReport.player_stats.initial.mp}/{selectedReport.player_stats.initial.max_mp}</p>
                        <p>攻击: {selectedReport.player_stats.initial.attack}</p>
                        <p>防御: {selectedReport.player_stats.initial.defense}</p>
                        <p>经验: {selectedReport.player_stats.initial.exp}</p>
                      </div>
                    </div>
                    <div>
                      <p className="text-sm text-[var(--text-muted)] mb-2">最终状态</p>
                      <div className="space-y-1 text-sm">
                        <p>等级: {selectedReport.player_stats.final.level}</p>
                        <p>HP: {selectedReport.player_stats.final.hp}/{selectedReport.player_stats.final.max_hp}</p>
                        <p>MP: {selectedReport.player_stats.final.mp}/{selectedReport.player_stats.final.max_mp}</p>
                        <p>攻击: {selectedReport.player_stats.final.attack}</p>
                        <p>防御: {selectedReport.player_stats.final.defense}</p>
                        <p>经验: {selectedReport.player_stats.final.exp}</p>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Steps */}
              {selectedReport.steps && selectedReport.steps.length > 0 && (
                <div className="p-4 bg-[var(--surface-light)] rounded-lg">
                  <h4 className="font-medium mb-3">模拟步骤</h4>
                  <div className="space-y-2">
                    {selectedReport.steps.map((step, idx) => (
                      <div key={idx} className="p-3 bg-[var(--background)] rounded-lg">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="px-2 py-0.5 bg-[var(--primary)]/20 text-[var(--primary)] rounded text-xs">步骤 {step.step_number}</span>
                          <span className="px-2 py-0.5 bg-[var(--surface)] rounded text-xs text-[var(--text-muted)]">{step.type}</span>
                        </div>
                        <p className="text-sm text-[var(--text)]">{step.description}</p>
                        {step.location && (
                          <p className="text-xs text-[var(--text-muted)] mt-1">地点: {step.location}</p>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Rewards */}
              {selectedReport.rewards && selectedReport.rewards.length > 0 && (
                <div className="p-4 bg-[var(--surface-light)] rounded-lg">
                  <h4 className="font-medium mb-3">获得奖励</h4>
                  <div className="flex flex-wrap gap-2">
                    {selectedReport.rewards.map((reward, idx) => (
                      <span key={idx} className="px-3 py-1 bg-[var(--primary)]/20 text-[var(--primary)] rounded-full text-sm">
                        {reward.name} {reward.quantity && reward.quantity > 1 ? `x${reward.quantity}` : ''}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Full Log */}
              {selectedReport.full_log && selectedReport.full_log.length > 0 && (
                <div className="p-4 bg-[var(--surface-light)] rounded-lg">
                  <h4 className="font-medium mb-3">完整日志</h4>
                  <div className="bg-[var(--background)] rounded-lg p-3 font-mono text-sm max-h-60 overflow-auto">
                    {selectedReport.full_log.map((log, idx) => (
                      <div key={idx} className="py-0.5 text-[var(--text)]">
                        <span className="text-[var(--text-muted)]">[{new Date(log.timestamp * 1000).toLocaleTimeString()}]</span>
                        {' '}
                        <span className={
                          log.type === 'error' ? 'text-red-400' :
                          log.type === 'success' ? 'text-green-400' :
                          log.type === 'warning' ? 'text-yellow-400' :
                          'text-[var(--primary)]'
                        }>
                          [{log.type.toUpperCase()}]
                        </span>
                        {' '}{log.message}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Task Monitor */}
      {showTaskMonitor && activeTaskId && (
        <TaskMonitor
          taskId={activeTaskId}
          onComplete={handleTaskComplete}
          onClose={isVolumeSimulating ? closeVolumeSimulation : () => setShowTaskMonitor(false)}
          isVolumeSimulating={isVolumeSimulating}
          volumeSimComplete={volumeSimComplete}
          currentSimIndex={currentSimIndex}
          totalChapters={volumeSimQueue.length}
        />
      )}
    </div>
  );
}
