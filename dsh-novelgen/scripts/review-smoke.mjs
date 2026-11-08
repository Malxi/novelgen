import fs from 'node:fs';
import path from 'node:path';
import { buildSnapshot } from '../lib/state.js';
import { resolveNovelgenExe } from '../lib/cli.js';
import { prepareTask } from '../lib/workbench.js';

const source = path.resolve('books/system-log');
const target = path.resolve('.dsh-novelgen/review-project');
fs.rmSync(target, { recursive: true, force: true });
for (const dir of ['story/setup', 'story/compose', 'story/craft', 'story/recaps', 'chapters']) {
  fs.mkdirSync(path.join(target, dir), { recursive: true });
}
fs.copyFileSync(path.join(source, 'novel.json'), path.join(target, 'novel.json'));
fs.copyFileSync(path.join(source, 'story/setup/story_setup.json'), path.join(target, 'story/setup/story_setup.json'));
fs.copyFileSync(path.join(source, 'story/compose/outline.json'), path.join(target, 'story/compose/outline.json'));
for (const name of ['characters.json', 'locations.json']) {
  fs.copyFileSync(path.join(source, 'story/craft', name), path.join(target, 'story/craft', name));
}
const prose = [
  '# 第1章：倒计时',
  '',
  '李侑在清晨醒来，先看见窗纸上跳动的光。',
  '他伸手去摸枕边的木牌，指尖发凉，脑子里却浮出一行不属于自己的字。',
  '顾恒站在外门石阶上，盯着远处的钟楼，没有回头。',
  '两个人都不知道，倒计时已经开始了。'
].join('\n');
fs.writeFileSync(path.join(target, 'chapters/chapter-P1-V1-C1.md'), prose, 'utf8');

const snapshot = buildSnapshot(target);
snapshot.exe = resolveNovelgenExe(target, {});
snapshot.workspace = path.resolve('.');
console.log('written', snapshot.outlineChapters[0].written, snapshot.outlineChapters[0].words);
const result = await prepareTask({
  snapshot,
  config: {},
  args: { stage: 'review', targetId: 'chapter:P1-V1-C1', intent: 'review' }
});
console.log({
  ok: result.ok,
  mode: result.mode,
  skill: result.skill,
  apply: result.apply,
  task: result.taskFile && fs.existsSync(result.taskFile),
  context: result.contextSummary,
  warnings: result.warnings
});
