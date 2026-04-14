import { useEffect, useState } from 'react';
import {
  FileText,
  RefreshCw,
  Sparkles,
  Eye,
  Download,
  CheckCircle,
} from 'lucide-react';
import { getChapters, getChapter, createTask } from '../api';

interface Chapter {
  id: string;
  name: string;
  content?: string;
}

export function ChaptersViewer({ projectPath }: { projectPath: string }) {
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [selectedChapter, setSelectedChapter] = useState<Chapter | null>(null);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    loadChapters();
  }, [projectPath]);

  async function loadChapters() {
    try {
      setLoading(true);
      const data = await getChapters(projectPath);
      setChapters(data);
    } catch (err) {
      console.error('Failed to load chapters:', err);
    } finally {
      setLoading(false);
    }
  }

  async function viewChapter(chapter: Chapter) {
    try {
      const data = await getChapter(chapter.id, projectPath);
      setSelectedChapter({ ...chapter, content: data.content });
    } catch (err) {
      console.error('Failed to load chapter:', err);
    }
  }

  async function generateChapters() {
    try {
      setGenerating(true);
      await createTask({
        type: 'write',
        command: 'write',
        args: {
          project_dir: projectPath,
          subcommand: 'gen',
          all: true,
        },
      });
      setTimeout(() => loadChapters(), 10000);
    } catch (err) {
      console.error('Failed to generate chapters:', err);
    } finally {
      setGenerating(false);
    }
  }

  async function improveChapters() {
    try {
      setGenerating(true);
      await createTask({
        type: 'write',
        command: 'write',
        args: {
          project_dir: projectPath,
          subcommand: 'improve',
          all: true,
          'max-rounds': 2,
        },
      });
      setTimeout(() => loadChapters(), 10000);
    } catch (err) {
      console.error('Failed to improve chapters:', err);
    } finally {
      setGenerating(false);
    }
  }

  async function exportNovel() {
    try {
      await createTask({
        type: 'export',
        command: 'export',
        args: {
          project_dir: projectPath,
          subcommand: 'novel',
          format: 'md',
        },
      });
    } catch (err) {
      console.error('Failed to export novel:', err);
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
          <h1 className="text-2xl font-bold mb-1">最终章节</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {chapters.length} 个章节</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadChapters}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          {chapters.length > 0 && (
            <button
              onClick={exportNovel}
              className="btn btn-secondary"
            >
              <Download className="w-4 h-4" />
              导出
            </button>
          )}
          {chapters.length === 0 ? (
            <button
              onClick={generateChapters}
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
                  生成章节
                </>
              )}
            </button>
          ) : (
            <button
              onClick={improveChapters}
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
                  改进章节
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* Chapters List */}
      {chapters.length === 0 ? (
        <div className="text-center py-16">
          <FileText className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
          <h2 className="text-xl font-bold mb-2">暂无章节</h2>
          <p className="text-[var(--text-muted)] mb-6">基于草稿生成最终章节</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-1 space-y-2">
            {chapters.map((chapter, idx) => (
              <button
                key={chapter.id}
                onClick={() => viewChapter(chapter)}
                className={`w-full flex items-center gap-3 p-4 rounded-xl text-left transition-colors ${
                  selectedChapter?.id === chapter.id
                    ? 'bg-[var(--primary)]/10 border border-[var(--primary)]'
                    : 'glass hover:bg-[var(--surface-light)]'
                }`}
              >
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-[var(--success)]/20 flex items-center justify-center">
                  <CheckCircle className="w-4 h-4 text-[var(--success)]" />
                </div>
                <span className="flex-1 truncate">第 {idx + 1} 章</span>
                <Eye className="w-4 h-4 text-[var(--text-muted)]" />
              </button>
            ))}
          </div>

          <div className="lg:col-span-2">
            {selectedChapter ? (
              <div className="glass rounded-xl p-6 animate-fade-in">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold">{selectedChapter.name}</h3>
                  <button
                    onClick={() => setSelectedChapter(null)}
                    className="p-2 hover:bg-[var(--surface-light)] rounded-lg"
                  >
                    ✕
                  </button>
                </div>
                <div className="prose prose-invert max-w-none">
                  <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed text-[var(--text)]">
                    {selectedChapter.content}
                  </pre>
                </div>
              </div>
            ) : (
              <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)] h-full flex items-center justify-center">
                <div>
                  <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                  <p>选择一个章节查看内容</p>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
