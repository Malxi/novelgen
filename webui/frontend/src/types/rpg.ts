// RPG Types

export interface BaseStats {
  hp: number;
  mp: number;
  attack: number;
  defense: number;
  magic: number;
  resistance: number;
  speed: number;
  luck: number;
}

export interface GrowthStats {
  hp_growth: number;
  mp_growth: number;
  attack_growth: number;
  defense_growth: number;
  magic_growth: number;
  resistance_growth: number;
  speed_growth: number;
  luck_growth: number;
}

export type ElementType = 'none' | 'fire' | 'water' | 'wind' | 'earth' | 'light' | 'dark' | 'thunder' | 'ice' | 'poison';
export type Rarity = 'common' | 'uncommon' | 'rare' | 'epic' | 'legendary' | 'mythic';
export type CharacterType = 'player' | 'npc' | 'enemy' | 'boss' | 'companion';
export type CharacterState = 'normal' | 'battle' | 'dead' | 'poisoned' | 'stunned' | 'sleeping' | 'silenced';

export interface RPGCharacter {
  id: string;
  template_id?: string;
  name: string;
  description?: string;
  type: CharacterType;
  race?: string;
  class_id?: string;
  level: number;
  exp: number;
  exp_to_next: number;
  base_stats: BaseStats;
  current_stats: BaseStats;
  growth_stats: GrowthStats;
  element: ElementType;
  state: CharacterState;
  rarity: Rarity;
  skills?: string[];
  equipment?: {
    weapon?: string;
    armor?: string;
    helmet?: string;
    accessory1?: string;
    accessory2?: string;
  };
  inventory?: string[];
  relationships?: Record<string, number>;
}

export interface RPGSkill {
  id: string;
  name: string;
  description: string;
  type: 'active' | 'passive' | 'ultimate';
  element: ElementType;
  power: number;
  mp_cost: number;
  cooldown: number;
  target_type: 'single' | 'all' | 'self' | 'ally' | 'enemies';
  effects?: string[];
  unlock_level?: number;
}

export interface RPGItem {
  id: string;
  name: string;
  description: string;
  type: 'consumable' | 'equipment' | 'material' | 'quest' | 'key';
  rarity: Rarity;
  effect?: string;
  stats_bonus?: Partial<BaseStats>;
  usable?: boolean;
  sell_price?: number;
  buy_price?: number;
}

export interface RPGClass {
  id: string;
  name: string;
  description: string;
  base_stats: BaseStats;
  growth_stats: GrowthStats;
  skills: string[];
  element_affinity?: ElementType[];
  weapon_types?: string[];
  armor_types?: string[];
}

export interface RPGEvent {
  id: string;
  name: string;
  description: string;
  type: 'combat' | 'story' | 'choice' | 'random' | 'trap' | 'reward';
  location_id?: string;
  chapter_id?: string;
  conditions?: string[];
  effects?: string[];
  choices?: RPGChoice[];
}

export interface RPGChoice {
  id: string;
  text: string;
  condition?: string;
  effects: string[];
  next_event?: string;
}

export interface RPGQuest {
  id: string;
  name: string;
  description: string;
  type: 'main' | 'side' | 'daily' | 'hidden';
  status: 'not_started' | 'active' | 'completed' | 'failed';
  objectives: RPGQuestObjective[];
  rewards: RPGQuestReward;
  giver?: string;
  location?: string;
  prerequisites?: string[];
}

export interface RPGQuestObjective {
  id: string;
  description: string;
  type: 'kill' | 'collect' | 'talk' | 'reach' | 'use' | 'custom';
  target?: string;
  count: number;
  current: number;
  completed: boolean;
}

export interface RPGQuestReward {
  exp?: number;
  gold?: number;
  items?: { item_id: string; count: number }[];
  skills?: string[];
  reputation?: Record<string, number>;
}

export interface RPGData {
  characters: Record<string, RPGCharacter>;
  skills: Record<string, RPGSkill>;
  items: Record<string, RPGItem>;
  classes: Record<string, RPGClass>;
  events: Record<string, RPGEvent>;
  quests: Record<string, RPGQuest>;
}
