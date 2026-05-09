export interface Project {
  name: string;
  path: string;
  language: string;
  structure: {
    target_parts: number;
    target_volumes: number;
    target_chapters: number;
  };
  chapter_config: {
    target_words_per_chapter: number;
    min_words_per_chapter: number;
    max_words_per_chapter: number;
  };
  llm: {
    provider: string;
    model: string;
  };
  exists: boolean;
}

export interface Task {
  id: string;
  type: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress: number;
  message: string;
  output: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface FileVersion {
  filename: string;
  created_at: string;
  size: number;
}

export interface AICallSummary {
  id: string;
  agent: string;
  command?: string;
  model?: string;
  started_at: string;
  has_input: boolean;
  has_output: boolean;
  input_chars: number;
  output_chars: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  legacy: boolean;
}

export interface AICallDetail extends AICallSummary {
  skills?: string[];
  system_prompt: string;
  user_prompt: string;
  response: string;
  prompt_path?: string;
  response_path?: string;
}

export interface Outline {
  parts: Part[];
}

export interface Part {
  id: string;
  title: string;
  summary: string;
  volumes: Volume[];
}

export interface Volume {
  id: string;
  title: string;
  summary: string;
  chapters: Chapter[];
}

export interface ChapterEvent {
  type: string;
  characters: string[];
  subject: string;
  change: string;
  details?: string;
}

export interface Chapter {
  id: string;
  title: string;
  summary: string;
  characters: string[];
  location: string;
  events: ChapterEvent[];
  beats: string[];
  opening_beat: string;
  closing_beat: string;
  conflict: string;
  pacing: string;
}

export interface StorylineVolumeIntent {
  volume?: number;
  intent: string;
  pressure?: string;
  must_include?: string[];
  must_avoid?: string[];
}

export interface StorylineArcPhase {
  phase: string;
  purpose: string;
  volume?: number;
}

export interface StorylineBeatPrerequisite {
  beat: string;
  requires: string[];
}

export interface StorylineAgencyContract {
  character: string;
  must_choose: string;
  private_goal?: string;
  can_conflict_with_protagonist?: boolean;
}

export interface StorylineClueContract {
  truth: string;
  source: string;
  reliability?: string;
  cost?: string;
  must_not?: string;
}

export interface Storyline {
  name: string;
  description: string;
  type: string;
  importance: number;
  scope?: string;
  payoff_style?: string;
  setup_role?: string;
  desire?: string;
  opposition?: string;
  stakes?: string;
  turn?: string;
  payoff?: string;
  open_question?: string;
  pressure_points?: string[];
  key_characters?: string[];
  must_include?: string[];
  must_avoid?: string[];
  volume_intents?: StorylineVolumeIntent[];
  arc_phases?: StorylineArcPhase[];
  beat_prerequisites?: StorylineBeatPrerequisite[];
  required_costs?: string[];
  agency_contracts?: StorylineAgencyContract[];
  antagonist_moves?: string[];
  clue_contracts?: StorylineClueContract[];
  repeatable_pressure?: string;
  payoff_cadence?: string;
  mutation?: string;
  failure_mode?: string;
  appeal_engine?: AppealEngine;
}

export interface AppealEngine {
  appeal?: string;
  surface_limit?: string;
  exploit?: string;
  signature_win?: string;
  upgrade_path?: string;
  opponent_misread?: string;
  reward_type?: string;
}

export interface LongFormPlan {
  target_chapters?: number;
  target_volumes?: number;
  main_loop?: string;
  escalation_ladder?: string[];
  reader_promises?: string[];
  payoff_cadence?: string;
  volume_pattern?: string[];
  midpoint_mutation?: string;
  endgame_promise?: string;
}

export interface CoreCastSeed {
  id?: string;
  name: string;
  role: string;
  importance: number;
  story_function: string;
  relationship_to_lead?: string;
  relationship_arc?: string;
  entry_phase: string;
  payoff?: string;
  storyline_refs?: string[];
}

export interface PremiseProgression {
  level: number;
  name: string;
  description: string;
  requirements?: string;
}

export interface Premise {
  name: string;
  description: string;
  category: string;
  progression: PremiseProgression[];
  appeal_engine?: AppealEngine;
}

export interface WorldTimelineEntry {
  year: string;
  event: string;
  impact?: string;
  related_mystery?: string;
}

export interface WorldResource {
  name: string;
  category: string;
  scarcity: string;
  description: string;
}

export interface TemplateLibrary {
  version: string;
  templates: SystemTemplate[];
  applied_templates?: AppliedTemplateRef[];
}

export interface SystemTemplate {
  id: string;
  name: string;
  kind: 'progression' | 'item_rarity' | 'resource_tier' | string;
  description: string;
  tags?: string[];
  progression?: ProgressionTemplate;
  item_rarity?: ItemRarityTemplate;
  resource_tier?: ResourceTierTemplate;
  notes?: string[];
}

export interface ProgressionTemplate {
  category: string;
  stages: PremiseProgression[];
  rules?: string[];
  resource_hints?: WorldResource[];
}

export interface ItemRarityTemplate {
  power_scale?: string;
  rarities: ItemRarityStage[];
  rules?: string[];
}

export interface ItemRarityStage {
  id: string;
  name: string;
  rank: number;
  description: string;
  power_min?: number;
  power_max?: number;
  scarcity?: string;
  typical_items?: string[];
  rules?: string[];
}

export interface ResourceTierTemplate {
  categories: ResourceTierCategory[];
  rules?: string[];
}

export interface ResourceTierCategory {
  id: string;
  name: string;
  description?: string;
  tiers: ResourceTierStage[];
}

export interface ResourceTierStage {
  id: string;
  name: string;
  rank: number;
  scarcity?: string;
  description: string;
}

export interface AppliedTemplateRef {
  id: string;
  name: string;
  kind: string;
  applied_at: string;
}

export interface StorySetup {
  project_name: string;
  genres: string[];
  premise: string;
  theme: string;
  rules: string[];
  target_audience: string;
  tone: string;
  tense: string;
  pov_style: string;
  long_form_plan?: LongFormPlan;
  core_cast?: CoreCastSeed[];
  storylines?: Storyline[];
  premises?: Premise[];
  world_timeline?: WorldTimelineEntry[];
  world_resources?: WorldResource[];
}

export interface Character {
  id: string;
  name: string;
  description: string;
  personality: string;
  background: string;
  goals: string[];
  relationships: Record<string, string>;
}

export interface Location {
  id: string;
  name: string;
  description: string;
  significance: string;
}

export interface Item {
  id: string;
  name: string;
  description: string;
  significance: string;
}

export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}
