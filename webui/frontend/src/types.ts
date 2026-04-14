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

export interface StorySetup {
  genre: string[];
  premise: string;
  theme: string;
  story_rules: string[];
  target_audience: string;
  tone_style: string;
  narrative_tense: string;
  pov_style: string;
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
