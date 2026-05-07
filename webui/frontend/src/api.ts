import type { APIResponse, Project, Task, Outline, StorySetup, Character, Location, Item, AICallSummary, AICallDetail, TemplateLibrary } from './types';

const API_BASE = '/api';

async function fetchAPI<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  const data: APIResponse<T> = await response.json();

  if (!data.success) {
    throw new Error(data.error || 'Unknown error');
  }

  return data.data as T;
}

// Projects
export const listProjects = () => fetchAPI<Project[]>('/projects');
export const createProject = (project: {
  name: string;
  genre?: string;
  chapters?: number;
  provider?: string;
  model?: string;
  language?: string;
}) => fetchAPI<{ output: string }>('/projects', {
  method: 'POST',
  body: JSON.stringify(project),
});
export const getProject = (path: string) => fetchAPI<Project>(`/projects/${encodeURIComponent(path)}`);
export const getCurrentProject = () => fetchAPI<Project>('/projects/current');

// Tasks
export const listTasks = () => fetchAPI<Task[]>('/tasks');
export const getTask = (id: string) => fetchAPI<Task>(`/tasks/${id}`);
export const createTask = (task: {
  type: string;
  command: string;
  args?: Record<string, unknown>;
}) => fetchAPI<Task>('/tasks', {
  method: 'POST',
  body: JSON.stringify(task),
});
export const deleteTask = (id: string) => fetchAPI<void>(`/tasks/${id}`, { method: 'DELETE' });

// AI calls
export const listAICalls = (project?: string) =>
  fetchAPI<AICallSummary[]>(`/ai-calls${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getAICall = (id: string, project?: string) =>
  fetchAPI<AICallDetail>(`/ai-calls/${encodeURIComponent(id)}${project ? `?project=${encodeURIComponent(project)}` : ''}`);

// Content
export const getOutline = (project?: string) =>
  fetchAPI<Outline>(`/content/outline${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const listOutlineVersions = (project?: string) =>
  fetchAPI<Array<{ filename: string; created_at: string; size: number }>>(
    `/content/outline/versions${project ? `?project=${encodeURIComponent(project)}` : ''}`
  );

export const restoreOutlineVersion = (filename: string, project?: string) =>
  fetchAPI<{ message: string }>(`/content/outline/restore${project ? `?project=${encodeURIComponent(project)}` : ''}`, {
    method: 'POST',
    body: JSON.stringify({ filename }),
  });

export const getStorySetup = (project?: string) =>
  fetchAPI<StorySetup>(`/content/setup${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getCharacters = (project?: string) =>
  fetchAPI<{ characters: Character[] }>(`/content/characters${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getLocations = (project?: string) =>
  fetchAPI<{ locations: Location[] }>(`/content/locations${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getItems = (project?: string) =>
  fetchAPI<{ items: Item[] }>(`/content/items${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getChapters = (project?: string) =>
  fetchAPI<{ id: string; name: string }[]>(`/content/chapters${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getChapter = (id: string, project?: string) =>
  fetchAPI<{ id: string; content: string }>(`/content/chapters/${id}${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRecaps = (project?: string) =>
  fetchAPI<{ id: string; name: string }[]>(`/content/recaps${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getReviews = (project?: string) =>
  fetchAPI<{ id: string; name: string }[]>(`/content/reviews${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getTemplates = (project?: string) =>
  fetchAPI<TemplateLibrary>(`/files/story/templates/templates.json${project ? `?project=${encodeURIComponent(project)}` : ''}`);

// RPG Data
export const getRPGData = (project?: string) =>
  fetchAPI<unknown>(`/rpg/data${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGCharacters = (project?: string) =>
  fetchAPI<{ characters: unknown[] }>(`/rpg/characters${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGSkills = (project?: string) =>
  fetchAPI<{ skills: unknown[] }>(`/rpg/skills${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGItems = (project?: string) =>
  fetchAPI<{ items: unknown[] }>(`/rpg/items${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGClasses = (project?: string) =>
  fetchAPI<{ classes: unknown[] }>(`/rpg/classes${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGEvents = (project?: string) =>
  fetchAPI<{ events: unknown[] }>(`/rpg/events${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const getRPGQuests = (project?: string) =>
  fetchAPI<{ quests: unknown[] }>(`/rpg/quests${project ? `?project=${encodeURIComponent(project)}` : ''}`);

// Simulation Reports
export const listSimulationReports = (project?: string) =>
  fetchAPI<Array<{ filename: string; chapter_id: string; chapter_name: string; success: boolean }>>(
    `/simulations${project ? `?project=${encodeURIComponent(project)}` : ''}`
  );

export const getSimulationReport = (id: string, project?: string) =>
  fetchAPI<unknown>(`/simulations/${id}${project ? `?project=${encodeURIComponent(project)}` : ''}`);

// Files
export const getFile = (path: string, project?: string) =>
  fetchAPI<unknown>(`/files/${path}${project ? `?project=${encodeURIComponent(project)}` : ''}`);

export const saveFile = (path: string, content: string, project?: string) =>
  fetchAPI<void>(`/files/${path}${project ? `?project=${encodeURIComponent(project)}` : ''}`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  });

export const saveJSONFile = (path: string, value: unknown, project?: string) =>
  saveFile(path, JSON.stringify(value, null, 2), project);

// WebSocket
export function createWebSocketConnection(onMessage: (data: unknown) => void): WebSocket {
  const ws = new WebSocket(`ws://${window.location.host}/ws`);

  ws.onopen = () => {
    console.log('WebSocket connected');
  };

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    onMessage(data);
  };

  ws.onclose = () => {
    console.log('WebSocket disconnected');
  };

  ws.onerror = (error) => {
    console.error('WebSocket error:', error);
  };

  return ws;
}
