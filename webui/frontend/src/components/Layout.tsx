import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Activity,
  BookOpen,
  Bot,
  ChevronRight,
  FileText,
  LayoutDashboard,
  Loader2,
  MapPin,
  Menu,
  Package,
  RefreshCw,
  RotateCcw,
  Star,
  ScrollText,
  Settings,
  Sparkles,
  Swords,
  Users,
  X,
} from 'lucide-react';
import { ProjectSelector } from './ProjectSelector';
import { createWebSocketConnection, listProjects, listTasks } from '../api';
import { DEFAULT_PROJECT_STORAGE_KEY } from './ProjectSelector';
import type { Project, Task } from '../types';
import type { ReactNode } from 'react';

interface LayoutProps {
  children: ReactNode;
  activeTab: string;
  onTabChange: (tab: string) => void;
  selectedProject: Project | null;
  onSelectProject: (project: Project) => void;
}

const navItems = [
  { id: 'dashboard', label: '概览', icon: LayoutDashboard },
  { id: 'setup', label: '设定', icon: ScrollText },
  { id: 'outline', label: '大纲', icon: BookOpen },
  { id: 'craft', label: '世界元素', icon: Package },
  { id: 'rpg', label: '数值化系统', icon: Swords },
  { id: 'tasks', label: '任务', icon: Activity },
  { id: 'ai-calls', label: 'AI 调用', icon: Bot },
  { id: 'chapters', label: '章节', icon: FileText },
];

const outlineSubItems = [
  { id: 'outline-skeleton', label: '骨架', icon: ScrollText },
  { id: 'outline-volumes', label: '卷', icon: BookOpen },
];

const craftSubItems = [
  { id: 'characters', label: '角色', icon: Users },
  { id: 'locations', label: '地点', icon: MapPin },
  { id: 'items', label: '物品', icon: Package },
];

export function Layout({
  children,
  activeTab,
  onTabChange,
  selectedProject,
  onSelectProject,
}: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [outlineExpanded, setOutlineExpanded] = useState(false);
  const [craftExpanded, setCraftExpanded] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [projectRefreshToken, setProjectRefreshToken] = useState(0);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [now, setNow] = useState(() => Date.now());
  const settingsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let mounted = true;

    listTasks()
      .then((data) => {
        if (mounted) {
          setTasks(sortTasks(data));
        }
      })
      .catch((err) => console.error('Failed to load tasks:', err));

    const ws = createWebSocketConnection((data: unknown) => {
      const msg = data as { type: string; data: Task };
      if (msg.type === 'task_update') {
        setTasks((prev) => upsertTask(prev, msg.data));
      }
    });

    const interval = window.setInterval(() => setNow(Date.now()), 1000);

    return () => {
      mounted = false;
      window.clearInterval(interval);
      ws.close();
    };
  }, []);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (settingsRef.current && !settingsRef.current.contains(event.target as Node)) {
        setSettingsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const activeTasks = useMemo(
    () => tasks.filter((task) => task.status === 'running' || task.status === 'pending'),
    [tasks]
  );
  const visibleTask = activeTasks.find((task) => task.status === 'running') || activeTasks[0];
  const activeTaskCount = activeTasks.length;

  useEffect(() => {
    if (activeTab === 'outline' || activeTab === 'outline-skeleton' || activeTab === 'outline-volumes') setOutlineExpanded(true);
    if (activeTab === 'characters' || activeTab === 'locations' || activeTab === 'items') setCraftExpanded(true);
  }, [activeTab]);

  async function openDefaultProject() {
    const defaultPath = window.localStorage.getItem(DEFAULT_PROJECT_STORAGE_KEY);
    if (!defaultPath) {
      setProjectRefreshToken((token) => token + 1);
      setSettingsOpen(false);
      return;
    }

    try {
      const projects = await listProjects();
      const defaultProject = projects.find((project) => project.path === defaultPath);
      if (defaultProject) {
        onSelectProject(defaultProject);
      } else {
        window.localStorage.removeItem(DEFAULT_PROJECT_STORAGE_KEY);
        setProjectRefreshToken((token) => token + 1);
      }
    } catch (err) {
      console.error('Failed to open default project:', err);
    } finally {
      setSettingsOpen(false);
    }
  }

  function setCurrentProjectAsDefault() {
    if (selectedProject) {
      window.localStorage.setItem(DEFAULT_PROJECT_STORAGE_KEY, selectedProject.path);
    }
    setSettingsOpen(false);
  }

  return (
    <div className="flex h-screen bg-[var(--background)]">
      <aside
        className={`app-sidebar fixed inset-y-0 left-0 z-50 w-64 transition-transform duration-300 lg:static lg:translate-x-0 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex h-16 items-center justify-between border-b border-[var(--border)]/60 px-5">
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[var(--primary)]/15">
              <Sparkles className="h-5 w-5 text-[var(--primary)]" />
            </div>
            <span className="text-xl font-bold gradient-text">NovelGen</span>
          </div>
          <button
            type="button"
            onClick={() => setSidebarOpen(false)}
            className="rounded-lg p-2 hover:bg-[var(--surface-light)] lg:hidden"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="border-b border-[var(--border)]/60 p-4">
          <ProjectSelector
            selectedProject={selectedProject}
            onSelectProject={onSelectProject}
            refreshToken={projectRefreshToken}
          />
        </div>

        <nav className="space-y-1 p-3">
          {navItems.map((item) => {
            if (item.id === 'outline') {
              const active = activeTab === 'outline' || activeTab === 'outline-skeleton' || activeTab === 'outline-volumes';
              return (
                <div key={item.id}>
                  <button
                    type="button"
                    onClick={() => {
                      onTabChange('outline-skeleton');
                      setOutlineExpanded(!outlineExpanded);
                    }}
                    className={`nav-item justify-between ${active ? 'nav-item-active' : ''}`}
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <item.icon className="h-5 w-5 flex-none" />
                      <span className="truncate">{item.label}</span>
                    </div>
                    <ChevronRight className={`h-4 w-4 flex-none transition-transform ${outlineExpanded ? 'rotate-90' : ''}`} />
                  </button>
                  {outlineExpanded && (
                    <div className="ml-4 mt-1 space-y-1">
                      {outlineSubItems.map((subItem) => (
                        <button
                          key={subItem.id}
                          type="button"
                          onClick={() => onTabChange(subItem.id)}
                          className={`nav-item py-2 text-sm ${activeTab === subItem.id || (activeTab === 'outline' && subItem.id === 'outline-skeleton') ? 'nav-item-active' : ''}`}
                        >
                          <subItem.icon className="h-4 w-4 flex-none" />
                          <span className="truncate text-sm">{subItem.label}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            }

            if (item.id === 'craft') {
              const active = activeTab === 'characters' || activeTab === 'locations' || activeTab === 'items';
              return (
                <div key={item.id}>
                  <button
                    type="button"
                    onClick={() => setCraftExpanded(!craftExpanded)}
                    className={`nav-item justify-between ${active ? 'nav-item-active' : ''}`}
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <item.icon className="h-5 w-5 flex-none" />
                      <span className="truncate">{item.label}</span>
                    </div>
                    <ChevronRight className={`h-4 w-4 flex-none transition-transform ${craftExpanded ? 'rotate-90' : ''}`} />
                  </button>
                  {craftExpanded && (
                    <div className="ml-4 mt-1 space-y-1">
                      {craftSubItems.map((subItem) => (
                        <button
                          key={subItem.id}
                          type="button"
                          onClick={() => onTabChange(subItem.id)}
                          className={`nav-item py-2 text-sm ${activeTab === subItem.id ? 'nav-item-active' : ''}`}
                        >
                          <subItem.icon className="h-4 w-4 flex-none" />
                          <span className="truncate text-sm">{subItem.label}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            }

            return (
              <button
                key={item.id}
                type="button"
                onClick={() => onTabChange(item.id)}
                className={`nav-item ${activeTab === item.id ? 'nav-item-active' : ''}`}
              >
                <item.icon className="h-5 w-5 flex-none" />
                <span className="min-w-0 flex-1 truncate text-left">{item.label}</span>
                {item.id === 'tasks' && activeTaskCount > 0 && (
                  <span className="flex h-5 min-w-[1.25rem] flex-none items-center justify-center rounded-full bg-[var(--primary)] px-1.5 text-xs font-bold text-[#041312]">
                    {activeTaskCount}
                  </span>
                )}
              </button>
            );
          })}
        </nav>

        <div className="absolute bottom-0 left-0 right-0 border-t border-[var(--border)]/60 p-3">
          <div className="relative" ref={settingsRef}>
            {settingsOpen && (
              <div className="absolute bottom-full left-0 right-0 z-50 mb-2 rounded-lg border border-[var(--border)] bg-[var(--surface)] p-1.5 shadow-xl">
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-[var(--text-muted)] transition hover:bg-[var(--surface-light)] hover:text-[var(--text)]"
                  disabled={!selectedProject}
                  onClick={setCurrentProjectAsDefault}
                >
                  <Star className="h-4 w-4" />
                  <span>设当前为默认项目</span>
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-[var(--text-muted)] transition hover:bg-[var(--surface-light)] hover:text-[var(--text)]"
                  onClick={openDefaultProject}
                >
                  <BookOpen className="h-4 w-4" />
                  <span>打开默认项目</span>
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-[var(--text-muted)] transition hover:bg-[var(--surface-light)] hover:text-[var(--text)]"
                  onClick={() => {
                    setProjectRefreshToken((token) => token + 1);
                    setSettingsOpen(false);
                  }}
                >
                  <RefreshCw className="h-4 w-4" />
                  <span>刷新项目列表</span>
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-[var(--text-muted)] transition hover:bg-[var(--surface-light)] hover:text-[var(--text)]"
                  onClick={() => window.location.reload()}
                >
                  <RotateCcw className="h-4 w-4" />
                  <span>重新载入界面</span>
                </button>
              </div>
            )}
            <button
              type="button"
              className={`nav-item ${settingsOpen ? 'nav-item-active' : ''}`}
              aria-expanded={settingsOpen}
              onClick={() => setSettingsOpen((open) => !open)}
            >
              <Settings className="h-5 w-5 flex-none" />
              <span>设置</span>
            </button>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="app-topbar flex h-16 items-center justify-between px-5">
          <button
            type="button"
            onClick={() => setSidebarOpen(true)}
            className="rounded-lg p-2 hover:bg-[var(--surface-light)] lg:hidden"
          >
            <Menu className="h-5 w-5" />
          </button>
          <div className="flex min-w-0 items-center gap-4">
            {visibleTask && (
              <button
                type="button"
                onClick={() => onTabChange('tasks')}
                className="hidden max-w-[44rem] items-center gap-3 rounded-lg border border-[var(--primary)]/35 bg-[var(--primary)]/10 px-3 py-1.5 text-left transition hover:border-[var(--primary)]/70 hover:bg-[var(--primary)]/15 md:flex"
                title={visibleTask.message}
              >
                <Loader2 className="h-4 w-4 flex-none animate-spin text-[var(--primary)]" />
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-xs font-semibold text-[var(--primary)]">
                    <span>AI 正在执行</span>
                    {activeTaskCount > 1 && <span>+{activeTaskCount - 1}</span>}
                    <span className="text-[var(--text-muted)]">{formatElapsed(visibleTask.created_at, now)}</span>
                  </div>
                  <div className="max-w-[34rem] truncate text-sm text-[var(--text)]">
                    {visibleTask.message || visibleTask.type}
                  </div>
                  <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-[var(--surface)]">
                    <div
                      className="h-full rounded-full bg-gradient-to-r from-[var(--primary)] to-[var(--secondary)] transition-all"
                      style={{ width: `${Math.max(5, visibleTask.progress)}%` }}
                    />
                  </div>
                </div>
              </button>
            )}
            {visibleTask && (
              <button
                type="button"
                onClick={() => onTabChange('tasks')}
                className="flex h-9 items-center gap-2 rounded-lg border border-[var(--primary)]/35 bg-[var(--primary)]/10 px-2.5 text-sm font-semibold text-[var(--primary)] md:hidden"
              >
                <Loader2 className="h-4 w-4 animate-spin" />
                AI {activeTaskCount}
              </button>
            )}
            {selectedProject && (
              <div className="min-w-0 max-w-[24rem] rounded-lg border border-[var(--border)]/70 bg-[var(--surface)]/70 px-3 py-1.5 text-sm text-[var(--text-muted)]">
                当前项目{' '}
                <span className="inline-block max-w-[16rem] truncate align-bottom font-semibold text-[var(--text)]">
                  {selectedProject.name}
                </span>
              </div>
            )}
          </div>
        </header>

        <main className="flex-1 overflow-auto px-4 py-5 lg:px-6">
          <div className="page-shell">{children}</div>
        </main>
      </div>

      {sidebarOpen && <div className="fixed inset-0 z-40 bg-black/50 lg:hidden" onClick={() => setSidebarOpen(false)} />}
    </div>
  );
}

function sortTasks(tasks: Task[]) {
  return [...tasks].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
}

function upsertTask(tasks: Task[], updatedTask: Task) {
  const exists = tasks.some((task) => task.id === updatedTask.id);
  const next = exists
    ? tasks.map((task) => (task.id === updatedTask.id ? updatedTask : task))
    : [updatedTask, ...tasks];
  return sortTasks(next);
}

function formatElapsed(startedAt: string, now: number) {
  const elapsed = Math.max(0, now - new Date(startedAt).getTime());
  const seconds = Math.floor(elapsed / 1000);

  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }

  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}
