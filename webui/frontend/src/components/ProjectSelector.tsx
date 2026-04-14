import { useState, useEffect, useRef } from 'react';
import { BookOpen, ChevronDown, Check, FolderOpen } from 'lucide-react';
import { listProjects } from '../api';
import type { Project } from '../types';

interface ProjectSelectorProps {
  selectedProject: Project | null;
  onSelectProject: (project: Project) => void;
}

export function ProjectSelector({ selectedProject, onSelectProject }: ProjectSelectorProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadProjects();
  }, []);

  useEffect(() => {
    // Close dropdown when clicking outside
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
      
      // If no project selected, select first one (prefer mine)
      if (!selectedProject && data.length > 0) {
        const mineProject = data.find(p => p.path.includes('mine') || p.name.includes('mine'));
        onSelectProject(mineProject || data[0]);
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
      <div className="flex items-center gap-2 px-4 py-2 text-[var(--text-muted)]">
        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-[var(--primary)]"></div>
        <span className="text-sm">加载中...</span>
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="flex items-center gap-2 px-4 py-2 text-[var(--text-muted)]">
        <FolderOpen className="w-4 h-4" />
        <span className="text-sm">无项目</span>
      </div>
    );
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[var(--surface-light)] hover:bg-[var(--surface)] border border-[var(--border)] transition-colors min-w-[200px]"
      >
        <BookOpen className="w-4 h-4 text-[var(--primary)]" />
        <span className="flex-1 text-left truncate">
          {selectedProject?.name || '选择项目'}
        </span>
        <ChevronDown className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 right-0 mt-2 py-2 bg-[var(--surface)] border border-[var(--border)] rounded-lg shadow-xl z-50 max-h-[300px] overflow-auto">
          <div className="px-3 py-2 text-xs text-[var(--text-muted)] uppercase tracking-wider">
            选择小说项目
          </div>
          {projects.map((project) => (
            <button
              key={project.path}
              onClick={() => handleSelect(project)}
              className={`w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-[var(--surface-light)] transition-colors ${
                selectedProject?.path === project.path ? 'bg-[var(--primary)]/10' : ''
              }`}
            >
              <div className="flex-shrink-0 w-8 h-8 rounded-lg bg-gradient-to-br from-[var(--primary)] to-[var(--secondary)] flex items-center justify-center">
                <BookOpen className="w-4 h-4 text-white" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="font-medium truncate">{project.name}</div>
                <div className="text-xs text-[var(--text-muted)]">
                  {project.structure.target_parts}部 · {project.structure.target_volumes}卷 · {project.structure.target_chapters}章
                </div>
              </div>
              {selectedProject?.path === project.path && (
                <Check className="w-4 h-4 text-[var(--success)]" />
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
