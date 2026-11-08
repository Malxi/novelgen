import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  buildPreview,
  compactResources,
  findProjectRoot,
  isValidProject,
  publicState,
  readJsonFile,
  snapshotFor
} from './lib/state.js';
import { applyPatch, resolveNovelgenExe, toolRefresh } from './lib/cli.js';
import { buildSelectionPrompt, prepareTask } from './lib/workbench.js';

const API_BASE = '/plugins/novelgen';
const MAX_BODY_BYTES = 2 * 1024 * 1024;
const MODULE_DIR = path.dirname(fileURLToPath(import.meta.url));

const DEFAULTS = {
  projectRoot: '',
  projectRoots: [],
  novelgenPath: ''
};

function readBody(req, limit = MAX_BODY_BYTES) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > limit) {
        reject(new Error('request body too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      const text = Buffer.concat(chunks).toString('utf8');
      if (!text.trim()) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(text));
      } catch {
        reject(new Error('request body is not valid JSON'));
      }
    });
    req.on('error', reject);
  });
}

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store',
    'content-length': Buffer.byteLength(body)
  });
  res.end(body);
}

function isLoopbackAddress(address) {
  if (!address) return true;
  return (
    address === '127.0.0.1' ||
    address === '::1' ||
    address === '::ffff:127.0.0.1' ||
    address.startsWith('127.')
  );
}

function localOnly(handler, getConnection) {
  return async (req, res) => {
    if (!isLoopbackAddress(req.socket && req.socket.remoteAddress)) {
      sendJson(res, 403, { ok: false, error: 'Novelgen Studio is available on loopback only.' });
      return;
    }
    const connection = typeof getConnection === 'function' ? getConnection() : null;
    if (connection && typeof connection.requestRejection === 'function') {
      try {
        const rejection = connection.requestRejection({ headers: req.headers || {} });
        if (rejection) {
          sendJson(res, rejection, {
            ok: false,
            error: 'authentication required; reopen the DSH Web URL'
          });
          return;
        }
      } catch {
        sendJson(res, 403, { ok: false, error: 'request rejected by DSH connection policy' });
        return;
      }
    }
    await handler(req, res);
  };
}

function compactError(error) {
  if (error instanceof Error) return error.message;
  return String(error);
}

function losslessJson(value) {
  // Tool results cross a lossless-JSON boundary: `undefined`, functions, and
  // other non-JSON values are rejected. Round-trip once at the tool edge.
  return JSON.parse(JSON.stringify(value));
}

class SessionProjectStore {
  constructor() {
    this.file = path.join(os.homedir(), '.dsh', 'novelgen', 'sessions.json');
    this.data = { sessions: {}, recent: [] };
    try {
      const parsed = JSON.parse(fs.readFileSync(this.file, 'utf8'));
      if (parsed && typeof parsed === 'object') {
        this.data.sessions = parsed.sessions && typeof parsed.sessions === 'object' ? parsed.sessions : {};
        this.data.recent = Array.isArray(parsed.recent) ? parsed.recent.filter((item) => typeof item === 'string') : [];
      }
    } catch {
      // first run or unreadable state: use defaults
    }
  }

  getSession(sessionId) {
    if (!sessionId) return '';
    const entry = this.data.sessions[sessionId];
    return entry && typeof entry.projectDir === 'string' ? entry.projectDir : '';
  }

  setSession(sessionId, projectDir) {
    if (!sessionId || !projectDir) return;
    this.data.sessions[sessionId] = { projectDir, updatedAt: new Date().toISOString() };
    this.data.recent = [projectDir, ...this.data.recent.filter((item) => item !== projectDir)].slice(0, 16);
    this.save();
  }

  recent() {
    return this.data.recent.slice();
  }

  save() {
    try {
      fs.mkdirSync(path.dirname(this.file), { recursive: true });
      fs.writeFileSync(this.file, JSON.stringify(this.data, null, 2), 'utf8');
    } catch {
      // A read-only home must not break read-only tooling.
    }
  }
}

function projectMeta(root) {
  const doc = readJsonFile(path.join(root, 'novel.json')) || {};
  let updatedAt = doc.updated_at || '';
  if (!updatedAt) {
    try {
      updatedAt = fs.statSync(path.join(root, 'novel.json')).mtime.toISOString();
    } catch {
      updatedAt = '';
    }
  }
  return {
    root,
    name: doc.name || path.basename(root),
    language: doc.language || '',
    updatedAt
  };
}

function addCandidate(set, value) {
  if (!value || typeof value !== 'string') return;
  try {
    const resolved = path.resolve(value);
    if (isValidProject(resolved)) set.add(fs.realpathSync(resolved));
  } catch {
    // ignore
  }
}

function scanChildren(set, base) {
  if (!base) return;
  try {
    const resolved = path.resolve(base);
    if (isValidProject(resolved)) {
      set.add(fs.realpathSync(resolved));
      return;
    }
    for (const entry of fs.readdirSync(resolved, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const child = path.join(resolved, entry.name);
      if (isValidProject(child)) set.add(fs.realpathSync(child));
    }
  } catch {
    // ignore missing/inaccessible roots
  }
}

function discoverProjects(config, store, sessionId, sessionCwd) {
  const candidates = new Set();
  addCandidate(candidates, store.getSession(sessionId));
  for (const recent of store.recent()) addCandidate(candidates, recent);
  addCandidate(candidates, config.projectRoot);
  scanChildren(candidates, config.projectRoot);
  for (const base of Array.isArray(config.projectRoots) ? config.projectRoots : []) {
    scanChildren(candidates, base);
  }
  addCandidate(candidates, sessionCwd);
  scanChildren(candidates, path.join(sessionCwd || process.cwd(), 'books'));
  scanChildren(candidates, sessionCwd || process.cwd());
  const cwd = process.cwd();
  addCandidate(candidates, cwd);
  scanChildren(candidates, path.join(cwd, 'books'));
  scanChildren(candidates, cwd);
  // Development checkout: the bundle lives in <repo>/dsh-novelgen, so its
  // sibling books/ directory is a natural project library even before the
  // session has a live agent.
  addCandidate(candidates, MODULE_DIR);
  scanChildren(candidates, path.join(MODULE_DIR, 'books'));
  scanChildren(candidates, path.join(MODULE_DIR, '..', 'books'));
  return [...candidates]
    .map((root) => projectMeta(root))
    .sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)) || a.name.localeCompare(b.name))
    .slice(0, 80);
}

function noProjectError(candidates) {
  const names = candidates.slice(0, 8).map((item) => `- ${item.name}: ${item.root}`);
  return [
    'No Novelgen project found (a project directory contains novel.json).',
    'Pass projectDir explicitly, open the session inside the project, or configure projectRoot in the plugin config.',
    names.length ? `Candidates:\n${names.join('\n')}` : ''
  ]
    .filter(Boolean)
    .join('\n');
}

export function apply(ctx, config = {}) {
  const pluginConfig = {
    ...DEFAULTS,
    ...(config && typeof config === 'object' ? config : {})
  };
  if (!Array.isArray(pluginConfig.projectRoots)) pluginConfig.projectRoots = [];
  const store = new SessionProjectStore();
  const agents = ctx.get('agents');
  const workingDirectory = ctx.get('workingDirectory');

  async function cwdFor(sessionId, agent) {
    try {
      if (agent && workingDirectory) {
        const cwd = await workingDirectory.ensure(agent);
        if (cwd) return cwd;
      }
    } catch {
      // fall through to the saved/configured root
    }
    try {
      if (sessionId && agents) {
        const live = agents.get(sessionId);
        if (live && workingDirectory) return await workingDirectory.ensure(live);
      }
    } catch {
      // ignore
    }
    return '';
  }

  async function resolveProject({ sessionId, explicit, agent } = {}) {
    if (explicit) {
      if (isValidProject(explicit)) return fs.realpathSync(path.resolve(explicit));
      const found = findProjectRoot(explicit);
      if (found) return fs.realpathSync(found);
    }
    if (sessionId) {
      const saved = store.getSession(sessionId);
      if (saved && isValidProject(saved)) return fs.realpathSync(saved);
    }
    const cwd = await cwdFor(sessionId, agent);
    const start = cwd || pluginConfig.projectRoot || process.cwd();
    const found = findProjectRoot(start);
    if (found) return fs.realpathSync(found);
    if (pluginConfig.projectRoot && isValidProject(pluginConfig.projectRoot)) {
      return fs.realpathSync(path.resolve(pluginConfig.projectRoot));
    }
    return null;
  }

  function projectSnapshot(root) {
    const snapshot = snapshotFor(root);
    snapshot.exe = resolveNovelgenExe(root, pluginConfig);
    return snapshot;
  }

  async function requireProject(options) {
    const root = await resolveProject(options);
    if (root) {
      const snapshot = projectSnapshot(root);
      snapshot.workspace = (await cwdFor(options?.sessionId, options?.agent)) || root;
      return snapshot;
    }
    const candidates = discoverProjects(
      pluginConfig,
      store,
      options?.sessionId,
      await cwdFor(options?.sessionId, options?.agent)
    );
    const error = new Error(noProjectError(candidates));
    error.code = 'NOVELGEN_NO_PROJECT';
    error.candidates = candidates;
    throw error;
  }

  // ---- agent tools ------------------------------------------------------
  const tools = ctx.get('tools');
  if (tools) {
    const register = (definition) => {
      const { outputSchema, ...tool } = definition;
      ctx.effect(() =>
        tools.register({
          ...tool,
          output: {
            schema: outputSchema || { type: 'object', additionalProperties: true },
            render: (_args, value) => [{ type: 'text', text: JSON.stringify(value, null, 2) }]
          }
        })
      );
    };

    register({
      name: 'novelgen_status',
      description:
        'Inspect a Novelgen novel project and return stage progress, resource counts, and the single best next creative step. Use this first for any novelgen request. Read-only.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string', description: 'Novelgen project directory (contains novel.json). Omit to use the session project.' },
          includeResources: { type: 'boolean', description: 'Include a compact resource list (default false).' }
        },
        additionalProperties: false
      },
      timeoutMs: 60000,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        const value = {
          ok: true,
          project: snapshot.project,
          counts: snapshot.counts,
          stages: snapshot.stages,
          next: snapshot.next,
          guidance:
            snapshot.next && snapshot.next.stage === 'review'
              ? [
                  'This next step is a read-only review: call novelgen_prepare and pass delegate.prompt to the subagent, then report the review JSON without novelgen_apply.',
                  'If the user approves fixes, prepare a write/improve task separately.'
                ]
              : [
                  'For concrete creation call novelgen_prepare, then pass the returned delegate.prompt verbatim to the subagent tool.',
                  'After the subagent returns a JSON object call novelgen_apply with the same stage/target/id.',
                  'For long-range work, record the objective with create_goal or schedule_create and repeat status -> prepare -> subagent -> apply.'
                ]
        };
        if (args.includeResources) value.resources = compactResources(snapshot, {}).resources;
        return value;
      }
    });

    register({
      name: 'novelgen_resources',
      description:
        'List or search Novelgen resources: setup, outline parts/volumes/chapters, craft characters/locations/items/organizations, storylines, and recaps. Read-only; use ids with novelgen_prepare or novelgen_select.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string', description: 'Novelgen project directory. Omit to use the session project.' },
          kind: {
            type: 'string',
            enum: ['setup', 'part', 'volume', 'chapter', 'character', 'location', 'item', 'organization', 'storyline', 'recap', 'all'],
            description: 'Filter by resource kind.'
          },
          query: { type: 'string', description: 'Case-insensitive substring over id, title, summary, and path.' },
          status: { type: 'string', description: 'Filter by status: done, active, missing, mention.' },
          ids: { type: 'array', items: { type: 'string' }, description: 'Exact resource ids; returns full summaries for these ids.' },
          preview: { type: 'boolean', description: 'With ids, include the same preview text the UI shows.' },
          limit: { type: 'integer', minimum: 1, maximum: 500, description: 'Maximum rows (default 200).' }
        },
        additionalProperties: false
      },
      timeoutMs: 60000,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        if (Array.isArray(args.ids) && args.ids.length) {
          const resources = args.ids.map((id) => {
            const resource = snapshot.index.get(String(id));
            if (!resource) return { id, missing: true };
            const base = {
              id: resource.id,
              kind: resource.kind,
              title: resource.title,
              status: resource.status,
              summary: resource.summary,
              path: resource.path,
              words: resource.words,
              degree: resource.degree,
              meta: resource.meta
            };
            if (args.preview) base.preview = buildPreview(snapshot, id);
            return base;
          });
          return { ok: true, project: snapshot.project, count: resources.length, resources };
        }
        const list = compactResources(snapshot, args);
        return {
          ok: true,
          project: snapshot.project,
          total: list.total,
          resources: list.resources
        };
      }
    });

    register({
      name: 'novelgen_prepare',
      description:
        'Prepare one creative step for delegation. Returns a self-contained delegate.prompt for the subagent tool, a task packet path with context, and the exact novelgen_apply arguments. For missing outline structure it returns a direct compose pipeline command instead. Read-only for the project.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string' },
          stage: {
            type: 'string',
            enum: ['next', 'setup', 'outline', 'craft', 'write', 'recap', 'review'],
            description: 'Creative stage. Omit or use next to follow novelgen_status.next.'
          },
          targetId: { type: 'string', description: 'Resource id such as chapter:P1-V1-C1, character:Lin, volume:P1-V1, or setup.' },
          id: { type: 'string', description: 'Alias for targetId.' },
          chapterId: { type: 'string', description: 'Bare chapter id, expanded to chapter:<id>.' },
          intent: { type: 'string', description: 'write, improve, review, plan, recap, or auto.' },
          idea: { type: 'string', description: 'User idea/prompt for setup generation.' },
          note: { type: 'string', description: 'Additional creative guidance.' }
        },
        additionalProperties: false
      },
      timeoutMs: 300000,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        return await prepareTask({ snapshot, config: pluginConfig, args });
      }
    });

    register({
      name: 'novelgen_apply',
      description:
        'Validate and apply a subagent JSON result to a Novelgen project: complete setup/outline artifacts use tool replace, while craft/chapter/recap and targeted outline improvements use tool patch (dry-run first, then --apply when non-blocking). Returns changes, checks, and the next step. This writes project state.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string' },
          stage: {
            type: 'string',
            enum: ['setup', 'outline', 'craft', 'chapter', 'write', 'recap'],
            description: 'Patch stage. chapter and write are aliases.'
          },
          target: {
            type: 'string',
            enum: ['chapter', 'volume', 'character', 'item', 'location', 'organization'],
            description: 'Outline/craft patch target.'
          },
          id: { type: 'string', description: 'Chapter id, volume id, or craft name.' },
          payload: { type: 'object', description: 'The JSON object returned by the subagent (not a string).' },
          apply: { type: 'boolean', description: 'Default true. Set false for a dry-run only.' },
          refreshDerived: { type: 'boolean', description: 'For chapter patches, also refresh derived RPG DSL (default false; prefer novelgen_refresh).' }
        },
        required: ['stage', 'payload'],
        additionalProperties: false
      },
      timeoutMs: 600000,
      isConcurrencySafe: () => false,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        const stage = args.stage === 'write' ? 'chapter' : args.stage;
        if (!['setup', 'outline', 'craft', 'chapter', 'recap'].includes(stage)) {
          return { ok: false, phase: 'input', error: `unsupported stage: ${args.stage}` };
        }
        if (!args.payload || typeof args.payload !== 'object') {
          return { ok: false, phase: 'input', error: 'payload must be a JSON object' };
        }
        const result = await applyPatch({
          root: snapshot.project.root,
          config: pluginConfig,
          stage,
          target: args.target,
          id: args.id,
          payload: args.payload,
          apply: args.apply !== false,
          refreshDerived: args.refreshDerived === true
        });
        let next = null;
        try {
          const refreshed = projectSnapshot(snapshot.project.root);
          next = refreshed.next;
        } catch {
          next = null;
        }
        return losslessJson({
          ok: result.applied === true || result.ok !== false,
          projectDir: snapshot.project.root,
          stage,
          target: args.target,
          id: args.id,
          ...result,
          next
        });
      }
    });

    register({
      name: 'novelgen_refresh',
      description:
        'Refresh derived Novelgen artifacts explicitly. Use chapter-dsl for one written chapter after novelgen_apply, or narrative-memory after a batch of changes. This writes derived files only, never story source.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string' },
          target: {
            type: 'string',
            enum: ['chapter-dsl', 'narrative-memory'],
            description: 'Derived artifact to rebuild.'
          },
          id: { type: 'string', description: 'Chapter id for chapter-dsl.' },
          batchSize: { type: 'number', description: 'Chapter batch size for chapter-dsl (default 10).' }
        },
        required: ['target'],
        additionalProperties: false
      },
      timeoutMs: 600000,
      isConcurrencySafe: () => false,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        const target = args.target === 'narrative-memory' ? 'narrative-memory' : 'chapter-dsl';
        if (target === 'chapter-dsl' && !args.id) {
          return { ok: false, error: 'id is required for chapter-dsl refresh' };
        }
        const result = await toolRefresh(snapshot.project.root, target, {
          config: pluginConfig,
          id: args.id,
          batchSize: args.batchSize
        });
        return losslessJson({
          ok: result.ok,
          projectDir: snapshot.project.root,
          target,
          id: args.id,
          result: result.json,
          error: result.error,
          stderr: result.stderr
        });
      }
    });

    register({
      name: 'novelgen_select',
      description:
        'Turn selected Novelgen resource ids into a select prompt suitable for the main conversation input. This mirrors the UI selection tray. Read-only.',
      parameters: {
        type: 'object',
        properties: {
          projectDir: { type: 'string' },
          ids: { type: 'array', items: { type: 'string' }, description: 'Resource ids from novelgen_resources.' },
          intent: {
            type: 'string',
            enum: ['auto', 'write', 'improve', 'review', 'plan', 'recap', 'ask'],
            description: 'What the user wants to do with the selection.'
          },
          note: { type: 'string', description: 'Additional user instruction.' }
        },
        required: ['ids'],
        additionalProperties: false
      },
      timeoutMs: 30000,
      async execute(args = {}, exec) {
        const snapshot = await requireProject({
          sessionId: exec?.agent?.id,
          explicit: args.projectDir,
          agent: exec?.agent
        });
        return {
          ok: true,
          projectDir: snapshot.project.root,
          prompt: buildSelectionPrompt(snapshot, args.ids, args.intent, args.note)
        };
      }
    });
  }

  // ---- Web UI bridge ----------------------------------------------------
  const web = ctx.get('webServer');
  if (web) {
    async function stateFromRequest(url) {
      const sessionId = url.searchParams.get('sessionId') || '';
      const explicit = url.searchParams.get('project') || '';
      const root = await resolveProject({ sessionId, explicit });
      if (!root) {
        return {
          ok: false,
          error: 'no-project',
          message: 'No Novelgen project selected. Pick one or open the session inside a project.',
          projects: discoverProjects(pluginConfig, store, sessionId, await cwdFor(sessionId))
        };
      }
      const snapshot = projectSnapshot(root);
      return {
        ok: true,
        state: publicState(snapshot),
        projects: discoverProjects(pluginConfig, store, sessionId, await cwdFor(sessionId))
      };
    }

    const routes = [
      {
        kind: 'exact',
        path: `${API_BASE}/state`,
        handler: async (req, res) => {
          try {
            const url = new URL(req.url || '/', 'http://localhost');
            sendJson(res, 200, await stateFromRequest(url));
          } catch (error) {
            sendJson(res, 500, { ok: false, error: compactError(error) });
          }
        }
      },
      {
        kind: 'exact',
        path: `${API_BASE}/projects`,
        handler: async (req, res) => {
          try {
            const url = new URL(req.url || '/', 'http://localhost');
            const sessionId = url.searchParams.get('sessionId') || '';
            const root = await resolveProject({ sessionId });
            const cwd = await cwdFor(sessionId);
            sendJson(res, 200, {
              ok: true,
              current: root ? projectMeta(root) : null,
              projects: discoverProjects(pluginConfig, store, sessionId, cwd)
            });
          } catch (error) {
            sendJson(res, 500, { ok: false, error: compactError(error) });
          }
        }
      },
      {
        kind: 'exact',
        path: `${API_BASE}/preview`,
        handler: async (req, res) => {
          try {
            const url = new URL(req.url || '/', 'http://localhost');
            const sessionId = url.searchParams.get('sessionId') || '';
            const explicit = url.searchParams.get('project') || '';
            const id = url.searchParams.get('id') || '';
            const root = await resolveProject({ sessionId, explicit });
            if (!root) {
              sendJson(res, 200, { ok: false, error: 'no-project' });
              return;
            }
            const preview = buildPreview(projectSnapshot(root), id);
            if (!preview) {
              sendJson(res, 404, { ok: false, error: `resource not found: ${id}` });
              return;
            }
            sendJson(res, 200, { ok: true, preview });
          } catch (error) {
            sendJson(res, 500, { ok: false, error: compactError(error) });
          }
        }
      },
      {
        kind: 'exact',
        path: `${API_BASE}/project`,
        handler: async (req, res) => {
          try {
            if (req.method !== 'POST') {
              sendJson(res, 405, { ok: false, error: 'POST required' });
              return;
            }
            const body = await readBody(req);
            const sessionId = String(body.sessionId || '');
            const projectDir = String(body.projectDir || '');
            if (!isValidProject(projectDir)) {
              sendJson(res, 400, { ok: false, error: `not a Novelgen project: ${projectDir}` });
              return;
            }
            const root = fs.realpathSync(path.resolve(projectDir));
            if (sessionId) store.setSession(sessionId, root);
            sendJson(res, 200, { ok: true, project: projectMeta(root) });
          } catch (error) {
            sendJson(res, 400, { ok: false, error: compactError(error) });
          }
        }
      },
      {
        kind: 'exact',
        path: `${API_BASE}/selection`,
        handler: async (req, res) => {
          try {
            if (req.method !== 'POST') {
              sendJson(res, 405, { ok: false, error: 'POST required' });
              return;
            }
            const body = await readBody(req);
            const sessionId = String(body.sessionId || '');
            const root = await resolveProject({ sessionId, explicit: body.project });
            if (!root) {
              sendJson(res, 200, { ok: false, error: 'no-project' });
              return;
            }
            const ids = Array.isArray(body.ids) ? body.ids : [];
            if (!ids.length) {
              sendJson(res, 400, { ok: false, error: 'select at least one resource' });
              return;
            }
            const snapshot = projectSnapshot(root);
            const prompt = buildSelectionPrompt(snapshot, ids, body.intent, body.note);
            sendJson(res, 200, { ok: true, projectDir: snapshot.project.root, prompt });
          } catch (error) {
            sendJson(res, 400, { ok: false, error: compactError(error) });
          }
        }
      }
    ];

    for (const route of routes) {
      ctx.effect(() => web.register({ ...route, handler: localOnly(route.handler, () => ctx.get('connection')) }));
    }
  }
}
