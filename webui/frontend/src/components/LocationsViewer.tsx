import { useEffect, useState } from 'react';
import {
  MapPin,
  Plus,
  RefreshCw,
  Sparkles,
  ChevronRight,
  Info,
} from 'lucide-react';
import { getLocations, createTask } from '../api';
import type { Location } from '../types';

interface LocationCardProps {
  location: Location;
  onClick: () => void;
}

function LocationCard({ location, onClick }: LocationCardProps) {
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-green-500 to-emerald-600 flex items-center justify-center flex-shrink-0">
          <MapPin className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold truncate">{location.name}</h3>
          <p className="text-sm text-[var(--text-muted)] line-clamp-2 mt-1">
            {location.description}
          </p>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)] flex-shrink-0" />
      </div>
    </div>
  );
}

interface LocationDetailProps {
  location: Location;
  onClose: () => void;
}

function LocationDetail({ location, onClose }: LocationDetailProps) {
  return (
    <div className="glass rounded-xl p-6 animate-fade-in">
      <div className="flex items-start justify-between mb-6">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-green-500 to-emerald-600 flex items-center justify-center">
            <MapPin className="w-8 h-8 text-white" />
          </div>
          <div>
            <h2 className="text-2xl font-bold">{location.name}</h2>
            <p className="text-[var(--text-muted)]">地点详情</p>
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
        {location.description && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2">描述</h3>
            <p className="text-[var(--text)]">{location.description}</p>
          </div>
        )}

        {location.significance && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2 flex items-center gap-2">
              <Info className="w-4 h-4" />
              重要性
            </h3>
            <p className="text-[var(--text)]">{location.significance}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export function LocationsViewer({ projectPath }: { projectPath: string }) {
  const [locations, setLocations] = useState<Location[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [selectedLocation, setSelectedLocation] = useState<Location | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadLocations();
  }, [projectPath]);

  async function loadLocations() {
    try {
      setLoading(true);
      const data = await getLocations(projectPath);
      setLocations(data.locations || []);
      setError(null);
    } catch (err) {
      setError('地点数据不存在');
      setLocations([]);
    } finally {
      setLoading(false);
    }
  }

  async function generateLocations() {
    try {
      setGenerating(true);
      await createTask({
        type: 'craft',
        command: 'craft',
        args: {
          project_dir: projectPath,
          subcommand: 'gen',
          type: 'locations',
        },
      });
      setTimeout(() => loadLocations(), 5000);
    } catch (err) {
      console.error('Failed to generate locations:', err);
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

  if (error && locations.length === 0) {
    return (
      <div className="text-center py-16">
        <MapPin className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
        <h2 className="text-xl font-bold mb-2">暂无地点</h2>
        <p className="text-[var(--text-muted)] mb-6">使用 AI 从大纲中提取并生成地点</p>
        <button
          onClick={generateLocations}
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
              生成地点
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
          <h1 className="text-2xl font-bold mb-1">地点列表</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {locations.length} 个地点</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadLocations}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          <button
            onClick={generateLocations}
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
                生成地点
              </>
            )}
          </button>
        </div>
      </div>

      {/* Locations Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {locations.map((location) => (
              <LocationCard
                key={location.id}
                location={location}
                onClick={() => setSelectedLocation(location)}
              />
            ))}
          </div>
        </div>

        <div>
          {selectedLocation ? (
            <LocationDetail
              location={selectedLocation}
              onClose={() => setSelectedLocation(null)}
            />
          ) : (
            <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)]">
              <MapPin className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>选择一个地点查看详情</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
