import { useEffect, useState } from 'react';
import {
  Users,
  Zap,
  Package,
  Trophy,
  Map,
  Scroll,
  RefreshCw,
  ChevronRight,
  Shield,
  Flame,
  Droplets,
  Wind,
  Mountain,
  Sun,
  Moon,
  Bolt,
  Snowflake,
  Skull,
  Layers,
  CheckCircle2,
} from 'lucide-react';
import { createTask, getRPGCharacters, getRPGSkills, getRPGItems, getRPGClasses, getRPGEvents, getRPGQuests, getTemplates } from '../api';
import type { SystemTemplate, TemplateLibrary } from '../types';
import type { RPGCharacter, RPGSkill, RPGItem, RPGClass, RPGEvent, RPGQuest, ElementType, Rarity } from '../types/rpg';

type RPGTab = 'characters' | 'skills' | 'items' | 'classes' | 'events' | 'quests' | 'templates';

interface RPGManagerProps {
  projectPath: string;
}

const elementIcons: Record<ElementType, React.ElementType> = {
  none: Shield,
  fire: Flame,
  water: Droplets,
  wind: Wind,
  earth: Mountain,
  light: Sun,
  dark: Moon,
  thunder: Bolt,
  ice: Snowflake,
  poison: Skull,
};

const elementColors: Record<ElementType, string> = {
  none: 'text-gray-400',
  fire: 'text-red-500',
  water: 'text-blue-500',
  wind: 'text-cyan-500',
  earth: 'text-amber-600',
  light: 'text-yellow-400',
  dark: 'text-purple-500',
  thunder: 'text-yellow-500',
  ice: 'text-cyan-300',
  poison: 'text-green-500',
};

const rarityColors: Record<Rarity, string> = {
  common: 'text-gray-400',
  uncommon: 'text-green-400',
  rare: 'text-blue-400',
  epic: 'text-purple-400',
  legendary: 'text-orange-400',
  mythic: 'text-red-400',
};

const rarityBgColors: Record<Rarity, string> = {
  common: 'bg-gray-500/20',
  uncommon: 'bg-green-500/20',
  rare: 'bg-blue-500/20',
  epic: 'bg-purple-500/20',
  legendary: 'bg-orange-500/20',
  mythic: 'bg-red-500/20',
};

function StatBar({ label, value, max = 100 }: { label: string; value: number; max?: number }) {
  const percentage = Math.min((value / max) * 100, 100);
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-12 text-[var(--text-muted)]">{label}</span>
      <div className="flex-1 h-2 bg-[var(--surface-light)] rounded-full overflow-hidden">
        <div
          className="h-full bg-gradient-to-r from-[var(--primary)] to-[var(--secondary)] rounded-full"
          style={{ width: `${percentage}%` }}
        />
      </div>
      <span className="w-8 text-right">{value}</span>
    </div>
  );
}

function CharacterCard({ character, onClick }: { character: RPGCharacter; onClick: () => void }) {
  const ElementIcon = elementIcons[character.element] || Shield;
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className={`w-12 h-12 rounded-xl ${rarityBgColors[character.rarity]} flex items-center justify-center`}>
          <Users className="w-6 h-6" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold truncate">{character.name}</h3>
            <ElementIcon className={`w-4 h-4 ${elementColors[character.element]}`} />
          </div>
          <p className="text-xs text-[var(--text-muted)] capitalize">{character.type} · Lv.{character.level}</p>
          <div className="mt-2 space-y-1">
            <StatBar label="HP" value={character.current_stats?.hp || character.base_stats?.hp || 0} />
            <StatBar label="MP" value={character.current_stats?.mp || character.base_stats?.mp || 0} />
          </div>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function SkillCard({ skill, onClick }: { skill: RPGSkill; onClick: () => void }) {
  const ElementIcon = elementIcons[skill.element] || Shield;
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-[var(--primary)]/20 flex items-center justify-center">
          <Zap className="w-6 h-6 text-[var(--primary)]" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold truncate">{skill.name}</h3>
            <ElementIcon className={`w-4 h-4 ${elementColors[skill.element]}`} />
          </div>
          <p className="text-xs text-[var(--text-muted)]">{skill.type} · Power: {skill.power}</p>
          <div className="flex gap-2 mt-2 text-xs">
            <span className="px-2 py-1 bg-[var(--surface-light)] rounded">MP: {skill.mp_cost}</span>
            <span className="px-2 py-1 bg-[var(--surface-light)] rounded">CD: {skill.cooldown}s</span>
          </div>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function ItemCard({ item, onClick }: { item: RPGItem; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className={`w-12 h-12 rounded-xl ${rarityBgColors[item.rarity]} flex items-center justify-center`}>
          <Package className="w-6 h-6" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold truncate">{item.name}</h3>
            <span className={`text-xs ${rarityColors[item.rarity]}`}>{item.rarity}</span>
          </div>
          <p className="text-xs text-[var(--text-muted)] capitalize">{item.type}</p>
          {item.effect && (
            <p className="text-xs text-[var(--text)] mt-1 truncate">{item.effect}</p>
          )}
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function ClassCard({ classData, onClick }: { classData: RPGClass; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center">
          <Trophy className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold truncate">{classData.name}</h3>
          <p className="text-xs text-[var(--text-muted)] line-clamp-2">{classData.description}</p>
          <div className="flex gap-2 mt-2">
            {classData.skills?.slice(0, 3).map((skill, idx) => (
              <span key={idx} className="text-xs px-2 py-1 bg-[var(--surface-light)] rounded">
                {skill}
              </span>
            ))}
          </div>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function EventCard({ event, onClick }: { event: RPGEvent; onClick: () => void }) {
  const typeColors: Record<string, string> = {
    combat: 'bg-red-500/20 text-red-400',
    story: 'bg-blue-500/20 text-blue-400',
    choice: 'bg-purple-500/20 text-purple-400',
    random: 'bg-yellow-500/20 text-yellow-400',
    trap: 'bg-orange-500/20 text-orange-400',
    reward: 'bg-green-500/20 text-green-400',
  };

  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className={`w-12 h-12 rounded-xl ${typeColors[event.type] || 'bg-[var(--surface-light)]'} flex items-center justify-center`}>
          <Map className="w-6 h-6" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold truncate">{event.name}</h3>
            <span className={`text-xs px-2 py-0.5 rounded ${typeColors[event.type] || ''}`}>
              {event.type}
            </span>
          </div>
          <p className="text-xs text-[var(--text-muted)] line-clamp-2 mt-1">{event.description}</p>
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function QuestCard({ quest, onClick }: { quest: RPGQuest; onClick: () => void }) {
  const statusColors: Record<string, string> = {
    not_started: 'text-gray-400',
    active: 'text-blue-400',
    completed: 'text-green-400',
    failed: 'text-red-400',
  };

  const typeColors: Record<string, string> = {
    main: 'bg-purple-500/20 text-purple-400',
    side: 'bg-blue-500/20 text-blue-400',
    daily: 'bg-green-500/20 text-green-400',
    hidden: 'bg-orange-500/20 text-orange-400',
  };

  return (
    <div
      onClick={onClick}
      className="glass rounded-xl p-4 card-hover cursor-pointer"
    >
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center">
          <Scroll className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-semibold truncate">{quest.name}</h3>
            <span className={`text-xs px-2 py-0.5 rounded ${typeColors[quest.type] || ''}`}>
              {quest.type}
            </span>
            <span className={`text-xs ${statusColors[quest.status]}`}>
              {quest.status}
            </span>
          </div>
          <p className="text-xs text-[var(--text-muted)] line-clamp-2 mt-1">{quest.description}</p>
          {quest.objectives && (
            <div className="mt-2 text-xs">
              <span className="text-[var(--text-muted)]">进度: </span>
              <span className="text-[var(--primary)]">
                {quest.objectives.filter(o => o.completed).length}/{quest.objectives.length}
              </span>
            </div>
          )}
        </div>
        <ChevronRight className="w-5 h-5 text-[var(--text-muted)]" />
      </div>
    </div>
  );
}

function TemplateCard({
  template,
  active,
  busy,
  onApply,
}: {
  template: SystemTemplate;
  active: boolean;
  busy: boolean;
  onApply: () => void;
}) {
  const stageCount = template.progression?.stages?.length || 0;
  const rarityCount = template.item_rarity?.rarities?.length || 0;
  const resourceCategoryCount = template.resource_tier?.categories?.length || 0;
  const resourceTierCount = (template.resource_tier?.categories || []).reduce((total, category) => total + (category.tiers?.length || 0), 0);
  const meta = template.kind === 'progression'
    ? `${template.progression?.category || 'progression'} · ${stageCount} stages`
    : template.kind === 'resource_tier'
      ? `${resourceCategoryCount} categories · ${resourceTierCount} tiers`
      : `${template.item_rarity?.power_scale || 'item scale'} · ${rarityCount} tiers`;

  return (
    <div className="glass rounded-xl p-4">
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-[var(--primary)]/15 flex items-center justify-center">
          <Layers className="w-6 h-6 text-[var(--primary)]" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-semibold">{template.name}</h3>
            <span className="text-xs px-2 py-0.5 rounded bg-[var(--surface-light)] text-[var(--text-muted)]">{template.kind}</span>
            {active && (
              <span className="text-xs px-2 py-0.5 rounded bg-green-500/15 text-green-400 flex items-center gap-1">
                <CheckCircle2 className="w-3 h-3" />
                active
              </span>
            )}
          </div>
          <p className="text-xs text-[var(--text-muted)] mt-1">{meta}</p>
          <p className="text-sm text-[var(--text)] mt-2 line-clamp-2">{template.description}</p>
          {template.tags && template.tags.length > 0 && (
            <div className="flex gap-2 mt-3 flex-wrap">
              {template.tags.slice(0, 5).map((tag) => (
                <span key={tag} className="text-xs px-2 py-1 bg-[var(--surface-light)] rounded">{tag}</span>
              ))}
            </div>
          )}
        </div>
        <button
          onClick={onApply}
          disabled={busy}
          className="btn btn-secondary text-sm"
        >
          {active ? '重应用' : '应用'}
        </button>
      </div>
    </div>
  );
}

export function RPGManager({ projectPath }: RPGManagerProps) {
  const [activeTab, setActiveTab] = useState<RPGTab>('characters');
  const [characters, setCharacters] = useState<RPGCharacter[]>([]);
  const [skills, setSkills] = useState<RPGSkill[]>([]);
  const [items, setItems] = useState<RPGItem[]>([]);
  const [classes, setClasses] = useState<RPGClass[]>([]);
  const [events, setEvents] = useState<RPGEvent[]>([]);
  const [quests, setQuests] = useState<RPGQuest[]>([]);
  const [templates, setTemplates] = useState<TemplateLibrary | null>(null);
  const [loading, setLoading] = useState(true);
  const [templateBusy, setTemplateBusy] = useState(false);

  const tabs = [
    { id: 'characters' as RPGTab, label: '角色', icon: Users },
    { id: 'skills' as RPGTab, label: '技能', icon: Zap },
    { id: 'items' as RPGTab, label: '物品', icon: Package },
    { id: 'classes' as RPGTab, label: '职业', icon: Trophy },
    { id: 'events' as RPGTab, label: '事件', icon: Map },
    { id: 'quests' as RPGTab, label: '任务', icon: Scroll },
    { id: 'templates' as RPGTab, label: '模板', icon: Layers },
  ];

  useEffect(() => {
    loadRPGData();
  }, [projectPath]);

  async function loadRPGData() {
    try {
      setLoading(true);
      const [charsData, skillsData, itemsData, classesData, eventsData, questsData, templatesData] = await Promise.all([
        getRPGCharacters(projectPath).catch(() => ({ characters: [] })),
        getRPGSkills(projectPath).catch(() => ({ skills: [] })),
        getRPGItems(projectPath).catch(() => ({ items: [] })),
        getRPGClasses(projectPath).catch(() => ({ classes: [] })),
        getRPGEvents(projectPath).catch(() => ({ events: [] })),
        getRPGQuests(projectPath).catch(() => ({ quests: [] })),
        getTemplates(projectPath).catch(() => null),
      ]);

      setCharacters((charsData.characters || []) as RPGCharacter[]);
      setSkills((skillsData.skills || []) as RPGSkill[]);
      setItems((itemsData.items || []) as RPGItem[]);
      setClasses((classesData.classes || []) as RPGClass[]);
      setEvents((eventsData.events || []) as RPGEvent[]);
      setQuests((questsData.quests || []) as RPGQuest[]);
      setTemplates(templatesData);
    } catch (err) {
      console.error('Failed to load RPG data:', err);
    } finally {
      setLoading(false);
    }
  }

  async function runTemplateCommand(subcommand: 'init' | 'apply', templateId?: string) {
    try {
      setTemplateBusy(true);
      await createTask({
        type: `template-${subcommand}`,
        command: 'template',
        args: {
          project_dir: projectPath,
          subcommand,
          ...(templateId ? { _positional: templateId } : {}),
        },
      });
      setTimeout(loadRPGData, 1200);
    } catch (err) {
      console.error('Failed to run template command:', err);
    } finally {
      setTemplateBusy(false);
    }
  }

  function renderContent() {
    if (loading) {
      return (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--primary)]"></div>
        </div>
      );
    }

    if (activeTab === 'templates') {
      const applied = new Set((templates?.applied_templates || []).map((t) => t.id));
      if (!templates) {
        return (
          <div className="glass rounded-xl p-6 flex items-center justify-between gap-4">
            <div>
              <h3 className="font-semibold mb-1">模板库尚未初始化</h3>
              <p className="text-sm text-[var(--text-muted)]">创建默认的升级体系和物品等级模板。</p>
            </div>
            <button
              onClick={() => runTemplateCommand('init')}
              disabled={templateBusy}
              className="btn btn-primary"
            >
              初始化模板
            </button>
          </div>
        );
      }
      return (
        <div className="space-y-4">
          {templates.templates.map((template) => (
            <TemplateCard
              key={template.id}
              template={template}
              active={applied.has(template.id)}
              busy={templateBusy}
              onApply={() => runTemplateCommand('apply', template.id)}
            />
          ))}
        </div>
      );
    }

    const currentData = {
      characters,
      skills,
      items,
      classes,
      events,
      quests,
      templates: templates?.templates || [],
    }[activeTab];

    if (currentData.length === 0) {
      return (
        <div className="text-center py-16 text-[var(--text-muted)]">
          <div className="text-6xl mb-4">🎮</div>
          <p>暂无{tabs.find(t => t.id === activeTab)?.label}数据</p>
        </div>
      );
    }

    switch (activeTab) {
      case 'characters':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {characters.map((character, idx) => (
              <CharacterCard key={idx} character={character} onClick={() => {}} />
            ))}
          </div>
        );
      case 'skills':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {skills.map((skill, idx) => (
              <SkillCard key={idx} skill={skill} onClick={() => {}} />
            ))}
          </div>
        );
      case 'items':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {items.map((item, idx) => (
              <ItemCard key={idx} item={item} onClick={() => {}} />
            ))}
          </div>
        );
      case 'classes':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {classes.map((classData, idx) => (
              <ClassCard key={idx} classData={classData} onClick={() => {}} />
            ))}
          </div>
        );
      case 'events':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {events.map((event, idx) => (
              <EventCard key={idx} event={event} onClick={() => {}} />
            ))}
          </div>
        );
      case 'quests':
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {quests.map((quest, idx) => (
              <QuestCard key={idx} quest={quest} onClick={() => {}} />
            ))}
          </div>
        );
      default:
        return null;
    }
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold mb-1">数值化系统</h1>
          <p className="text-[var(--text-muted)] text-sm">
            {characters.length} 角色 · {skills.length} 技能 · {items.length} 物品 · {classes.length} 职业 · {events.length} 事件 · {quests.length} 任务 · {templates?.templates?.length || 0} 模板
          </p>
        </div>
        <button
          onClick={loadRPGData}
          className="btn btn-secondary"
        >
          <RefreshCw className="w-4 h-4" />
          刷新
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-2 overflow-x-auto pb-2">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => {
                setActiveTab(tab.id);
              }}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg whitespace-nowrap transition-colors ${
                activeTab === tab.id
                  ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
                  : 'bg-[var(--surface)] hover:bg-[var(--surface-light)]'
              }`}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Content */}
      {renderContent()}
    </div>
  );
}
