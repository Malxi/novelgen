import { useEffect, useState } from 'react';
import {
  Package,
  Plus,
  RefreshCw,
  Sparkles,
  ChevronRight,
  Info,
} from 'lucide-react';
import { getItems, createTask } from '../api';
import type { Item } from '../types';

interface ItemCardProps {
  item: Item;
  onClick: () => void;
}

function ItemCard({ item, onClick }: ItemCardProps) {
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center flex-shrink-0">
          <Package className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold truncate">{item.name}</h3>
          <p className="text-sm text-[var(--text-muted)] line-clamp-2 mt-1">
            {item.description}
          </p>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)] flex-shrink-0" />
      </div>
    </div>
  );
}

interface ItemDetailProps {
  item: Item;
  onClose: () => void;
}

function ItemDetail({ item, onClose }: ItemDetailProps) {
  return (
    <div className="glass rounded-xl p-6 animate-fade-in">
      <div className="flex items-start justify-between mb-6">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center">
            <Package className="w-8 h-8 text-white" />
          </div>
          <div>
            <h2 className="text-2xl font-bold">{item.name}</h2>
            <p className="text-[var(--text-muted)]">物品详情</p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="p-2 hover:bg-[var(--surface-light)] rounded-lg"
        >
          ✕
        </button>
      </div>

      <div className="space-y-6">
        {item.description && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2">描述</h3>
            <p className="text-[var(--text)]">{item.description}</p>
          </div>
        )}

        {item.significance && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2 flex items-center gap-2">
              <Info className="w-4 h-4" />
              重要性
            </h3>
            <p className="text-[var(--text)]">{item.significance}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export function ItemsViewer({ projectPath }: { projectPath: string }) {
  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [selectedItem, setSelectedItem] = useState<Item | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadItems();
  }, [projectPath]);

  async function loadItems() {
    try {
      setLoading(true);
      const data = await getItems(projectPath);
      setItems(data.items || []);
      setError(null);
    } catch (err) {
      setError('物品数据不存在');
      setItems([]);
    } finally {
      setLoading(false);
    }
  }

  async function generateItems() {
    try {
      setGenerating(true);
      await createTask({
        type: 'craft',
        command: 'craft',
        args: {
          project_dir: projectPath,
          subcommand: 'gen',
          type: 'items',
        },
      });
      setTimeout(() => loadItems(), 5000);
    } catch (err) {
      console.error('Failed to generate items:', err);
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

  if (error && items.length === 0) {
    return (
      <div className="text-center py-16">
        <Package className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
        <h2 className="text-xl font-bold mb-2">暂无物品</h2>
        <p className="text-[var(--text-muted)] mb-6">使用 AI 从大纲中提取并生成物品</p>
        <button
          onClick={generateItems}
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
              生成物品
            </>
          )}
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold mb-1">物品列表</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {items.length} 个物品</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadItems}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          <button
            onClick={generateItems}
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
                <Plus className="w-4 h-4" />
                生成物品
              </>
            )}
          </button>
        </div>
      </div>

      {/* Items Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {items.map((item) => (
              <ItemCard
                key={item.id}
                item={item}
                onClick={() => setSelectedItem(item)}
              />
            ))}
          </div>
        </div>

        <div>
          {selectedItem ? (
            <ItemDetail
              item={selectedItem}
              onClose={() => setSelectedItem(null)}
            />
          ) : (
            <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)]">
              <Package className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>选择一个物品查看详情</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
