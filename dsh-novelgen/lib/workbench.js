import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { toolQuery } from './cli.js';

const INTENTS = {
  auto: '根据所选资源判断最合适的下一步',
  write: '续写或生成正文',
  improve: '改进现有内容',
  review: '审校并指出问题',
  plan: '规划或补全结构',
  recap: '抽取或更新章节 recap',
  ask: '只讨论，不修改项目'
};

const CRAFT_TYPES = {
  character: 'craft-character',
  item: 'craft-item',
  location: 'craft-location',
  organization: 'craft-organization'
};

const CRAFT_TARGET = {
  character: 'character',
  item: 'item',
  location: 'location',
  organization: 'organization'
};

const CRAFT_SKILL = {
  character: 'craft-character-workflow',
  item: 'craft-element-workflow',
  location: 'craft-element-workflow',
  organization: 'craft-element-workflow'
};

function slugify(value) {
  return String(value || '')
    .replace(/[^\p{L}\p{N}._-]+/gu, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48) || 'task';
}

function timestamp() {
  const now = new Date();
  const pad = (value) => String(value).padStart(2, '0');
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
}

function cleanupTaskRoot(taskRoot) {
  const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000;
  try {
    for (const entry of fs.readdirSync(taskRoot, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const full = path.join(taskRoot, entry.name);
      try {
        const stat = fs.statSync(full);
        if (stat.mtimeMs < cutoff) fs.rmSync(full, { recursive: true, force: true });
      } catch {
        // best-effort cleanup
      }
    }
  } catch {
    // no task directory yet
  }
}

function truncate(text, limit) {
  const value = typeof text === 'string' ? text : '';
  return value.length > limit ? `${value.slice(0, limit - 1)}…` : value;
}

function chapterIdOf(resourceId) {
  return String(resourceId || '').startsWith('chapter:') ? String(resourceId).slice('chapter:'.length) : '';
}

function volumeIdOf(resourceId) {
  return String(resourceId || '').startsWith('volume:') ? String(resourceId).slice('volume:'.length) : '';
}

function entityOf(resourceId) {
  const value = String(resourceId || '');
  const index = value.indexOf(':');
  if (index < 0) return null;
  const kind = value.slice(0, index);
  if (!CRAFT_TARGET[kind]) return null;
  return { kind, name: value.slice(index + 1) };
}

export function inferTarget(snapshot, args = {}) {
  let targetId = String(args.targetId || args.id || '').trim();
  if (!targetId && args.chapterId) targetId = `chapter:${args.chapterId}`;
  let stage = String(args.stage || '').trim();
  if (!stage || stage === 'next') stage = snapshot.next?.stage || '';
  const intent = String(args.intent || (stage === 'write' ? 'write' : 'auto')).trim();

  if (!targetId) {
    if (stage === 'setup') targetId = 'setup';
    else if (stage === 'outline') {
      if (snapshot.next?.stage === 'outline' && snapshot.next.targetId) targetId = snapshot.next.targetId;
      else if (snapshot.volumes && snapshot.volumes[0]) targetId = `volume:${snapshot.volumes[0].id}`;
    } else if (stage === 'craft') {
      if (snapshot.next?.stage === 'craft' && snapshot.next.targetId) targetId = snapshot.next.targetId;
    } else if ((stage === 'write' || stage === 'recap') && snapshot.next?.stage === stage && snapshot.next.targetId) {
      targetId = snapshot.next.targetId;
    }
  }

  if (!stage) {
    if (targetId === 'setup') stage = 'setup';
    else if (targetId.startsWith('volume:') || targetId.startsWith('part:')) stage = 'outline';
    else if (targetId.startsWith('chapter:')) stage = 'write';
    else if (targetId.startsWith('recap:')) stage = 'recap';
    else if (entityOf(targetId)) stage = 'craft';
    else stage = snapshot.next?.stage || 'write';
  }

  if (stage === 'recap' && String(targetId).startsWith('recap:')) {
    targetId = `chapter:${String(targetId).slice('recap:'.length)}`;
  }
  if (stage === 'craft' && !entityOf(targetId)) {
    const nextEntity = entityOf(snapshot.next?.targetId || '');
    if (nextEntity) targetId = snapshot.next.targetId;
    else {
      const missing = (snapshot.coreCastNames || []).find((name) => !snapshot.craft.characters[name]);
      if (missing) targetId = `character:${missing}`;
    }
  }
  if (stage === 'write' && intent === 'review' && !String(targetId).startsWith('chapter:')) {
    const written = (snapshot.outlineChapters || []).find((chapter) => chapter.written);
    if (written) targetId = written.nodeId;
  }
  if (stage === 'write' && !String(targetId).startsWith('chapter:')) {
    const unwritten = (snapshot.outlineChapters || []).find((chapter) => !chapter.written);
    if (unwritten) targetId = unwritten.nodeId;
    else if (String(snapshot.next?.targetId || '').startsWith('chapter:')) targetId = snapshot.next.targetId;
  }
  if (stage === 'recap' && !String(targetId).startsWith('chapter:')) {
    const unrecapped = (snapshot.outlineChapters || []).find(
      (chapter) => chapter.written && !snapshot.recapFiles.has(chapter.id)
    );
    if (unrecapped) targetId = `chapter:${unrecapped.id}`;
    else if (String(snapshot.next?.targetId || '').startsWith('chapter:')) targetId = snapshot.next.targetId;
  }
  if (stage === 'review' && !String(targetId).startsWith('chapter:')) {
    const chapter =
      (snapshot.outlineChapters || []).find((item) => item.written) || (snapshot.outlineChapters || [])[0];
    if (chapter) targetId = chapter.nodeId;
    else if (String(snapshot.next?.targetId || '').startsWith('chapter:')) targetId = snapshot.next.targetId;
  }

  if (stage === 'craft' && targetId) {
    const entity = entityOf(targetId);
    if (!entity) {
      const next = snapshot.next?.targetId;
      const fallback = entityOf(next || '');
      if (fallback) targetId = next;
    }
  }

  const resource = snapshot.index.get(targetId) || null;
  if (stage === 'write' && resource?.kind === 'chapter' && intent === 'recap') {
    stage = 'recap';
    targetId = `chapter:${chapterIdOf(resource.id)}`;
  }
  if (stage === 'write' && resource?.kind === 'chapter' && intent === 'review' && resource.meta?.written) {
    stage = 'review';
    targetId = `chapter:${chapterIdOf(resource.id)}`;
  }
  if (stage === 'recap' && targetId && !targetId.startsWith('chapter:')) {
    const chapterId = chapterIdOf(targetId) || snapshot.next?.chapterId || '';
    if (chapterId) targetId = `chapter:${chapterId}`;
  }
  if (stage === 'write' && resource?.kind === 'recap') {
    stage = 'recap';
    targetId = `chapter:${resource.id.slice('recap:'.length)}`;
  }

  return { stage, targetId, intent, resource };
}

function targetDescription(snapshot, target) {
  const { stage, targetId, resource } = target;
  if (stage === 'setup') {
    return {
      id: 'setup',
      title: snapshot.setup.project_name || snapshot.project.name,
      existing: Boolean(snapshot.setup.project_name || snapshot.setup.premise)
    };
  }
  if (stage === 'write' || stage === 'recap' || stage === 'review') {
    const chapterId = chapterIdOf(targetId) || snapshot.next?.chapterId || '';
    const chapter = snapshot.outlineChapterById.get(chapterId);
    return {
      id: chapterId,
      nodeId: `chapter:${chapterId}`,
      title: chapter?.title || resource?.title || chapterId,
      volumeId: chapter?.volumeId || '',
      written: Boolean(chapter?.written),
      words: chapter?.words || 0,
      characters: Array.isArray(chapter?.characters) ? chapter.characters : [],
      recap: snapshot.recapFiles.has(chapterId)
    };
  }
  if (stage === 'craft') {
    const entity = entityOf(targetId) || entityOf(snapshot.next?.targetId || '');
    if (!entity) return null;
    const fileKey = {
      character: 'characters',
      item: 'items',
      location: 'locations',
      organization: 'organizations'
    }[entity.kind];
    const existing = snapshot.craft[fileKey]?.[entity.name];
    return {
      kind: entity.kind,
      name: entity.name,
      id: `${entity.kind}:${entity.name}`,
      existing: existing || null
    };
  }
  if (stage === 'outline') {
    let volumeId = volumeIdOf(targetId);
    const chapterId = chapterIdOf(targetId);
    const chapter = chapterId ? snapshot.outlineChapterById.get(chapterId) : null;
    if (!volumeId && String(targetId).startsWith('part:')) {
      const partId = String(targetId).slice('part:'.length);
      const part = (snapshot.outline.parts || []).find((item) => item.id === partId);
      const partVolumes = Array.isArray(part && part.volumes) ? part.volumes : [];
      const chosen = partVolumes.find((volume) => !(Array.isArray(volume.chapters) && volume.chapters.length)) || partVolumes[0];
      if (chosen && chosen.id) volumeId = chosen.id;
    }
    const volume = snapshot.volumes.find((item) => item.id === (volumeId || chapter?.volumeId || '')) || null;
    return {
      volumeId: volume?.id || chapter?.volumeId || '',
      globalIndex: volume?.globalIndex || 0,
      chapterId: chapter?.id || '',
      chapterTitle: chapter?.title || '',
      chapterCount: volume?.chapterCount ?? (chapter ? 1 : 0),
      volumeTitle: volume?.title || '',
      existing: Boolean(volume || chapter)
    };
  }
  return null;
}

function contextPlan(stage, target, snapshot, config) {
  if (stage === 'setup') {
    return {
      query: ['story-setup', '--view', 'brief'],
      label: 'story_setup brief',
      optional: true
    };
  }
  if (stage === 'craft') {
    const type = CRAFT_TYPES[target.kind] || 'craft-character';
    return {
      query: ['context', '--type', type, '--name', target.name],
      label: `${type} context for ${target.name}`,
      optional: false
    };
  }
  if (stage === 'write') {
    const type = target.written ? 'chapter-repair' : 'chapter-write';
    const query = ['context', '--type', type, '--id', target.id];
    if (!target.written) {
      query.push('--fields', 'path,navigation,narrative_memory,stats,warnings', '--view', 'brief');
    }
    return { query, label: `${type} context for ${target.id}`, optional: false };
  }
  if (stage === 'review') {
    return {
      query: ['context', '--type', 'chapter-repair', '--id', target.id],
      label: `chapter-repair context for ${target.id}`,
      optional: true
    };
  }
  if (stage === 'recap') {
    return {
      query: ['context', '--type', 'recap-repair', '--id', target.id],
      label: `recap-repair context for ${target.id}`,
      optional: true
    };
  }
  if (stage === 'outline') {
    const volumeId = target.volumeId;
    if (volumeId) {
      return {
        query: ['context', '--type', 'outline-volume', '--id', volumeId],
        label: `outline-volume context for ${volumeId}`,
        optional: false
      };
    }
    return { query: ['outline', '--view', 'index'], label: 'outline index', optional: true };
  }
  return { query: null, label: '', optional: true };
}

function outputSchema(stage, target) {
  if (stage === 'setup') {
    return {
      type: 'object',
      description:
        'A Novelgen story-setup artifact. Use real setup fields only: project_name, genres, premise, theme, rules, target_audience, tone, tense, pov_style, writing_style, long_form_plan, core_cast, storylines, premises. At least one core_cast item must use role="protagonist" exactly; descriptive roles may be used for other cast members.',
      additionalProperties: true
    };
  }
  if (stage === 'craft') {
    return {
      type: 'object',
      description:
        `A single ${target.kind} craft object. Do not wrap it in "craft", "${target.kind}", or "payload". Use the typed craft fields (name, background, personality, motivation, skills, abilities, voice, notes, role_in_story, rpg_role, rpg_stats, ...).`,
      additionalProperties: true
    };
  }
  if (stage === 'write') {
    return {
      type: 'object',
      properties: {
        content: { type: 'string', description: 'Complete chapter prose. If a heading is included, use exactly one top-level # heading.' }
      },
      required: ['content'],
      additionalProperties: false
    };
  }
  if (stage === 'review') {
    return {
      type: 'object',
      description:
        'A read-only chapter review. Do not rewrite the chapter. Return score (0-100), summary, strengths, weaknesses, and concrete suggestions.',
      properties: {
        score: { type: 'number' },
        summary: { type: 'string' },
        strengths: { type: 'array', items: { type: 'string' } },
        weaknesses: { type: 'array', items: { type: 'string' } },
        suggestions: { type: 'array', items: { type: 'string' } }
      },
      required: ['summary'],
      additionalProperties: true
    };
  }
  if (stage === 'recap') {
    return {
      type: 'object',
      description:
        'A typed Novelgen recap. Keep plot_beats <= 8, decisions <= 5, reveals <= 6, unresolved <= 5, promises <= 4, items/status <= 6. last_line must be the chapter last sentence; next_opening_hint must continue it with a concrete noun/character/place.',
      properties: {
        location: { type: 'string' },
        present: { type: 'array', items: { type: 'string' } },
        time: { type: 'string' },
        plot_beats: { type: 'array', items: { type: 'string' } },
        decisions: { type: 'array', items: { type: 'string' } },
        reveals: { type: 'array', items: { type: 'string' } },
        unresolved: { type: 'array', items: { type: 'string' } },
        promises: { type: 'array', items: { type: 'string' } },
        items: { type: 'array' },
        status: { type: 'array' },
        last_line: { type: 'string' },
        next_opening_hint: { type: 'string' }
      },
      required: ['location', 'present', 'last_line', 'next_opening_hint'],
      additionalProperties: true
    };
  }
  if (stage === 'outline') {
    return {
      type: 'object',
      properties: {
        summary: { type: 'string', description: 'Optional new volume summary.' },
        changed_chapters: {
          type: 'array',
          items: {
            type: 'object',
            properties: {
              id: { type: 'string' },
              title: { type: 'string' },
              summary: { type: 'string' },
              characters: { type: 'array', items: { type: 'string' } },
              location: { type: 'string' },
              events: { type: 'array' },
              conflict: { type: 'string' },
              pacing: { type: 'string' },
              timeline: { type: 'object' },
              state_anchor: { type: 'object' },
              scenes: { type: 'array' },
              mysteries: { type: 'object' },
              storyline_advances: { type: 'array' },
              chapter_payoff: { type: 'object' }
            },
            required: ['id'],
            additionalProperties: false
          }
        }
      },
      required: ['changed_chapters'],
      additionalProperties: false
    };
  }
  return { type: 'object', additionalProperties: true };
}

function buildTaskMarkdown({ stage, target, snapshot, skill, schema, contextFile, taskFile, args }) {
  const project = snapshot.project;
  const lines = [];
  lines.push('# Novelgen 创作任务包');
  lines.push('');
  lines.push('## 项目');
  lines.push(`- 名称: ${project.name}`);
  lines.push(`- 根目录: \`${project.root}\``);
  lines.push(`- 语言: ${project.language || 'zh'}`);
  lines.push(`- 阶段: ${stage}`);
  lines.push(`- 任务: ${taskTitle(stage, target)}`);
  lines.push('');
  lines.push('## 技能');
  lines.push(`必须遵循 Novelgen 技能 \`${skill}\`。完整技能正文：`);
  lines.push('');
  lines.push('```powershell');
  lines.push(`"${snapshot.exe || 'novelgen'}" tool skill read ${skill} --dir "${project.root}"`);
  lines.push('```');
  lines.push('');
  lines.push('## 上下文');
  lines.push(`完整上下文保存在：\`${contextFile}\``);
  if (stage === 'setup' && (args.idea || args.note)) {
    lines.push('');
    lines.push('用户创作意图：');
    lines.push('');
    lines.push(truncate(String(args.idea || args.note), 4000));
  }
  lines.push('');
  lines.push('## 输出');
  lines.push('最终只输出一个 JSON 对象，必须匹配下面的 schema。不要输出 Markdown、解释、代码围栏、命令记录或额外字段。');
  lines.push('');
  lines.push('```json');
  lines.push(JSON.stringify(schema, null, 2));
  lines.push('```');
  lines.push('');
  lines.push('## 硬性约束');
  lines.push('- 不要直接写项目文件。');
  if (stage === 'review') {
    lines.push('- 这是只读审校任务：不要修改正文、不要运行 patch；只返回审校 JSON。');
  } else {
    lines.push('- 不要运行 `tool patch`、`tool check`、`tool refresh` 或 `patch-buffer`；落盘由主代理用 `novelgen_apply` 完成。');
  }
  lines.push('- 不要浏览 Novelgen 源码，不要使用 `git`，不要修改依赖。');
  lines.push('- 只处理本任务包指定的目标；不要顺手改其他章节、角色或设定。');
  if (stage === 'write') {
    lines.push(`- 目标章节: ${target.title} (\`${target.id}\`)。`);
    lines.push(`- 目标篇幅: ${args.targetWords || 3000} 字左右；硬上限 ${args.maxWords || Math.round((args.targetWords || 3000) * 1.35)} 字，超过会被要求压缩。`);
    if (Array.isArray(target.characters) && target.characters.length) {
      lines.push(`- 必须出现的大纲角色全名: ${target.characters.join('、')}。正文里每个名字至少出现一次，不要只写简称。`);
    }
  }
  if (stage === 'review') {
    lines.push(`- 目标章节: ${target.title} (\`${target.id}\`)。`);
    lines.push('- 只评价已保存的正文，不要提出需要改项目结构才能执行的建议。');
  }
  if (stage === 'outline') {
    lines.push('- 只返回目标卷的 `changed_chapters`；不要新增章节、不要改其他卷。');
  }
  if (stage === 'recap') {
    lines.push(`- 目标章节: ${target.title} (\`${target.id}\`)。`);
  }
  if (stage === 'setup') {
    lines.push('- `core_cast` 至少一个条目的 `role` 必须精确为 `"protagonist"`；不要用描述性角色名替代。');
    lines.push('- 角色姓名必须优先从 context 的 `name_cards` 中选择，不要自创“林X/苏X”式通用名；至少比较 5 个候选再定。');
    lines.push('- 替身名遵循 JOJO 传统，用音乐/乐队/歌曲 reference；不要复用其他项目的“系统日志”题材或同名金手指。');
    lines.push('- 每条 `storylines` 至少包含 name/description；重要故事线需要压力或兑现字段，否则 setup check 会报 medium 问题。');
    lines.push('- 完整 story setup 会通过 `storage replace` 创建；不要再把 setup 当成 partial patch。');
  }
  lines.push('');
  lines.push('## 任务文件');
  lines.push(`\`${taskFile}\``);
  return lines.join('\n');
}

function taskTitle(stage, target) {
  if (stage === 'setup') return '生成或改进故事设定';
  if (stage === 'craft') return `${target.kind === 'character' ? '角色' : '设定元素'} craft：${target.name || target.id}`;
  if (stage === 'write') return `${target.written ? '改进' : '撰写'}章节：${target.id} ${target.title || ''}`.trim();
  if (stage === 'review') return `审校章节：${target.id} ${target.title || ''}`.trim();
  if (stage === 'recap') return `抽取章节 recap：${target.id}`;
  if (stage === 'outline') return `改进大纲卷：${target.volumeId || target.id}`;
  return stage;
}

function targetWords(snapshot) {
  const value = Number(snapshot.doc?.chapter_config?.target_words_per_chapter);
  return Number.isFinite(value) && value > 0 ? value : 3000;
}

function maxWords(snapshot) {
  const configured = Number(snapshot.doc?.chapter_config?.max_words_per_chapter);
  if (Number.isFinite(configured) && configured > 0) return configured;
  return Math.round(targetWords(snapshot) * 1.35);
}

function applyArguments(stage, target) {
  if (stage === 'setup') {
    return { stage: 'setup', payload: '<REPLACE_WITH_THE_SUBAGENT_JSON_OBJECT>' };
  }
  if (stage === 'craft') {
    return {
      stage: 'craft',
      target: CRAFT_TARGET[target.kind],
      id: target.name,
      payload: '<REPLACE_WITH_THE_SUBAGENT_JSON_OBJECT>'
    };
  }
  if (stage === 'write') {
    return {
      stage: 'chapter',
      id: target.id,
      payload: '<REPLACE_WITH_THE_SUBAGENT_JSON_OBJECT>'
    };
  }
  if (stage === 'review') {
    return null;
  }
  if (stage === 'recap') {
    return {
      stage: 'recap',
      id: target.id,
      payload: '<REPLACE_WITH_THE_SUBAGENT_JSON_OBJECT>'
    };
  }
  if (stage === 'outline') {
    return {
      stage: 'outline',
      target: target.chapterId ? 'chapter' : 'volume',
      id: target.chapterId || target.volumeId,
      payload: '<REPLACE_WITH_THE_SUBAGENT_JSON_OBJECT>'
    };
  }
  return null;
}

function checkCommand(stage, target) {
  if (stage === 'setup') return 'novelgen tool check all --target setup';
  if (stage === 'craft') {
    return `novelgen tool check schema --target craft --scope ${target.kind} --id "${target.name}"`;
  }
  if (stage === 'write' || stage === 'review') {
    return `novelgen tool check all --target chapter --id "${target.id}"`;
  }
  if (stage === 'recap') {
    return `novelgen tool check all --target recap --id "${target.id}"`;
  }
  if (stage === 'outline') {
    return target.chapterId
      ? `novelgen tool check all --target outline --scope chapter --id "${target.chapterId}"`
      : `novelgen tool check all --target outline --scope volume --id "${target.volumeId}"`;
  }
  return '';
}

function directCompose(snapshot, target) {
  const root = snapshot.project.root;
  const exe = snapshot.exe || 'novelgen';
  const volumeIndex = target.globalIndex || 1;
  const command = `"${exe}" compose pipeline --from-volume ${volumeIndex} --to-volume ${volumeIndex} --dir "${root}"`;
  return {
    ok: true,
    mode: 'direct',
    stage: 'outline',
    task: target.volumeId ? 'compose-volume' : 'compose-skeleton',
    targetId: target.volumeId ? `volume:${target.volumeId}` : '',
    projectDir: root,
    command,
    note:
      '章节 ID 分配和结构校验由 Novelgen Go 管线完成。把这条命令作为后台任务运行；完成后调用 novelgen_status 查看进度。',
    delegate: null,
    apply: null,
    check: target.volumeId
      ? `novelgen tool check all --target outline --scope volume --id "${target.volumeId}"`
      : 'novelgen tool check all --target outline'
  };
}

export async function prepareTask({ snapshot, config, args = {} }) {
  const target = inferTarget(snapshot, args);
  const description = targetDescription(snapshot, target);
  if (!description) {
    return {
      ok: false,
      error: `Cannot infer a novelgen target from stage=${target.stage || '(empty)'} targetId=${target.targetId || '(empty)'}.`
    };
  }

  if (target.stage === 'outline' && (!description.existing || description.chapterCount === 0)) {
    return directCompose(snapshot, description);
  }

  if (target.stage === 'review' && !description.written) {
    return {
      ok: false,
      mode: 'guidance',
      stage: 'review',
      task: 'review',
      projectDir: snapshot.project.root,
      targetId: description.nodeId || '',
      error: '章节还没有正文，先写正文再做审校。',
      delegate: null,
      apply: null
    };
  }

  const descriptionTarget = description;
  const plan = contextPlan(target.stage, descriptionTarget, snapshot, config);
  let context = null;
  let contextError = null;
  if (plan.query) {
    const result = await toolQuery(snapshot.project.root, plan.query, { config });
    if (result.ok) {
      context = result.json;
    } else {
      contextError = result.error || result.stderr || 'context query failed';
      context = { ok: false, error: contextError, query: plan.query };
    }
  }

  if (target.stage === 'setup') {
    const naming = await toolQuery(
      snapshot.project.root,
      ['ideas', '--type', 'namecard', '--genre', '都市', '--count', '10', '--seed', '42'],
      { config }
    );
    context = {
      ...(context && typeof context === 'object' ? context : {}),
      name_cards: naming.ok ? naming.json : { ok: false, error: naming.error || 'namecard query failed' },
      setup_shape: {
        project_name: '<book title>',
        genres: ['<genre>'],
        premise: '<one paragraph>',
        theme: '<theme>',
        rules: ['<world rule>'],
        target_audience: '<audience>',
        tone: '<tone>',
        tense: 'past',
        pov_style: '<pov>',
        writing_style: { description: '<style>' },
        long_form_plan: {
          target_chapters: '<number>',
          target_volumes: '<number>',
          main_loop: '<loop>',
          escalation_ladder: ['<step>'],
          reader_promises: ['<promise>'],
          payoff_cadence: '<cadence>',
          volume_pattern: '<pattern>'
        },
        core_cast: [
          { name: '<from name_cards>', role: 'protagonist', importance: 10, story_function: '<function>', entry_phase: '<phase>' }
        ],
        storylines: [
          { name: '<name>', description: '<description>', scope: 'book', setup_role: '<role>', payoff_style: '<style>', repeatable_pressure: '<pressure>', payoff_cadence: '<cadence>' }
        ],
        premises: [{ name: '<name>', description: '<description>' }]
      }
    };
    if (!naming.ok && !contextError) contextError = naming.error || 'namecard query failed';
  }

  if (target.stage === 'recap' && (!context || context.ok === false)) {
    const file = snapshot.chapterFiles.get(descriptionTarget.id);
    let text = '';
    if (file) {
      try {
        text = fs.readFileSync(file, 'utf8');
      } catch {
        text = '';
      }
    }
    const chapter = snapshot.outlineChapterById.get(descriptionTarget.id);
    context = {
      ok: Boolean(text),
      chapter_id: descriptionTarget.id,
      chapter: chapter ? { title: chapter.title, summary: chapter.summary, location: chapter.location } : null,
      content: truncate(text, 16000),
      note: 'recap-repair context was unavailable; use the chapter content and outline facts above.'
    };
  }

  if (target.stage === 'review' && (!context || context.ok === false)) {
    const file = snapshot.chapterFiles.get(descriptionTarget.id);
    let text = '';
    if (file) {
      try {
        text = fs.readFileSync(file, 'utf8');
      } catch {
        text = '';
      }
    }
    const chapter = snapshot.outlineChapterById.get(descriptionTarget.id);
    context = {
      ok: Boolean(text),
      chapter_id: descriptionTarget.id,
      chapter: chapter ? { title: chapter.title, summary: chapter.summary, location: chapter.location } : null,
      content: truncate(text, 20000),
      note: 'chapter-repair context was unavailable; review the saved chapter content above.'
    };
  }

  const skill =
    target.stage === 'setup'
      ? description.existing
        ? 'setup-improve-workflow'
        : 'setup-generate-workflow'
      : target.stage === 'craft'
        ? CRAFT_SKILL[description.kind]
        : target.stage === 'write'
          ? description.written
            ? 'write-improve-workflow'
            : 'write-chapter-workflow'
          : target.stage === 'review'
            ? 'write-review-workflow'
            : target.stage === 'recap'
              ? 'recap-extract-workflow'
              : 'outline-improve-volume-workflow';

  const schema = outputSchema(target.stage, description);
  const workspace = snapshot.workspace || snapshot.project.root;
  const taskRoot = path.join(workspace, '.dsh-novelgen', 'tasks');
  cleanupTaskRoot(taskRoot);
  const taskDir = path.join(
    taskRoot,
    `${timestamp()}-${slugify(`${target.stage}-${description.id || description.volumeId || ''}`)}`
  );
  let taskFile = '';
  let contextFile = '';
  const warnings = contextError ? [`context query failed: ${contextError}`] : [];
  let contextJson = '';
  try {
    contextJson = JSON.stringify(context, null, 2);
  } catch {
    contextJson = '{"ok":false,"error":"context is not serializable"}';
  }

  try {
    fs.mkdirSync(taskDir, { recursive: true });
    contextFile = path.join(taskDir, 'context.json');
    taskFile = path.join(taskDir, 'task.md');
    fs.writeFileSync(contextFile, contextJson, 'utf8');
  } catch (error) {
    warnings.push(`task packet files could not be written: ${error instanceof Error ? error.message : String(error)}`);
    taskFile = '';
    contextFile = '';
    try {
      const fallbackDir = path.join(os.tmpdir(), 'dsh-novelgen', 'tasks', path.basename(taskDir));
      fs.mkdirSync(fallbackDir, { recursive: true });
      const fallbackFile = path.join(fallbackDir, 'task.md');
      const fallbackContext = path.join(fallbackDir, 'context.json');
      fs.writeFileSync(fallbackContext, contextJson, 'utf8');
      fs.writeFileSync(fallbackFile, 'pending', 'utf8');
      taskFile = fallbackFile;
      contextFile = fallbackContext;
    } catch (fallbackError) {
      warnings.push(
        `temp task packet also failed: ${fallbackError instanceof Error ? fallbackError.message : String(fallbackError)}`
      );
    }
  }

  let markdown = buildTaskMarkdown({
    stage: target.stage,
    target: description,
    snapshot,
    skill,
    schema,
    contextFile: contextFile || '(inline below)',
    taskFile: taskFile || '(inline below)',
    args: { ...args, targetWords: targetWords(snapshot), maxWords: maxWords(snapshot) }
  });
  if (taskFile) {
    markdown = markdown.replace(
      '## 任务文件',
      `## 上下文文件\n\`${contextFile}\`\n\n## 任务文件`
    );
    try {
      fs.writeFileSync(taskFile, markdown, 'utf8');
    } catch (error) {
      warnings.push(`task file write failed: ${error instanceof Error ? error.message : String(error)}`);
      taskFile = '';
      contextFile = '';
    }
  }
  if (!taskFile) {
    markdown += `\n\n## 上下文数据\n\n\`\`\`json\n${truncate(contextJson, 40000)}\n\`\`\`\n`;
  }

  const reviewOnly = target.stage === 'review';
  const prompt = taskFile
    ? [
        '你是 Novelgen 创作子代理，只负责一个具体创作任务。',
        `任务包：${taskFile}`,
        `项目根目录：${snapshot.project.root}`,
        `任务：${taskTitle(target.stage, description)}`,
        '',
        '先读取任务包，严格按任务包里的技能、上下文和输出 schema 工作。',
        '最终只输出一个 JSON 对象；不要输出 Markdown、解释、代码围栏、命令记录或额外字段。',
        reviewOnly
          ? '这是只读审校任务：不要修改项目，不要调用 novelgen_apply。'
          : '不要直接写项目文件，不要运行 tool patch/check/refresh；落盘由主代理完成。'
      ].join('\n')
    : [
        '你是 Novelgen 创作子代理，只负责一个具体创作任务。',
        `项目根目录：${snapshot.project.root}`,
        `任务：${taskTitle(target.stage, description)}`,
        '',
        '任务包内联在下面。严格按其中的技能、上下文和输出 schema 工作。',
        '最终只输出一个 JSON 对象；不要输出 Markdown、解释、代码围栏、命令记录或额外字段。',
        reviewOnly
          ? '这是只读审校任务：不要修改项目，不要调用 novelgen_apply。'
          : '不要直接写项目文件，不要运行 tool patch/check/refresh；落盘由主代理完成。',
        '',
        markdown
      ].join('\n');

  return {
    ok: true,
    mode: 'subagent',
    stage: target.stage,
    task: skill.replace(/-workflow$/, ''),
    targetId:
      description.nodeId ||
      description.id ||
      (description.name ? `${description.kind}:${description.name}` : '') ||
      (description.volumeId ? `volume:${description.volumeId}` : ''),
    projectDir: snapshot.project.root,
    target: description,
    skill,
    taskFile,
    contextFile,
    contextSummary: summarizeContext(context),
    delegate: {
      description: `Novelgen ${target.stage}: ${description.name || description.id || description.volumeId || ''}`.slice(0, 60),
      prompt
    },
    apply: applyArguments(target.stage, description),
    check: checkCommand(target.stage, description),
    warnings
  };
}

function summarizeContext(context) {
  if (!context || typeof context !== 'object') return { bytes: 0 };
  const text = JSON.stringify(context);
  return {
    bytes: text.length,
    keys: Object.keys(context).slice(0, 20),
    ok: context.ok
  };
}

export function buildSelectionPrompt(snapshot, ids, intent = 'auto', note = '') {
  const unique = [...new Set((Array.isArray(ids) ? ids : []).map((id) => String(id).trim()).filter(Boolean))].slice(0, 20);
  const resources = unique.map((id) => snapshot.index.get(id)).filter(Boolean);
  const intentText = INTENTS[intent] || INTENTS.auto;
  const lines = [];
  lines.push('# Novelgen 创作请求');
  lines.push('');
  lines.push(`项目：《${snapshot.project.name}》`);
  lines.push(`项目目录：\`${snapshot.project.root}\``);
  lines.push(`当前进度：正文 ${snapshot.counts.written}/${snapshot.counts.chapters} 章 · 设定 ${snapshot.counts.setup ? '有' : '无'} · 大纲 ${snapshot.counts.volumes} 卷`);
  lines.push('');
  if (resources.length) {
    lines.push('已选资源：');
    for (const resource of resources) {
      const detail = [];
      if (resource.kind === 'chapter') {
        detail.push(resource.meta?.written ? `已写 ${resource.words || 0} 字` : '未写');
        if (resource.meta?.volumeId) detail.push(resource.meta.volumeId);
      } else if (resource.kind === 'volume') {
        detail.push(`${resource.meta?.chapters || 0} 章`);
      } else if (resource.status === 'missing') {
        detail.push('尚缺');
      }
      lines.push(
        `- [${resource.kind}] ${resource.title} (\`${resource.id}\`)${resource.path ? ` · ${resource.path}` : ''}${detail.length ? ` · ${detail.join(' · ')}` : ''}`
      );
    }
    lines.push('');
  }
  lines.push(`意图：${intentText}`);
  if (note && String(note).trim()) {
    lines.push('');
    lines.push(`补充要求：${String(note).trim().slice(0, 2000)}`);
  }
  lines.push('');
  lines.push('执行方式：');
  lines.push('1. 先调用 `novelgen_status`（projectDir 使用上面的项目目录）确认状态。');
  if (intent === 'ask') {
    lines.push('2. 这是讨论请求：只做分析、比较和建议，不要修改项目，也不要调用 `novelgen_apply`。');
    lines.push('3. 如果分析后需要落盘修改，先向用户确认，再调用 `novelgen_prepare`、`subagent` 和 `novelgen_apply`。');
  } else if (intent === 'review') {
    lines.push('2. 这是只读审校请求：调用 `novelgen_prepare`（intent 使用 `review`），把 `delegate.prompt` 交给 `subagent`，收集审校 JSON。');
    lines.push('3. 审校结果只汇报，不要调用 `novelgen_apply`；需要修改时先向用户确认，再单独发起改进任务。');
  } else {
    lines.push('2. 需要具体创作时调用 `novelgen_prepare`，把返回的 `delegate.prompt` 原样交给 `subagent`。');
    lines.push('3. 子代理返回 JSON 后调用 `novelgen_apply` 落盘；落盘成功再继续下一步。');
  }
  lines.push('4. 如果这是长程目标，用 `create_goal` 或 `schedule_create` 记录目标，按 status → prepare → subagent → apply 循环推进。');
  return lines.join('\n');
}
