import { useEffect, useState } from 'react';
import {
  FileText,
  RefreshCw,
  Sparkles,
  Eye,
} from 'lucide-react';
import { getDrafts, getDraft, createTask } from '../api';

interface Draft {
  id: string;
  name: string;
  content?: string;
}

export function DraftsViewer({ projectPath }: { projectPath: string }) {
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [selectedDraft, setSelectedDraft] = useState<Draft | null>(null);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [tasks, setTasks] = useState<Record<string, string>>({});

  useEffect(() => {
    loadDrafts();
  }, [projectPath]);

  async function loadDrafts() {
    try {
      setLoading(true);
      const data = await getDrafts(projectPath);
      setDrafts(data);
    } catch (err) {
      console.error('Failed to load drafts:', err);
    } finally {
      setLoading(false);
    }
  }

  async function viewDraft(draft: Draft) {
    try {
      const data = await getDraft(draft.id, projectPath);
      setSelectedDraft({ ...draft, content: data.content });
    } catch (err) {
      console.error('Failed to load draft:', err);
    }
  }

  async function generateDrafts() {
    try {
      setGenerating(true);
      const task = await createTask({
        type: 'draft',
        command: 'draft',
        args: {
          project_dir: projectPath,
          subcommand: 'gen',
          all: true,
        },
      });
      setTasks({ ...tasks, [task.id]: 'generating' });
      setTimeout(() => loadDrafts(), 10000);
    } catch (err) {
      console.error('Failed to generate drafts:', err);
    } finally {
      setGenerating(false);
    }
  }

  async function improveDrafts() {
    try {
      setGenerating(true);
      const task = await createTask({
        type: 'draft',
        command: 'draft',
        args: {
          project_dir: projectPath,
          subcommand: 'improve',
          all: true,
          'max-rounds': 2,
        },
      });
      setTasks({ ...tasks, [task.id]: 'improving' });
      setTimeout(() => loadDrafts(), 10000);
    } catch (err) {
      console.error('Failed to improve drafts:', err);
    } finally {
      setGenerating(false);
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
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold mb-1">草稿管理</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {drafts.length} 个草稿</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadDrafts}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          {drafts.length === 0 ? (
            <button
              onClick={generateDrafts}
              disabled={generating}
              className="btn btn-primary"
            >
              {generating ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                  生成中...
                </>
              ) : (
                <>
                  <Sparkles className="w-4 h-4" />
                  生成草稿
                </>
              )}
            </button>
          ) : (
            <button
              onClick={improveDrafts}
              disabled={generating}
              className="btn btn-primary"
            >
              {generating ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                  改进中...
                </>
              ) : (
                <>
                  <Sparkles className="w-4 h-4" />
                  改进草稿
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* Drafts List */}
      {drafts.length === 0 ? (
        <div className="text-center py-16">
          <FileText className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
          <h2 className="text-xl font-bold mb-2">暂无草稿</h2>
          <p className="text-[var(--text-muted)] mb-6">基于大纲生成草稿章节</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-1 space-y-2">
            {drafts.map((draft) => (
              <button
                key={draft.id}
                onClick={() => viewDraft(draft)}
                className={`w-full flex items-center gap-3 p-4 rounded-xl text-left transition-colors ${
                  selectedDraft?.id === draft.id
                    ? 'bg-[var(--primary)]/10 border border-[var(--primary)]'
                    : 'glass hover:bg-[var(--surface-light)]'
                }`}
              >
                <FileText className="w-5 h-5 text-[var(--primary)]" />
                <span className="flex-1 truncate">{draft.name}</span>
                <Eye className="w-4 h-4 text-[var(--text-muted)]" />
              </button>
            ))}
          </div>

          <div className="lg:col-span-2">
            {selectedDraft ? (
              <div className="glass rounded-xl p-6 animate-fade-in">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold">{selectedDraft.name}</h3>
                  <button
                    onClick={() => setSelectedDraft(null)}
                    className="p-2 hover:bg-[var(--surface-light)] rounded-lg"
                  >
                    ✕
                  </button>
                </div>
                <div className="prose prose-invert max-w-none">
                  <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed text-[var(--text)]">
                    {selectedDraft.content}
                  </pre>
                </div>
              </div>
            ) : (
              <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)] h-full flex items-center justify-center">
                <div>
                  <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                  <p>选择一个草稿查看内容</p>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
