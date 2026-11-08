import fs from 'node:fs';
import path from 'node:path';
import { Readable } from 'node:stream';
import { apply } from '../index.js';

const root = path.resolve('books/system-log');
const project = fs.realpathSync(root);
const tools = new Map();
const routes = new Map();

const ctx = {
  get(name) {
    if (name === 'tools') {
      return {
        register(definition) {
          tools.set(definition.name, definition);
          return () => {};
        }
      };
    }
    if (name === 'webServer') {
      return {
        register(route) {
          routes.set(route.path, route);
          return () => {};
        }
      };
    }
    if (name === 'agents') {
      return { get: () => undefined };
    }
    if (name === 'workingDirectory') {
      return { ensure: async () => process.cwd() };
    }
    return undefined;
  },
  effect(callback) {
    const disposer = callback();
    return typeof disposer === 'function' ? disposer : () => {};
  }
};

apply(ctx, { projectRoot: '' });

function makeResponse() {
  let resolve;
  const done = new Promise((resolver) => {
    resolve = resolver;
  });
  return {
    status: 0,
    headers: {},
    body: '',
    writeHead(status, headers) {
      this.status = status;
      this.headers = headers;
    },
    end(body) {
      this.body = body || '';
      resolve(this);
    },
    done
  };
}

function makeGet(url) {
  return {
    url,
    method: 'GET',
    headers: {},
    socket: { remoteAddress: '127.0.0.1' }
  };
}

function makePost(url, body) {
  const stream = Readable.from([Buffer.from(JSON.stringify(body), 'utf8')]);
  stream.url = url;
  stream.method = 'POST';
  stream.headers = { 'content-type': 'application/json' };
  stream.socket = { remoteAddress: '127.0.0.1' };
  return stream;
}

async function route(pathname, req) {
  const entry = routes.get(pathname);
  if (!entry) throw new Error(`missing route ${pathname}`);
  const res = makeResponse();
  await entry.handler(req, res);
  return { status: res.status, json: JSON.parse(res.body) };
}

const checks = [];
checks.push(['tools', [...tools.keys()].sort().join(',')]);
checks.push(['routes', [...routes.keys()].sort().join(',')]);

const projects = await route('/plugins/novelgen/projects', makeGet('/plugins/novelgen/projects?sessionId=test'));
checks.push(['projects.ok', projects.json.ok]);
checks.push(['projects.count', projects.json.projects.length]);
checks.push([
  'projects.roots',
  projects.json.projects.map((item) => `${item.name}=${item.root}`).join(' | ')
]);

const state = await route(
  '/plugins/novelgen/state',
  makeGet(`/plugins/novelgen/state?sessionId=test&project=${encodeURIComponent(project)}`)
);
checks.push(['state.ok', state.json.ok]);
checks.push(['state.resources', state.json.state.resources.length]);
checks.push([
  'state.updatedAt',
  state.json.state.resources.filter((resource) => resource.meta && resource.meta.updatedAt).length
]);
checks.push(['state.next', JSON.stringify(state.json.state.next)]);

const preview = await route(
  '/plugins/novelgen/preview',
  makeGet(
    `/plugins/novelgen/preview?sessionId=test&project=${encodeURIComponent(project)}&id=${encodeURIComponent('chapter:P1-V1-C1')}`
  )
);
checks.push(['preview.ok', preview.json.ok]);
checks.push(['preview.bytes', (preview.json.preview && preview.json.preview.text || '').length]);

const selection = await route(
  '/plugins/novelgen/selection',
  makePost('/plugins/novelgen/selection', {
    sessionId: 'test',
    project,
    ids: ['chapter:P1-V1-C1', 'character:李侑'],
    intent: 'write',
    note: 'harness'
  })
);
checks.push(['selection.ok', selection.json.ok]);
checks.push(['selection.bytes', (selection.json.prompt || '').length]);

const statusTool = tools.get('novelgen_status');
const statusValue = await statusTool.execute({ projectDir: project }, { agent: { id: 'test' } });
checks.push(['tool.status.ok', statusValue.ok]);
checks.push(['tool.status.next', `${statusValue.next.stage}:${statusValue.next.targetId}`]);

const resourcesTool = tools.get('novelgen_resources');
const resourcesValue = await resourcesTool.execute(
  { projectDir: project, kind: 'chapter', limit: 2 },
  { agent: { id: 'test' } }
);
checks.push(['tool.resources.count', resourcesValue.resources.length]);

const prepareTool = tools.get('novelgen_prepare');
const prepareValue = await prepareTool.execute(
  { projectDir: project, stage: 'craft', targetId: 'character:李侑', intent: 'improve' },
  { agent: { id: 'test' } }
);
checks.push(['tool.prepare.ok', prepareValue.ok]);
checks.push(['tool.prepare.mode', prepareValue.mode]);
checks.push(['tool.prepare.skill', prepareValue.skill]);
checks.push(['tool.prepare.task', prepareValue.taskFile && fs.existsSync(prepareValue.taskFile)]);

const outlinePrepare = await prepareTool.execute(
  { projectDir: project, stage: 'outline', targetId: 'volume:P1-V1', intent: 'improve' },
  { agent: { id: 'test' } }
);
checks.push(['tool.prepare.outline.mode', outlinePrepare.mode]);
checks.push(['tool.prepare.outline.skill', outlinePrepare.skill]);
checks.push(['tool.prepare.outline.target', JSON.stringify(outlinePrepare.apply)]);

const partPrepare = await prepareTool.execute(
  { projectDir: project, stage: 'outline', targetId: 'part:P1', intent: 'improve' },
  { agent: { id: 'test' } }
);
checks.push(['tool.prepare.part.mode', partPrepare.mode]);
checks.push(['tool.prepare.part.target', JSON.stringify(partPrepare.apply)]);

const outlineNoTarget = await prepareTool.execute(
  { projectDir: project, stage: 'outline', intent: 'improve' },
  { agent: { id: 'test' } }
);
checks.push(['tool.prepare.outline.noTarget.mode', outlineNoTarget.mode]);
checks.push(['tool.prepare.outline.noTarget.target', JSON.stringify(outlineNoTarget.apply)]);

const applyTool = tools.get('novelgen_apply');
const applyValue = await applyTool.execute(
  {
    projectDir: project,
    stage: 'craft',
    target: 'character',
    id: '李侑',
    payload: { notes: 'harness dry-run only' },
    apply: false
  },
  { agent: { id: 'test' } }
);
checks.push(['tool.apply.ok', applyValue.ok]);
checks.push(['tool.apply.applied', applyValue.applied]);
checks.push(['tool.apply.blocking', applyValue.dryRun && applyValue.dryRun.check && applyValue.dryRun.check.blocking]);

for (const [key, value] of checks) console.log(`${key}=${value}`);
