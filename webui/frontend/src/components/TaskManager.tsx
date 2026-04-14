import { useEffect, useState } from 'react';
import {
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  Trash2,
  RefreshCw,
} from 'lucide-react';
import { listTasks, deleteTask, createWebSocketConnection } from '../api';
import type { Task } from '../types';

export function TaskManager() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadTasks();

    // Setup WebSocket for real-time updates
    const ws = createWebSocketConnection((data: unknown) => {
      const msg = data as { type: string; data: Task };
      if (msg.type === 'task_update') {
        const updatedTask = msg.data;
        setTasks((prev) =>
          prev.map((t) => (t.id === updatedTask.id ? updatedTask : t))
        );
      }
    });

    return () => {
      ws.close();
    };
  }, []);

  async function loadTasks() {
    try {
      setLoading(true);
      const data = await listTasks();
      setTasks(data.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
    } catch (err) {
      console.error('Failed to load tasks:', err);
    } finally {
      setLoading(false);
    }
  }

  async function handleDeleteTask(id: string) {
    try {
      await deleteTask(id);
      setTasks(tasks.filter((t) => t.id !== id));
    } catch (err) {
      console.error('Failed to delete task:', err);
    }
  }

  function getStatusIcon(status: string) {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-5 h-5 text-[var(--success)]" />;
      case 'failed':
        return <XCircle className="w-5 h-5 text-[var(--error)]" />;
      case 'running':
        return <Loader2 className="w-5 h-5 text-[var(--primary)] animate-spin" />;
      default:
        return <Clock className="w-5 h-5 text-[var(--text-muted)]" />;
    }
  }

  function getStatusBadge(status: string) {
    switch (status) {
      case 'completed':
        return <span className="badge badge-success">已完成</span>;
      case 'failed':
        return <span className="badge badge-error">失败</span>;
      case 'running':
        return <span className="badge badge-info">运行中</span>;
      default:
        return <span className="badge">等待中</span>;
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
          <h1 className="text-2xl font-bold mb-1">任务管理</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {tasks.length} 个任务</p>
        </div>
        <button
          onClick={loadTasks}
          className="btn btn-secondary"
        >
          <RefreshCw className="w-4 h-4" />
          刷新
        </button>
      </div>

      {/* Task List */}
      {tasks.length === 0 ? (
        <div className="text-center py-16 text-[var(--text-muted)]">
          <Clock className="w-16 h-16 mx-auto mb-4 opacity-50" />
          <p>暂无任务</p>
        </div>
      ) : (
        <div className="space-y-3">
          {tasks.map((task) => (
            <div
              key={task.id}
              className="glass rounded-xl p-4 hover:bg-[var(--surface-light)] transition-colors"
            >
              <div className="flex items-center gap-4">
                {getStatusIcon(task.status)}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{task.type}</span>
                    {getStatusBadge(task.status)}
                  </div>
                  <p className="text-sm text-[var(--text-muted)] truncate">
                    {task.message}
                  </p>
                  {task.status === 'running' && (
                    <div className="mt-2">
                      <div className="progress-bar">
                        <div
                          className="progress-bar-fill"
                          style={{ width: `${task.progress}%` }}
                        />
                      </div>
                      <p className="text-xs text-[var(--text-muted)] mt-1">
                        {task.progress}%
                      </p>
                    </div>
                  )}
                </div>
                <button
                  onClick={() => handleDeleteTask(task.id)}
                  className="p-2 hover:bg-[var(--error)]/20 hover:text-[var(--error)] rounded-lg transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
              {task.output && (
                <div className="mt-3 p-3 bg-[var(--surface-light)] rounded-lg">
                  <pre className="text-xs text-[var(--text-muted)] whitespace-pre-wrap">
                    {task.output.slice(-500)}
                  </pre>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
