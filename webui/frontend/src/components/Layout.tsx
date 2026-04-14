import { useState } from 'react';
import {
  BookOpen,
  LayoutDashboard,
  FileText,
  Users,
  MapPin,
  Package,
  Settings,
  Menu,
  X,
  Sparkles,
  ChevronRight,
  Swords,
} from 'lucide-react';
import { ProjectSelector } from './ProjectSelector';
import type { Project } from '../types';

interface LayoutProps {
  children: React.ReactNode;
  activeTab: string;
  onTabChange: (tab: string) => void;
  selectedProject: Project | null;
  onSelectProject: (project: Project) => void;
}

const navItems = [
  { id: 'dashboard', label: '概览', icon: LayoutDashboard },
  { id: 'outline', label: '大纲', icon: BookOpen },
  { id: 'craft', label: '世界元素', icon: Package },
  { id: 'rpg', label: 'RPG数据', icon: Swords },
  { id: 'drafts', label: '草稿', icon: FileText },
  { id: 'chapters', label: '章节', icon: FileText },
];

const craftSubItems = [
  { id: 'characters', label: '角色', icon: Users },
  { id: 'locations', label: '地点', icon: MapPin },
  { id: 'items', label: '物品', icon: Package },
];

export function Layout({ children, activeTab, onTabChange, selectedProject, onSelectProject }: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [craftExpanded, setCraftExpanded] = useState(false);

  return (
    <div className="flex h-screen bg-[var(--background)]">
      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 bg-[var(--surface)] border-r border-[var(--border)] transition-transform duration-300 lg:static lg:translate-x-0 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex items-center justify-between h-16 px-6 border-b border-[var(--border)]">
          <div className="flex items-center gap-2">
            <Sparkles className="w-6 h-6 text-[var(--primary)]" />
            <span className="text-xl font-bold gradient-text">NovelGen</span>
          </div>
          <button
            onClick={() => setSidebarOpen(false)}
            className="lg:hidden p-2 rounded-lg hover:bg-[var(--surface-light)]"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Project Selector */}
        <div className="p-4 border-b border-[var(--border)]">
          <ProjectSelector 
            selectedProject={selectedProject}
            onSelectProject={onSelectProject}
          />
        </div>

        <nav className="p-4 space-y-1">
          {navItems.map((item) => {
            if (item.id === 'craft') {
              return (
                <div key={item.id}>
                  <button
                    onClick={() => setCraftExpanded(!craftExpanded)}
                    className={`w-full flex items-center justify-between gap-3 px-4 py-3 rounded-lg transition-colors ${
                      activeTab.startsWith('craft') || activeTab === 'characters' || activeTab === 'locations' || activeTab === 'items'
                        ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
                        : 'hover:bg-[var(--surface-light)]'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <item.icon className="w-5 h-5" />
                      <span>{item.label}</span>
                    </div>
                    <ChevronRight
                      className={`w-4 h-4 transition-transform ${craftExpanded ? 'rotate-90' : ''}`}
                    />
                  </button>
                  {craftExpanded && (
                    <div className="ml-4 mt-1 space-y-1">
                      {craftSubItems.map((subItem) => (
                        <button
                          key={subItem.id}
                          onClick={() => onTabChange(subItem.id)}
                          className={`w-full flex items-center gap-3 px-4 py-2 rounded-lg transition-colors ${
                            activeTab === subItem.id
                              ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
                              : 'hover:bg-[var(--surface-light)]'
                          }`}
                        >
                          <subItem.icon className="w-4 h-4" />
                          <span className="text-sm">{subItem.label}</span>
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
                onClick={() => onTabChange(item.id)}
                className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${
                  activeTab === item.id
                    ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
                    : 'hover:bg-[var(--surface-light)]'
                }`}
              >
                <item.icon className="w-5 h-5" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-[var(--border)]">
          <button className="w-full flex items-center gap-3 px-4 py-3 rounded-lg hover:bg-[var(--surface-light)] transition-colors">
            <Settings className="w-5 h-5" />
            <span>设置</span>
          </button>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Header */}
        <header className="h-16 flex items-center justify-between px-6 bg-[var(--surface)] border-b border-[var(--border)]">
          <button
            onClick={() => setSidebarOpen(true)}
            className="lg:hidden p-2 rounded-lg hover:bg-[var(--surface-light)]"
          >
            <Menu className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-4">
            {selectedProject && (
              <div className="text-sm text-[var(--text-muted)]">
                当前项目: <span className="text-[var(--text)] font-medium">{selectedProject.name}</span>
              </div>
            )}
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
    </div>
  );
}
