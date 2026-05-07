import { useEffect, useState } from 'react';
import {
  BookOpen,
  Users,
  MapPin,
  Package,
  Sparkles,
  Play,
  CheckCircle,
  Clock,
} from 'lucide-react';
import { getOutline, getCharacters, getLocations, getItems } from '../api';
import type { Project } from '../types';

interface StatCardProps {
  title: string;
  value: string | number;
  icon: React.ElementType;
  color: string;
}

function StatCard({ title, value, icon: Icon, color }: StatCardProps) {
  return (
    <div className="glass rounded-xl p-6 card-hover">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-[var(--text-muted)] text-sm">{title}</p>
          <p className="text-2xl font-bold mt-1">{value}</p>
        </div>
        <div className={`p-3 rounded-lg ${color}`}>
          <Icon className="w-6 h-6" />
        </div>
      </div>
    </div>
  );
}

interface WorkflowStepProps {
  number: number;
  title: string;
  description: string;
  status: 'completed' | 'active' | 'pending';
  onClick: () => void;
}

function WorkflowStep({ number, title, description, status, onClick }: WorkflowStepProps) {
  const statusStyles = {
    completed: 'border-[var(--success)] text-[var(--success)]',
    active: 'border-[var(--primary)] text-[var(--primary)] bg-[var(--primary)]/10',
    pending: 'border-[var(--border)] text-[var(--text-muted)]',
  };

  const iconStyles = {
    completed: <CheckCircle className="w-5 h-5" />,
    active: <Play className="w-5 h-5" />,
    pending: <Clock className="w-5 h-5" />,
  };

  return (
    <button
      onClick={onClick}
      className={`w-full text-left p-4 rounded-xl border-2 transition-all hover:scale-[1.02] ${statusStyles[status]}`}
    >
      <div className="flex items-start gap-4">
        <div className={`flex-shrink-0 w-10 h-10 rounded-full border-2 flex items-center justify-center font-bold ${statusStyles[status]}`}>
          {status === 'completed' ? <CheckCircle className="w-5 h-5" /> : number}
        </div>
        <div className="flex-1">
          <h3 className="font-semibold">{title}</h3>
          <p className="text-sm text-[var(--text-muted)] mt-1">{description}</p>
        </div>
        <div className="flex-shrink-0">{iconStyles[status]}</div>
      </div>
    </button>
  );
}

interface DashboardProps {
  project: Project;
  onTabChange: (tab: string) => void;
}

export function Dashboard({ project, onTabChange }: DashboardProps) {
  const [stats, setStats] = useState({
    chapters: 0,
    characters: 0,
    locations: 0,
    items: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, [project]);

  async function loadData() {
    try {
      // Load stats for the selected project
      const [outline, charactersData, locationsData, itemsData] = await Promise.all([
        getOutline(project.path).catch(() => null),
        getCharacters(project.path).catch(() => ({ characters: [] })),
        getLocations(project.path).catch(() => ({ locations: [] })),
        getItems(project.path).catch(() => ({ items: [] })),
      ]);

      let chapterCount = 0;
      if (outline) {
        outline.parts?.forEach((part: { volumes?: { chapters?: unknown[] }[] }) => {
          part.volumes?.forEach((volume) => {
            chapterCount += volume.chapters?.length || 0;
          });
        });
      }

      setStats({
        chapters: chapterCount,
        characters: charactersData.characters?.length || 0,
        locations: locationsData.locations?.length || 0,
        items: itemsData.items?.length || 0,
      });
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--primary)]"></div>
      </div>
    );
  }



  return (
    <div className="space-y-8 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold mb-2">{project.name}</h1>
        <p className="text-[var(--text-muted)]">
          {project.structure.target_parts} 部 · {project.structure.target_volumes} 卷 · {project.structure.target_chapters} 章
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="章节大纲"
          value={stats.chapters}
          icon={BookOpen}
          color="bg-blue-500/20 text-blue-500"
        />
        <StatCard
          title="角色"
          value={stats.characters}
          icon={Users}
          color="bg-green-500/20 text-green-500"
        />
        <StatCard
          title="地点"
          value={stats.locations}
          icon={MapPin}
          color="bg-purple-500/20 text-purple-500"
        />
        <StatCard
          title="物品"
          value={stats.items}
          icon={Package}
          color="bg-pink-500/20 text-pink-500"
        />
      </div>

      {/* Workflow */}
      <div className="glass rounded-xl p-6">
        <h2 className="text-xl font-bold mb-6 flex items-center gap-2">
          <Sparkles className="w-5 h-5 text-[var(--primary)]" />
          创作流程
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <WorkflowStep
            number={1}
            title="初始化项目"
            description="创建项目结构和基础配置"
            status="completed"
            onClick={() => {}}
          />
          <WorkflowStep
            number={2}
            title="故事设定"
            description="定义类型、前提、主题等"
            status={stats.chapters > 0 ? 'completed' : 'active'}
            onClick={() => onTabChange('setup')}
          />
          <WorkflowStep
            number={3}
            title="大纲生成"
            description="构建部→卷→章的层级大纲"
            status={stats.chapters > 0 ? 'completed' : 'pending'}
            onClick={() => onTabChange('outline')}
          />
          <WorkflowStep
            number={4}
            title="世界元素"
            description="创建角色、地点、物品"
            status={stats.characters > 0 ? 'completed' : stats.chapters > 0 ? 'active' : 'pending'}
            onClick={() => onTabChange('characters')}
          />
          <WorkflowStep
            number={5}
            title="最终章节"
            description="基于大纲和世界元素生成最终章节"
            status={stats.characters > 0 ? 'active' : 'pending'}
            onClick={() => onTabChange('chapters')}
          />
        </div>
      </div>
    </div>
  );
}
