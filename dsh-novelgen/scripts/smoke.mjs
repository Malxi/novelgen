import fs from 'node:fs';
import path from 'node:path';
import { buildSnapshot, publicState, buildPreview } from '../lib/state.js';
import { resolveNovelgenExe, toolQuery, applyPatch } from '../lib/cli.js';
import { buildSelectionPrompt, prepareTask } from '../lib/workbench.js';

const root = path.resolve(process.argv[2] || 'books/system-log');
const config = {};
const snapshot = buildSnapshot(root);
snapshot.exe = resolveNovelgenExe(root, config);
snapshot.workspace = path.resolve('.');

console.log('PROJECT', snapshot.project);
console.log('COUNTS', snapshot.counts);
console.log('STAGES', snapshot.stages);
console.log('NEXT', snapshot.next);
console.log('RESOURCES', snapshot.resources.length, 'EDGES', snapshot.edges.length);
console.log(
  'FIRST',
  snapshot.resources.slice(0, 8).map((r) => `${r.kind}:${r.title}`)
);
console.log('PREVIEW', (buildPreview(snapshot, 'setup') || {}).text?.slice(0, 120));

const state = publicState(snapshot);
const bytes = JSON.stringify(state).length;
console.log('PUBLIC_STATE_BYTES', bytes);

const selection = buildSelectionPrompt(
  snapshot,
  ['chapter:P1-V1-C1', 'character:李侑', 'volume:P1-V1'],
  'write',
  '先写第一章'
);
console.log('SELECTION\n' + selection);

const prepared = await prepareTask({
  snapshot,
  config,
  args: { stage: 'write', targetId: 'chapter:P1-V1-C1', intent: 'write' }
});
console.log('PREPARED', {
  ok: prepared.ok,
  mode: prepared.mode,
  stage: prepared.stage,
  skill: prepared.skill,
  taskFile: prepared.taskFile,
  contextFile: prepared.contextFile,
  contextSummary: prepared.contextSummary,
  delegate: prepared.delegate?.description,
  apply: prepared.apply,
  warnings: prepared.warnings
});
if (prepared.taskFile) {
  console.log('TASK_EXISTS', fs.existsSync(prepared.taskFile), fs.statSync(prepared.taskFile).size);
}

const craft = await prepareTask({
  snapshot,
  config,
  args: { stage: 'craft', targetId: 'character:李侑', intent: 'improve' }
});
console.log('CRAFT_PREP', {
  ok: craft.ok,
  mode: craft.mode,
  skill: craft.skill,
  target: craft.target,
  taskFile: craft.taskFile,
  warnings: craft.warnings
});

const dry = await applyPatch({
  root,
  config,
  stage: 'craft',
  target: 'character',
  id: '李侑',
  payload: { notes: 'smoke test dry-run note' },
  apply: false
});
console.log('DRY_RUN', JSON.stringify(dry, null, 2).slice(0, 3000));

const direct = await prepareTask({
  snapshot,
  config,
  args: { stage: 'outline', targetId: 'volume:P9-V9', intent: 'plan' }
});
console.log('DIRECT', direct);
