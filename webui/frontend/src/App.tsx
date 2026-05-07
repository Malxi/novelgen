import { useState, useEffect } from 'react';
import { Layout } from './components/Layout';
import { Dashboard } from './components/Dashboard';
import { OutlineWorkbench } from './components/OutlineWorkbench';
import { StorySetupWorkbench } from './components/StorySetupWorkbench';
import { CharactersViewer } from './components/CharactersViewer';
import { LocationsViewer } from './components/LocationsViewer';
import { ItemsViewer } from './components/ItemsViewer';
import { ChaptersViewer } from './components/ChaptersViewer';
import { TaskManager } from './components/TaskManager';
import { RPGManager } from './components/RPGManager';
import { AICallsViewer } from './components/AICallsViewer';
import { listProjects } from './api';
import type { Project } from './types';

function App() {
  const [activeTab, setActiveTab] = useState('dashboard');
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadProjects();
  }, []);

  async function loadProjects() {
    try {
      const projects = await listProjects();
      if (projects.length > 0) {
        // Prefer 'mine' project if exists, otherwise use first project
        const mineProject = projects.find(p => p.path.includes('mine') || p.name.includes('mine'));
        setSelectedProject(mineProject || projects[0]);
      }
    } catch (err) {
      console.error('Failed to load projects:', err);
    } finally {
      setLoading(false);
    }
  }

  function handleSelectProject(project: Project) {
    setSelectedProject(project);
  }

  function renderContent() {
    if (loading) {
      return (
        <div className="flex items-center justify-center h-full">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--primary)]"></div>
        </div>
      );
    }

    if (!selectedProject) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-[var(--text-muted)]">
          <div className="text-6xl mb-4">📚</div>
          <h2 className="text-2xl font-bold mb-2 text-[var(--text)]">暂无项目</h2>
          <p>请在左侧选择一个项目开始</p>
        </div>
      );
    }

    switch (activeTab) {
      case 'dashboard':
        return <Dashboard project={selectedProject} onTabChange={setActiveTab} />;
      case 'setup':
        return <StorySetupWorkbench projectPath={selectedProject.path} />;
      case 'outline':
        return <OutlineWorkbench projectPath={selectedProject.path} />;
      case 'characters':
        return <CharactersViewer projectPath={selectedProject.path} />;
      case 'locations':
        return <LocationsViewer projectPath={selectedProject.path} />;
      case 'items':
        return <ItemsViewer projectPath={selectedProject.path} />;
      case 'rpg':
        return <RPGManager projectPath={selectedProject.path} />;
      case 'ai-calls':
        return <AICallsViewer projectPath={selectedProject.path} />;
      case 'chapters':
        return <ChaptersViewer projectPath={selectedProject.path} />;
      case 'tasks':
        return <TaskManager />;
      default:
        return <Dashboard project={selectedProject} onTabChange={setActiveTab} />;
    }
  }

  return (
    <Layout 
      activeTab={activeTab} 
      onTabChange={setActiveTab}
      selectedProject={selectedProject}
      onSelectProject={handleSelectProject}
    >
      {renderContent()}
    </Layout>
  );
}

export default App;
