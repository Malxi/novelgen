import { useEffect, useState } from 'react';
import {
  Users,
  Plus,
  RefreshCw,
  Sparkles,
  User,
  Heart,
  Target,
  Link,
  ChevronRight,
} from 'lucide-react';
import { getCharacters, createTask } from '../api';
import type { Character } from '../types';

interface CharacterCardProps {
  character: Character;
  onClick: () => void;
}

function CharacterCard({ character, onClick }: CharacterCardProps) {
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-full bg-gradient-to-br from-[var(--primary)] to-[var(--secondary)] flex items-center justify-center flex-shrink-0">
          <User className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold truncate">{character.name}</h3>
          <p className="text-sm text-[var(--text-muted)] line-clamp-2 mt-1">
            {character.description}
          </p>
          {character.personality && (
            <div className="flex items-center gap-1 mt-2 text-xs text-[var(--text-muted)]">
              <Heart className="w-3 h-3" />
              <span className="truncate">{character.personality}</span>
            </div>
          )}
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)] flex-shrink-0" />
      </div>
    </div>
  );
}

interface CharacterDetailProps {
  character: Character;
  onClose: () => void;
}

function CharacterDetail({ character, onClose }: CharacterDetailProps) {
  return (
    <div className="glass rounded-xl p-6 animate-fade-in">
      <div className="flex items-start justify-between mb-6">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 rounded-full bg-gradient-to-br from-[var(--primary)] to-[var(--secondary)] flex items-center justify-center">
            <User className="w-8 h-8 text-white" />
          </div>
          <div>
            <h2 className="text-2xl font-bold">{character.name}</h2>
            <p className="text-[var(--text-muted)]">角色详情</p>
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
        {character.description && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2">描述</h3>
            <p className="text-[var(--text)]">{character.description}</p>
          </div>
        )}

        {character.personality && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2 flex items-center gap-2">
              <Heart className="w-4 h-4" />
              性格
            </h3>
            <p className="text-[var(--text)]">{character.personality}</p>
          </div>
        )}

        {character.background && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2">背景</h3>
            <p className="text-[var(--text)]">{character.background}</p>
          </div>
        )}

        {character.goals && character.goals.length > 0 && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2 flex items-center gap-2">
              <Target className="w-4 h-4" />
              目标
            </h3>
            <ul className="space-y-1">
              {character.goals.map((goal, idx) => (
                <li key={idx} className="flex items-start gap-2">
                  <span className="text-[var(--primary)]">•</span>
                  <span>{goal}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        {character.relationships && Object.keys(character.relationships).length > 0 && (
          <div>
            <h3 className="text-sm font-medium text-[var(--text-muted)] mb-2 flex items-center gap-2">
              <Link className="w-4 h-4" />
              关系
            </h3>
            <div className="space-y-2">
              {Object.entries(character.relationships).map(([name, relation], idx) => (
                <div key={idx} className="flex items-center gap-2 p-2 bg-[var(--surface-light)] rounded-lg">
                  <span className="font-medium">{name}</span>
                  <span className="text-[var(--text-muted)]">→</span>
                  <span>{relation}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export function CharactersViewer({ projectPath }: { projectPath: string }) {
  const [characters, setCharacters] = useState<Character[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [selectedCharacter, setSelectedCharacter] = useState<Character | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadCharacters();
  }, [projectPath]);

  async function loadCharacters() {
    try {
      setLoading(true);
      const data = await getCharacters(projectPath);
      setCharacters(data.characters || []);
      setError(null);
    } catch (err) {
      setError('角色数据不存在');
      setCharacters([]);
    } finally {
      setLoading(false);
    }
  }

  async function generateCharacters() {
    try {
      setGenerating(true);
      await createTask({
        type: 'craft',
        command: 'craft',
        args: {
          project_dir: projectPath,
          subcommand: 'gen',
          type: 'characters',
        },
      });
      setTimeout(() => loadCharacters(), 5000);
    } catch (err) {
      console.error('Failed to generate characters:', err);
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

  if (error && characters.length === 0) {
    return (
      <div className="text-center py-16">
        <Users className="w-16 h-16 mx-auto text-[var(--text-muted)] mb-4" />
        <h2 className="text-xl font-bold mb-2">暂无角色</h2>
        <p className="text-[var(--text-muted)] mb-6">使用 AI 从大纲中提取并生成角色</p>
        <button
          onClick={generateCharacters}
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
              生成角色
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
          <h1 className="text-2xl font-bold mb-1">角色列表</h1>
          <p className="text-[var(--text-muted)] text-sm">共 {characters.length} 个角色</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={loadCharacters}
            className="btn btn-secondary"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
          <button
            onClick={generateCharacters}
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
                生成角色
              </>
            )}
          </button>
        </div>
      </div>

      {/* Characters Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {characters.map((character) => (
              <CharacterCard
                key={character.id}
                character={character}
                onClick={() => setSelectedCharacter(character)}
              />
            ))}
          </div>
        </div>

        <div>
          {selectedCharacter ? (
            <CharacterDetail
              character={selectedCharacter}
              onClose={() => setSelectedCharacter(null)}
            />
          ) : (
            <div className="glass rounded-xl p-6 text-center text-[var(--text-muted)]">
              <User className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>选择一个角色查看详情</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
