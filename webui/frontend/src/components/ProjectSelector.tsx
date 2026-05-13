import { useEffect, useRef, useState } from 'react';
import { BookOpen, Check, ChevronDown, FolderOpen } from 'lucide-react';
import { listProjects } from '../api';
import type { Project } from '../types';

export const DEFAULT_PROJECT_STORAGE_KEY = 'novelgen.defaultProjectPath';

interface ProjectSelectorProps {
  selectedProject: Project | null;
  onSelectProject: (project: Project) => void;
  refreshToken?: number;
}

export function ProjectSelector({ selectedProject, onSelectProject, refreshToken = 0 }: ProjectSelectorProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadProjects();
  }, [refreshToken]);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  async function loadProjects() {
    try {
      setLoading(true);
      const data = await listProjects();
      setProjects(data);

      if (!selectedProject && data.length > 0) {
        const defaultPath = window.localStorage.getItem(DEFAULT_PROJECT_STORAGE_KEY);
        const defaultProject = defaultPath ? data.find((project) => project.path === defaultPath) : null;
        const mineProject = data.find((project) => project.path.includes('mine') || project.name.includes('mine'));
        onSelectProject(defaultProject || mineProject || data[0]);
      }
    } catch (err) {
      console.error('Failed to load projects:', err);
    } finally {
      setLoading(false);
    }
  }

  function handleSelect(project: Project) {
    onSelectProject(project);
    setIsOpen(false);
  }

  if (loading) {
    return (
      <div className="flex w-full items-center gap-2 px-4 py-2 text-[var(--text-muted)]">
        <div className="h-4 w-4 animate-spin rounded-full border-b-2 border-[var(--primary)]"></div>
        <span className="text-sm">加载中...</span>
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="flex w-full items-center gap-2 px-4 py-2 text-[var(--text-muted)]">
        <FolderOpen className="h-4 w-4" />
        <span className="text-sm">无项目</span>
      </div>
    );
  }

  return (
    <div className="relative w-full min-w-0" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        title={selectedProject?.name || '选择项目'}
        className="flex w-full min-w-0 items-center gap-2 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-light)] px-4 py-2 text-left transition-colors hover:bg-[var(--surface)]"
      >
        <BookOpen className="h-4 w-4 flex-none text-[var(--primary)]" />
        <span className="min-w-0 flex-1 truncate">{selectedProject?.name || '选择项目'}</span>
        <ChevronDown className={`h-4 w-4 flex-none transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute left-0 right-0 top-full z-50 mt-2 max-h-[300px] overflow-auto rounded-lg border border-[var(--border)] bg-[var(--surface)] py-2 shadow-xl">
          <div className="px-3 py-2 text-xs uppercase tracking-wider text-[var(--text-muted)]">选择小说项目</div>
          {projects.map((project) => (
            <button
              key={project.path}
              type="button"
              onClick={() => handleSelect(project)}
              className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-[var(--surface-light)] ${
                selectedProject?.path === project.path ? 'bg-[var(--primary)]/10' : ''
              }`}
            >
              <div className="flex h-8 w-8 flex-none items-center justify-center rounded-lg bg-gradient-to-br from-[var(--primary)] to-[var(--secondary)]">
                <BookOpen className="h-4 w-4 text-white" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{project.name}</div>
                <div className="text-xs text-[var(--text-muted)]">
                  {project.structure.target_parts} 部 · {project.structure.target_volumes} 卷 ·{' '}
                  {project.structure.target_chapters} 章
                </div>
              </div>
              {selectedProject?.path === project.path && <Check className="h-4 w-4 flex-none text-[var(--success)]" />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
