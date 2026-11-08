import fs from 'node:fs';
import path from 'node:path';

/**
 * Read-only projection of a Novelgen project.
 *
 * The projection deliberately stays outside Novelgen's Go modules: it reads the
 * documented project layout only, never writes it, and every mutation still
 * goes through `novelgen tool patch --apply`.
 */

const SNAPSHOT_CACHE = new Map();
const CHAPTER_CHAR_CACHE = new Map();
const CACHE_LIMIT = 24;

export function readJsonFile(file) {
  try {
    const text = fs.readFileSync(file, 'utf8');
    if (!text.trim()) return null;
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export function isValidProject(dir) {
  if (!dir) return false;
  try {
    return fs.statSync(path.join(dir, 'novel.json')).isFile();
  } catch {
    return false;
  }
}

export function findProjectRoot(start) {
  if (!start) return null;
  let dir;
  try {
    dir = path.resolve(start);
  } catch {
    return null;
  }
  for (let depth = 0; depth < 12; depth += 1) {
    if (isValidProject(dir)) return dir;
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
}

function statSignature(file) {
  try {
    const st = fs.statSync(file);
    return `${st.mtimeMs}:${st.size}`;
  } catch {
    return '-';
  }
}

function fileMtime(file) {
  try {
    return fs.statSync(file).mtimeMs;
  } catch {
    return 0;
  }
}

function isoTime(ms) {
  return ms ? new Date(ms).toISOString() : '';
}

function dirSignature(dir) {
  try {
    const st = fs.statSync(dir);
    return `${st.mtimeMs}`;
  } catch {
    return '-';
  }
}

function chapterListSignature(dir) {
  try {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    const parts = [];
    for (const entry of entries) {
      if (!entry.isFile() || !entry.name.endsWith('.md')) continue;
      const st = fs.statSync(path.join(dir, entry.name));
      parts.push(`${entry.name}:${st.mtimeMs}:${st.size}`);
    }
    return parts.join(';');
  } catch {
    return '-';
  }
}

function projectFingerprint(root) {
  const files = [
    'novel.json',
    'story/setup/story_setup.json',
    'story/compose/outline.json',
    'story/craft/characters.json',
    'story/craft/locations.json',
    'story/craft/items.json',
    'story/craft/organizations.json'
  ].map((rel) => statSignature(path.join(root, rel)));
  files.push(dirSignature(path.join(root, 'story/recaps')));
  files.push(chapterListSignature(path.join(root, 'chapters')));
  return files.join('|');
}

function countProseChars(text) {
  return text.replace(/\s+/g, '').length;
}

function readChapterHeading(file) {
  try {
    const text = fs.readFileSync(file, 'utf8');
    const match = /^#\s+(.+)$/m.exec(text);
    return match ? match[1].trim() : '';
  } catch {
    return '';
  }
}

function chapterInfo(root, id) {
  if (!id || /[\\/]/.test(id) || id.includes('..')) return null;
  const dir = path.join(root, 'chapters');
  for (const name of [`chapter-${id}.md`, `${id}.md`]) {
    const file = path.join(dir, name);
    let st;
    try {
      st = fs.statSync(file);
    } catch {
      continue;
    }
    if (!st.isFile()) continue;
    const cacheKey = `${file}:${st.mtimeMs}:${st.size}`;
    let chars = CHAPTER_CHAR_CACHE.get(cacheKey);
    if (chars === undefined) {
      try {
        chars = countProseChars(fs.readFileSync(file, 'utf8'));
      } catch {
        chars = 0;
      }
      CHAPTER_CHAR_CACHE.set(cacheKey, chars);
      if (CHAPTER_CHAR_CACHE.size > 4000) CHAPTER_CHAR_CACHE.clear();
    }
    return {
      file,
      rel: `chapters/${name}`,
      chars,
      mtimeMs: st.mtimeMs,
      heading: readChapterHeading(file)
    };
  }
  return null;
}

function asObject(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

function listCraft(file) {
  return asObject(readJsonFile(file));
}

function truncate(text, limit) {
  const value = typeof text === 'string' ? text : '';
  if (value.length <= limit) return value;
  return `${value.slice(0, Math.max(0, limit - 1)).trimEnd()}…`;
}

function firstText(...values) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return '';
}

function normalizeKind(raw) {
  const value = String(raw || '').trim().toLowerCase();
  if (value === 'character' || value === 'characters') return 'character';
  if (value === 'item' || value === 'items') return 'item';
  if (value === 'location' || value === 'locations') return 'location';
  if (value === 'organization' || value === 'organizations') return 'organization';
  return '';
}

export function buildSnapshot(root) {
  const resolvedRoot = path.resolve(root);
  const doc = asObject(readJsonFile(path.join(resolvedRoot, 'novel.json')));
  const setup = asObject(readJsonFile(path.join(resolvedRoot, 'story/setup/story_setup.json')));
  const outline = asObject(readJsonFile(path.join(resolvedRoot, 'story/compose/outline.json')));
  const craft = {
    characters: listCraft(path.join(resolvedRoot, 'story/craft/characters.json')),
    locations: listCraft(path.join(resolvedRoot, 'story/craft/locations.json')),
    items: listCraft(path.join(resolvedRoot, 'story/craft/items.json')),
    organizations: listCraft(path.join(resolvedRoot, 'story/craft/organizations.json'))
  };
  const setupMtime = fileMtime(path.join(resolvedRoot, 'story/setup/story_setup.json'));
  const outlineMtime = fileMtime(path.join(resolvedRoot, 'story/compose/outline.json'));
  const craftMtimes = {
    characters: fileMtime(path.join(resolvedRoot, 'story/craft/characters.json')),
    locations: fileMtime(path.join(resolvedRoot, 'story/craft/locations.json')),
    items: fileMtime(path.join(resolvedRoot, 'story/craft/items.json')),
    organizations: fileMtime(path.join(resolvedRoot, 'story/craft/organizations.json'))
  };

  const resources = [];
  const index = new Map();
  const edgeSet = new Set();
  const edges = [];
  const chapterFiles = new Map();

  function addResource(resource) {
    if (index.has(resource.id)) {
      return index.get(resource.id);
    }
    const node = {
      id: resource.id,
      kind: resource.kind,
      title: resource.title || resource.id,
      status: resource.status || 'done',
      summary: truncate(resource.summary || '', 1200),
      path: resource.path || '',
      meta: resource.meta || {},
      words: resource.words || 0,
      degree: 0
    };
    index.set(node.id, node);
    resources.push(node);
    return node;
  }

  function link(from, to, kind) {
    if (!from || !to || from === to) return;
    if (!index.has(from) || !index.has(to)) return;
    const key = `${from}\u0000${to}\u0000${kind}`;
    if (edgeSet.has(key)) return;
    edgeSet.add(key);
    edges.push([from, to, kind]);
    index.get(from).degree += 1;
    index.get(to).degree += 1;
  }

  function ensureEntity(kind, name, extra = {}) {
    const clean = typeof name === 'string' ? name.trim() : '';
    if (!clean || clean.length > 80) return null;
    const id = `${kind}:${clean}`;
    const existing = index.get(id);
    if (existing) {
      if (extra.status && existing.status === 'mention' && extra.status !== 'mention') {
        existing.status = extra.status;
      }
      return existing;
    }
    return addResource({
      id,
      kind,
      title: clean,
      status: extra.status || 'mention',
      summary: extra.summary || '',
      meta: extra.meta || {}
    });
  }

  // ---- setup ------------------------------------------------------------
  const setupReady = Boolean(
    setup.project_name || setup.premise || (Array.isArray(setup.core_cast) && setup.core_cast.length > 0)
  );
  addResource({
    id: 'setup',
    kind: 'setup',
    title: setup.project_name || doc.name || 'Story setup',
    status: setupReady ? 'done' : 'missing',
    summary: firstText(setup.premise, setup.theme),
    path: 'story/setup/story_setup.json',
    meta: {
      genres: Array.isArray(setup.genres) ? setup.genres : [],
      cast: Array.isArray(setup.core_cast) ? setup.core_cast.length : 0,
      storylines: Array.isArray(setup.storylines) ? setup.storylines.length : 0,
      updatedAt: isoTime(setupMtime)
    }
  });

  for (const storyline of Array.isArray(setup.storylines) ? setup.storylines : []) {
    const node = ensureEntity('storyline', storyline.name, {
      status: 'done',
      summary: firstText(storyline.description, storyline.type)
    });
    if (node) {
      node.meta = { type: storyline.type || '', importance: storyline.importance ?? null, updatedAt: isoTime(setupMtime) };
      link('setup', node.id, 'defines');
    }
  }

  const coreCast = Array.isArray(setup.core_cast) ? setup.core_cast : [];
  const coreCastNames = [];
  for (const cast of coreCast) {
    const name = typeof cast?.name === 'string' ? cast.name.trim() : '';
    if (!name) continue;
    coreCastNames.push(name);
    const crafted = Boolean(craft.characters[name]);
    const node = ensureEntity('character', name, {
      status: crafted ? 'done' : 'missing',
      summary: firstText(cast.story_function, cast.role)
    });
    if (node) {
      node.meta = {
        ...node.meta,
        importance: cast.importance ?? null,
        entry: cast.entry_phase || '',
        core: true,
        updatedAt: isoTime(craftMtimes.characters || setupMtime)
      };
      link('setup', node.id, 'casts');
    }
  }

  // ---- craft ------------------------------------------------------------
  const craftTargets = [
    ['characters', 'character'],
    ['locations', 'location'],
    ['items', 'item'],
    ['organizations', 'organization']
  ];
  const craftCounts = {};
  for (const [fileKey, kind] of craftTargets) {
    const entries = craft[fileKey] || {};
    craftCounts[kind] = Object.keys(entries).length;
    for (const [name, raw] of Object.entries(entries)) {
      const object = asObject(raw);
      const node = ensureEntity(kind, name, {
        status: 'done',
        summary: firstText(
          object.background,
          object.description,
          object.motivation,
          object.function,
          object.purpose
        )
      });
      if (node) {
        node.path = `story/craft/${fileKey}.json`;
        node.meta = {
          ...node.meta,
          role: firstText(object.role_in_story, object.rpg_role, object.type),
          power: object.power_level ?? null,
          updatedAt: isoTime(craftMtimes[fileKey])
        };
      }
    }
  }

  // ---- outline ----------------------------------------------------------
  const parts = Array.isArray(outline.parts) ? outline.parts : [];
  const volumes = [];
  const outlineChapters = [];
  const outlineChapterById = new Map();

  for (const part of parts) {
    const partId = typeof part.id === 'string' && part.id ? part.id : `P${parts.indexOf(part) + 1}`;
    const partNode = addResource({
      id: `part:${partId}`,
      kind: 'part',
      title: part.title || partId,
      status: 'done',
      summary: part.summary,
      path: 'story/compose/outline.json',
      meta: { updatedAt: isoTime(outlineMtime) }
    });
    link('setup', partNode.id, 'structures');

    for (const volume of Array.isArray(part.volumes) ? part.volumes : []) {
      const volumeId = typeof volume.id === 'string' && volume.id ? volume.id : `${partId}-V${(part.volumes || []).indexOf(volume) + 1}`;
      const volumeChapters = Array.isArray(volume.chapters) ? volume.chapters : [];
      const volumeNode = addResource({
        id: `volume:${volumeId}`,
        kind: 'volume',
        title: volume.title || volumeId,
        status: volumeChapters.length ? 'done' : 'missing',
        summary: volume.summary,
        path: 'story/compose/outline.json',
        meta: {
          partId,
          chapters: volumeChapters.length,
          payoff: volume.payoff_contract || null,
          updatedAt: isoTime(outlineMtime)
        }
      });
      link(partNode.id, volumeNode.id, 'contains');

      const volumeRecord = {
        id: volumeId,
        title: volume.title || volumeId,
        partId,
        partTitle: part.title || partId,
        globalIndex: volumes.length + 1,
        chapterCount: volumeChapters.length,
        summary: volume.summary || ''
      };
      volumes.push(volumeRecord);

      let previousChapterId = '';
      for (const chapter of volumeChapters) {
        const chapterId = typeof chapter.id === 'string' && chapter.id ? chapter.id : `${volumeId}-C${volumeChapters.indexOf(chapter) + 1}`;
        const info = chapterInfo(resolvedRoot, chapterId);
        const written = Boolean(info);
        if (info) chapterFiles.set(chapterId, info.file);
        const title = chapter.title || info?.heading || chapterId;
        const events = Array.isArray(chapter.events) ? chapter.events : [];
        const characters = Array.isArray(chapter.characters) ? chapter.characters : [];
        const chapterNode = addResource({
          id: `chapter:${chapterId}`,
          kind: 'chapter',
          title,
          status: written ? 'done' : 'missing',
          summary: chapter.summary,
          path: info?.rel || '',
          words: info?.chars || 0,
          meta: {
            volumeId,
            partId,
            order: outlineChapters.length,
            written,
            words: info?.chars || 0,
            events: events.length,
            characters: characters.length,
            location: chapter.location || '',
            updatedAt: info ? isoTime(info.mtimeMs) : isoTime(outlineMtime)
          }
        });
        link(volumeNode.id, chapterNode.id, 'contains');
        if (previousChapterId) link(previousChapterId, chapterNode.id, 'next');
        previousChapterId = chapterNode.id;

        const chapterRecord = {
          ...chapter,
          id: chapterId,
          volumeId,
          partId,
          nodeId: chapterNode.id,
          title,
          written,
          words: info?.chars || 0,
          info
        };
        outlineChapters.push(chapterRecord);
        outlineChapterById.set(chapterId, chapterRecord);

        for (const name of characters) {
          const node = ensureEntity('character', name, { status: 'mention' });
          if (node) link(chapterNode.id, node.id, 'features');
        }
        if (chapter.location) {
          const node = ensureEntity('location', chapter.location, { status: 'mention' });
          if (node) link(chapterNode.id, node.id, 'set_in');
        }
        for (const event of events) {
          const eventCharacters = Array.isArray(event.characters) ? event.characters : [];
          for (const name of eventCharacters) {
            const node = ensureEntity('character', name, { status: 'mention' });
            if (node) link(chapterNode.id, node.id, 'features');
          }
          if (event.actor) {
            const node = ensureEntity('character', event.actor, { status: 'mention' });
            if (node) link(chapterNode.id, node.id, 'features');
          }
          const targetKind = normalizeKind(event.target_type);
          if (event.target && targetKind) {
            const node = ensureEntity(targetKind, event.target, { status: 'mention' });
            if (node) link(chapterNode.id, node.id, 'uses');
          }
        }
        for (const advance of Array.isArray(chapter.storyline_advances) ? chapter.storyline_advances : []) {
          const name = advance.storyline_name || advance.name;
          if (!name) continue;
          const node = ensureEntity('storyline', name, { status: 'mention' });
          if (node) link(chapterNode.id, node.id, 'advances');
        }
      }

      volumeNode.meta.crafted = volumeChapters.filter((chapter) => {
        const chapterId = chapter.id || '';
        return chapterId && chapterFiles.has(chapterId);
      }).length;
    }
  }

  // ---- recaps -----------------------------------------------------------
  const recapFiles = new Map();
  try {
    for (const entry of fs.readdirSync(path.join(resolvedRoot, 'story/recaps'), { withFileTypes: true })) {
      if (!entry.isFile() || !entry.name.endsWith('.json')) continue;
      recapFiles.set(entry.name.slice(0, -5), path.join(resolvedRoot, 'story/recaps', entry.name));
    }
  } catch {
    // no recap directory yet
  }
  for (const chapter of outlineChapters) {
    const recapFile = recapFiles.get(chapter.id);
    if (!recapFile) continue;
    const recap = asObject(readJsonFile(recapFile));
    const node = addResource({
      id: `recap:${chapter.id}`,
      kind: 'recap',
      title: firstText(recap.title, chapter.title),
      status: 'done',
      summary: firstText(recap.last_line, recap.next_opening_hint, recap.summary),
      path: `story/recaps/${chapter.id}.json`,
      meta: {
        chapterId: chapter.id,
        location: recap.location || '',
        present: Array.isArray(recap.present) ? recap.present.length : 0,
        updatedAt: isoTime(fileMtime(recapFile))
      }
    });
    link(chapter.nodeId, node.id, 'recaps');
    node.meta.chapterNode = chapter.nodeId;
  }

  // ---- counts and stages ------------------------------------------------
  const chaptersWithRecap = outlineChapters.filter((chapter) => recapFiles.has(chapter.id)).length;
  const writtenChapters = outlineChapters.filter((chapter) => chapter.written);
  const writtenCount = writtenChapters.length;
  const totalWords = writtenChapters.reduce((sum, chapter) => sum + chapter.words, 0);
  const emptyVolumes = volumes.filter((volume) => volume.chapterCount === 0);
  const missingCast = coreCastNames.filter((name) => !craft.characters[name] && !craft.characters[String(name).trim()]);
  const skeletonReady = parts.length > 0 && volumes.length > 0;
  const outlineComplete = skeletonReady && emptyVolumes.length === 0;
  const firstUnwritten = outlineChapters.find((chapter) => !chapter.written) || null;

  const targetChapters = Number(doc?.structure?.target_chapters) || outlineChapters.length;
  const targetVolumes = Number(doc?.structure?.target_volumes) || volumes.length;

  const stages = [
    {
      key: 'setup',
      current: setupReady ? 1 : 0,
      total: 1,
      status: setupReady ? 'done' : 'pending',
      detail: setupReady ? 'story_setup.json' : ''
    },
    {
      key: 'outline',
      current: skeletonReady ? volumes.filter((volume) => volume.chapterCount > 0).length : 0,
      total: skeletonReady ? volumes.length : targetVolumes,
      status: !skeletonReady ? 'pending' : emptyVolumes.length ? 'active' : 'done',
      detail: `${parts.length} parts / ${volumes.length} volumes / ${outlineChapters.length} chapters`
    },
    {
      key: 'craft',
      current: coreCastNames.length ? coreCastNames.length - missingCast.length : Object.keys(craft.characters).length,
      total: coreCastNames.length || Object.keys(craft.characters).length,
      status: missingCast.length ? 'active' : coreCastNames.length || Object.keys(craft.characters).length ? 'done' : 'pending',
      detail: missingCast.length ? `missing: ${missingCast.slice(0, 6).join(', ')}` : ''
    },
    {
      key: 'write',
      current: writtenCount,
      total: outlineChapters.length || targetChapters,
      status: !outlineChapters.length ? 'pending' : writtenCount === outlineChapters.length ? 'done' : writtenCount ? 'active' : 'pending',
      detail: `${totalWords.toLocaleString()} chars`
    }
  ];

  let next = null;
  if (!setupReady) {
    next = {
      stage: 'setup',
      task: 'generate',
      targetId: 'setup',
      reason: '故事设定尚未生成。',
      prepareArguments: { stage: 'setup', intent: 'generate' }
    };
  } else if (!skeletonReady) {
    next = {
      stage: 'outline',
      task: 'compose-skeleton',
      targetId: '',
      reason: '大纲骨架尚未生成。',
      direct: true,
      prepareArguments: { stage: 'outline', intent: 'compose-skeleton' }
    };
  } else if (emptyVolumes.length) {
    const volume = emptyVolumes[0];
    next = {
      stage: 'outline',
      task: 'compose-volume',
      targetId: `volume:${volume.id}`,
      reason: `卷 ${volume.id} 还没有章节。`,
      direct: true,
      volumeId: volume.id,
      volumeIndex: volume.globalIndex,
      prepareArguments: { stage: 'outline', targetId: `volume:${volume.id}`, intent: 'compose-volume' }
    };
  } else if (missingCast.length) {
    const name = missingCast[0];
    next = {
      stage: 'craft',
      task: 'craft-character',
      targetId: `character:${name}`,
      reason: `核心角色 ${name} 缺少 craft 设定。`,
      prepareArguments: { stage: 'craft', targetId: `character:${name}`, intent: 'create' }
    };
  } else if (firstUnwritten) {
    next = {
      stage: 'write',
      task: 'write-chapter',
      targetId: firstUnwritten.nodeId,
      reason: `章节 ${firstUnwritten.id} 尚未写正文。`,
      chapterId: firstUnwritten.id,
      prepareArguments: { stage: 'write', targetId: firstUnwritten.nodeId, intent: 'write' }
    };
  } else {
    next = {
      stage: 'review',
      task: 'review',
      targetId: '',
      reason: outlineComplete ? '大纲和正文已完成，可以进入审校。' : '可以进入下一轮审校。',
      prepareArguments: { stage: 'review', intent: 'review' }
    };
  }

  return {
    root: resolvedRoot,
    doc,
    setup,
    outline,
    craft,
    resources,
    index,
    edges,
    outlineChapters,
    outlineChapterById,
    volumes,
    chapterFiles,
    recapFiles,
    coreCastNames,
    craftCounts,
    project: {
      root: resolvedRoot,
      name: doc.name || setup.project_name || path.basename(resolvedRoot),
      language: doc.language || '',
      updatedAt: doc.updated_at || '',
      createdAt: doc.created_at || '',
      structure: {
        parts: parts.length,
        volumes: volumes.length,
        chapters: outlineChapters.length,
        targetChapters,
        targetVolumes
      },
      words: totalWords
    },
    counts: {
      setup: setupReady ? 1 : 0,
      parts: parts.length,
      volumes: volumes.length,
      chapters: outlineChapters.length,
      written: writtenCount,
      words: totalWords,
      characters: Object.keys(craft.characters).length,
      locations: Object.keys(craft.locations).length,
      items: Object.keys(craft.items).length,
      organizations: Object.keys(craft.organizations).length,
      storylines: Array.isArray(setup.storylines) ? setup.storylines.length : 0,
      recaps: chaptersWithRecap,
      coreCast: coreCastNames.length,
      missingCast: missingCast.length
    },
    stages,
    next,
    generatedAt: new Date().toISOString()
  };
}

export function snapshotFor(root) {
  const resolved = path.resolve(root);
  const fingerprint = projectFingerprint(resolved);
  const cached = SNAPSHOT_CACHE.get(resolved);
  if (cached && cached.fingerprint === fingerprint) {
    return cached.snapshot;
  }
  const snapshot = buildSnapshot(resolved);
  SNAPSHOT_CACHE.set(resolved, { fingerprint, snapshot });
  if (SNAPSHOT_CACHE.size > CACHE_LIMIT) {
    const oldest = SNAPSHOT_CACHE.keys().next().value;
    if (oldest) SNAPSHOT_CACHE.delete(oldest);
  }
  return snapshot;
}

export function publicState(snapshot) {
  return {
    project: snapshot.project,
    counts: snapshot.counts,
    stages: snapshot.stages,
    next: snapshot.next,
    resources: snapshot.resources.map((resource) => ({
      id: resource.id,
      kind: resource.kind,
      title: resource.title,
      status: resource.status,
      summary: truncate(resource.summary, 220),
      path: resource.path,
      words: resource.words,
      degree: resource.degree,
      meta: resource.meta
    })),
    edges: snapshot.edges,
    generatedAt: snapshot.generatedAt
  };
}

export function compactResources(snapshot, filters = {}) {
  const kind = String(filters.kind || '').trim();
  const query = String(filters.query || '').trim().toLowerCase();
  const status = String(filters.status || '').trim();
  const limit = Math.max(1, Math.min(500, Number(filters.limit) || 200));
  let items = snapshot.resources;
  if (kind && kind !== 'all') items = items.filter((resource) => resource.kind === kind);
  if (status) items = items.filter((resource) => resource.status === status);
  if (query) {
    items = items.filter((resource) => {
      const haystack = `${resource.id}\n${resource.title}\n${resource.summary}\n${resource.path}`.toLowerCase();
      return haystack.includes(query);
    });
  }
  return {
    total: items.length,
    resources: items.slice(0, limit).map((resource) => ({
      id: resource.id,
      kind: resource.kind,
      title: resource.title,
      status: resource.status,
      summary: truncate(resource.summary, 300),
      path: resource.path,
      words: resource.words,
      degree: resource.degree,
      meta: resource.meta
    }))
  };
}

function compactOutlineChapter(chapter) {
  const events = Array.isArray(chapter.events) ? chapter.events : [];
  return {
    id: chapter.id,
    title: chapter.title,
    summary: chapter.summary,
    characters: chapter.characters || [],
    location: chapter.location || '',
    conflict: chapter.conflict || '',
    pacing: chapter.pacing || '',
    eventCount: events.length,
    events: events.slice(0, 8).map((event) => ({
      type: event.type,
      subject: event.subject,
      actor: event.actor,
      action: event.action,
      target: event.target,
      target_type: event.target_type,
      context: event.context,
      details: truncate(event.details, 700),
      result: truncate(event.result, 400)
    })),
    chapter_payoff: chapter.chapter_payoff || null,
    state_anchor: chapter.state_anchor || null
  };
}

export function buildPreview(snapshot, id) {
  const resource = snapshot.index.get(String(id || ''));
  if (!resource) return null;
  let text = '';
  let source = '';
  let data = null;
  let format = 'json';

  if (resource.kind === 'chapter' && resource.meta?.written) {
    const chapterId = resource.id.slice('chapter:'.length);
    const chapter = snapshot.outlineChapterById.get(chapterId);
    const file = snapshot.chapterFiles.get(chapterId);
    if (file) {
      try {
        text = fs.readFileSync(file, 'utf8');
        source = resource.path;
        format = 'markdown';
      } catch {
        text = '';
      }
    }
    data = chapter
      ? {
          id: chapter.id,
          title: chapter.title,
          summary: chapter.summary,
          characters: chapter.characters || [],
          location: chapter.location || '',
          written: true,
          words: chapter.words || 0
        }
      : null;
  } else if (resource.kind === 'chapter') {
    const chapterId = resource.id.slice('chapter:'.length);
    const chapter = snapshot.outlineChapterById.get(chapterId);
    data = chapter ? compactOutlineChapter(chapter) : null;
    text = data ? JSON.stringify(data, null, 2) : '';
    source = 'story/compose/outline.json';
  } else if (resource.kind === 'setup') {
    data = snapshot.setup;
    text = JSON.stringify(snapshot.setup, null, 2);
    source = 'story/setup/story_setup.json';
  } else if (resource.kind === 'part') {
    const targetId = resource.id.slice('part:'.length);
    const part = (snapshot.outline.parts || []).find((item) => item.id === targetId);
    data = part
      ? {
          id: part.id,
          title: part.title,
          summary: part.summary,
          volumes: (part.volumes || []).map((volume) => ({
            id: volume.id,
            title: volume.title,
            summary: truncate(volume.summary, 300),
            chapterCount: Array.isArray(volume.chapters) ? volume.chapters.length : 0
          }))
        }
      : null;
    text = JSON.stringify(data, null, 2);
    source = 'story/compose/outline.json';
  } else if (resource.kind === 'volume') {
    const targetId = resource.id.slice('volume:'.length);
    const volume = (snapshot.outline.parts || [])
      .flatMap((part) => (Array.isArray(part.volumes) ? part.volumes : []))
      .find((item) => item.id === targetId);
    data = volume
      ? {
          id: volume.id,
          title: volume.title,
          summary: volume.summary,
          payoff_contract: volume.payoff_contract || null,
          chapters: (volume.chapters || []).map((chapter) => ({
            id: chapter.id,
            title: chapter.title,
            summary: truncate(chapter.summary, 240),
            characters: chapter.characters || [],
            location: chapter.location || '',
            eventCount: Array.isArray(chapter.events) ? chapter.events.length : 0,
            written: snapshot.chapterFiles.has(chapter.id)
          }))
        }
      : null;
    text = JSON.stringify(data, null, 2);
    source = 'story/compose/outline.json';
  } else if (
    resource.kind === 'character' ||
    resource.kind === 'location' ||
    resource.kind === 'item' ||
    resource.kind === 'organization'
  ) {
    const fileKey = {
      character: 'characters',
      location: 'locations',
      item: 'items',
      organization: 'organizations'
    }[resource.kind];
    const name = resource.id.slice(resource.kind.length + 1);
    data = snapshot.craft[fileKey]?.[name] || null;
    text = data ? JSON.stringify(data, null, 2) : '';
    source = `story/craft/${fileKey}.json`;
  } else if (resource.kind === 'recap') {
    const chapterId = resource.id.slice('recap:'.length);
    const file = snapshot.recapFiles.get(chapterId);
    if (file) {
      try {
        text = fs.readFileSync(file, 'utf8');
        source = `story/recaps/${chapterId}.json`;
        data = JSON.parse(text);
      } catch {
        text = '';
      }
    }
  } else if (resource.kind === 'storyline') {
    const name = resource.id.slice('storyline:'.length);
    const storyline = (snapshot.setup.storylines || []).find((item) => item.name === name);
    data = storyline || null;
    text = storyline ? JSON.stringify(storyline, null, 2) : '';
    source = 'story/setup/story_setup.json';
  }

  const limit = 12000;
  return {
    id: resource.id,
    kind: resource.kind,
    title: resource.title,
    status: resource.status,
    summary: resource.summary,
    path: resource.path,
    meta: resource.meta,
    source,
    format,
    data,
    text: truncate(text, limit),
    truncated: text.length > limit
  };
}
