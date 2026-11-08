import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { buildPreview, buildSnapshot, publicState } from '../lib/state.js';

const requireFromWebui = createRequire(path.join(process.cwd(), 'webui/frontend/package.json'));
const React = requireFromWebui('react');
const ReactDOMServer = requireFromWebui('react-dom/server');

const root = path.resolve('books/system-log');
const snapshot = buildSnapshot(root);
const state = publicState(snapshot);
const projects = [{ name: snapshot.project.name, root: snapshot.project.root, updatedAt: snapshot.project.updatedAt }];
const previews = {
  setup: buildPreview(snapshot, 'setup'),
  volume: buildPreview(snapshot, 'volume:P1-V1'),
  character: buildPreview(snapshot, 'character:李侑'),
  chapter: buildPreview(snapshot, 'chapter:P1-V1-C1')
};

let captured = null;
const windowShim = {
  __ModuleLoader__: {
    load(definition) {
      captured = definition;
    }
  }
};
if (typeof globalThis.navigator === 'undefined') {
  globalThis.navigator = { language: 'zh-CN' };
}
const source = fs.readFileSync(path.resolve('dsh-novelgen/client.js'), 'utf8');
new Function('window', source)(windowShim);
if (!captured) throw new Error('client module was not registered');
const react = requireFromWebui('react');
const plugin = captured.factory((name) => {
  if (name === 'react') return react;
  throw new Error(`unexpected require: ${name}`);
});

const components = new Map();
let activeSlot = '';
const ctx = {
  effect(callback) {
    const disposer = callback();
    return typeof disposer === 'function' ? disposer : () => {};
  },
  slots: {
    inject(key, callback) {
      activeSlot = key;
      callback();
    },
    register(options, component) {
      components.set(activeSlot, component);
      return () => {};
    }
  }
};
plugin.apply(ctx);
if (components.has('conversation.input.dock')) throw new Error('dock registration should be removed');
if (!components.has('conversation.view')) throw new Error('view component not registered');

const view = components.get('conversation.view');

function render(extra) {
  return ReactDOMServer.renderToStaticMarkup(
    React.createElement(view, {
      sessionId: 'test',
      inputActions: {},
      initialData: state,
      initialProjects: projects,
      ...extra
    })
  );
}

for (const tab of ['flow', 'resources', 'graph']) {
  const html = render({ initialTab: tab, initialFocusId: 'chapter:P1-V1-C1' });
  console.log(`view:${tab}`, {
    bytes: html.length,
    root: /class="ng-root ng-view"/.test(html),
    panel: /class="ng-panel/.test(html),
    tabs: /class="ng-tab/.test(html),
    graph: /class="ng-graph/.test(html),
    flow: /class="ng-flow"/.test(html),
    recent: html.includes('最近变更'),
    run: html.includes('连续运行'),
    filters: html.includes('按卷筛选'),
    undefinedText: html.includes('undefined')
  });
}

const viewFlow = render({
  initialTab: 'flow',
  initialStage: 'setup',
  initialFocusId: 'setup',
  initialPreview: previews.setup
});
console.log('view:setup', {
  bytes: viewFlow.length,
  hero: /class="ng-preview-hero"/.test(viewFlow),
  cards: /class="ng-card-grid"/.test(viewFlow),
  stats: /class="ng-stat-grid"/.test(viewFlow),
  raw: viewFlow.includes('原始数据'),
  undefinedText: viewFlow.includes('undefined')
});

const viewOutline = render({
  initialTab: 'flow',
  initialStage: 'outline',
  initialFocusId: 'volume:P1-V1',
  initialPreview: previews.volume
});
console.log('view:outline', {
  bytes: viewOutline.length,
  rows: /class="ng-list-row"/.test(viewOutline),
  chapterList: viewOutline.includes('第1章'),
  stats: /class="ng-stat-grid"/.test(viewOutline),
  undefinedText: viewOutline.includes('undefined')
});

const viewCharacter = render({
  initialTab: 'flow',
  initialStage: 'craft',
  initialFocusId: 'character:李侑',
  initialPreview: previews.character
});
console.log('view:character', {
  bytes: viewCharacter.length,
  quote: /class="ng-quote"/.test(viewCharacter),
  chips: /class="ng-chip-row"/.test(viewCharacter),
  motivation: viewCharacter.includes('动机'),
  undefinedText: viewCharacter.includes('undefined')
});

const viewChapter = render({
  initialTab: 'flow',
  initialStage: 'write',
  initialFocusId: 'chapter:P1-V1-C1',
  initialPreview: previews.chapter
});
console.log('view:chapter', {
  bytes: viewChapter.length,
  timeline: /class="ng-timeline"/.test(viewChapter),
  stats: /class="ng-stat-grid"/.test(viewChapter),
  undefinedText: viewChapter.includes('undefined')
});
