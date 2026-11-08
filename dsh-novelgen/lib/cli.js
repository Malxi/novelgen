import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';

const MAX_OUTPUT_BYTES = 64 * 1024 * 1024;

export function resolveNovelgenExe(root, config = {}) {
  const candidates = [];
  const push = (value) => {
    if (typeof value === 'string' && value.trim()) candidates.push(value.trim());
  };
  push(config.novelgenPath);
  push(process.env.NOVELGEN_BIN);

  let dir = path.resolve(root || process.cwd());
  for (let depth = 0; depth < 10; depth += 1) {
    push(path.join(dir, 'bin', 'novelgen.exe'));
    push(path.join(dir, 'bin', 'novelgen'));
    push(path.join(dir, 'novelgen.exe'));
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  push(path.join(process.cwd(), 'bin', 'novelgen.exe'));
  push(path.join(process.cwd(), 'bin', 'novelgen'));

  for (const candidate of candidates) {
    try {
      if (fs.statSync(candidate).isFile()) return path.resolve(candidate);
    } catch {
      // A bare command name such as `novelgen` is resolved by the OS below.
    }
  }
  return process.platform === 'win32' ? 'novelgen.exe' : 'novelgen';
}

export function extractJson(text) {
  if (typeof text !== 'string') return null;
  const source = text.replace(/^\uFEFF/, '').replace(/\u001b\[[0-9;]*m/g, '');

  const parseAt = (start) => {
    const opening = source[start];
    const closing = opening === '{' ? '}' : ']';
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let index = start; index < source.length; index += 1) {
      const char = source[index];
      if (inString) {
        if (escaped) escaped = false;
        else if (char === '\\') escaped = true;
        else if (char === '"') inString = false;
        continue;
      }
      if (char === '"') {
        inString = true;
        continue;
      }
      if (char === opening) depth += 1;
      else if (char === closing) {
        depth -= 1;
        if (depth === 0) {
          try {
            return JSON.parse(source.slice(start, index + 1));
          } catch {
            return null;
          }
        }
      }
    }
    return null;
  };

  // Log prefixes often contain `[timestamp]`; try each bracket/brace in order
  // until one yields a complete JSON value.
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];
    if (char !== '{' && char !== '[') continue;
    const value = parseAt(index);
    if (value !== null && value !== undefined) return value;
  }
  return null;
}

function tail(text, limit = 4000) {
  const value = typeof text === 'string' ? text.trim() : '';
  return value.length > limit ? value.slice(value.length - limit) : value;
}

export function runNovelgen(root, args, options = {}) {
  const { config = {}, input, timeoutMs = 240000 } = options;
  const exe = options.exe || resolveNovelgenExe(root, config);
  return new Promise((resolve) => {
    let child;
    try {
      child = spawn(exe, args, {
        cwd: root,
        windowsHide: true,
        env: { ...process.env, NO_COLOR: '1', CLICOLOR: '0' },
        stdio: ['pipe', 'pipe', 'pipe']
      });
    } catch (error) {
      resolve({
        ok: false,
        code: null,
        stdout: '',
        stderr: '',
        json: null,
        error: `failed to spawn ${exe}: ${error instanceof Error ? error.message : String(error)}`
      });
      return;
    }

    let stdout = '';
    let stderr = '';
    let settled = false;
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      try {
        child.kill();
      } catch {
        // already gone
      }
    }, Math.max(1000, timeoutMs));

    const finish = (payload) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      const json = payload.json ?? extractJson(stdout);
      resolve({ ...payload, json, timedOut });
    };

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString('utf8');
      if (stdout.length > MAX_OUTPUT_BYTES) stdout = stdout.slice(-MAX_OUTPUT_BYTES);
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString('utf8');
      if (stderr.length > MAX_OUTPUT_BYTES) stderr = stderr.slice(-MAX_OUTPUT_BYTES);
    });
    child.on('error', (error) => {
      finish({
        ok: false,
        code: null,
        stdout,
        stderr,
        error: `failed to run ${exe}: ${error instanceof Error ? error.message : String(error)}`
      });
    });
    child.on('close', (code) => {
      const json = extractJson(stdout);
      finish({
        ok: code === 0 && (!json || json.ok !== false),
        code,
        stdout,
        stderr,
        json,
        error: code === 0 ? undefined : tail(stderr) || `novelgen exited with code ${code}`
      });
    });

    try {
      if (input === undefined) child.stdin.end();
      else child.stdin.end(String(input), 'utf8');
    } catch {
      // The child may have exited before reading stdin.
    }
  });
}

function compactIssue(issue) {
  if (!issue || typeof issue !== 'object') return issue;
  const value = {
    priority: issue.priority,
    category: issue.category,
    message: issue.message,
    description: issue.description,
    target: issue.target || issue.target_id || issue.chapter_id,
    suggestion: issue.suggestion || issue.fix || issue.recommendation
  };
  for (const key of Object.keys(value)) {
    if (value[key] === undefined || value[key] === null || value[key] === '') delete value[key];
  }
  if (typeof value.message === 'string') value.message = value.message.slice(0, 600);
  if (typeof value.description === 'string') value.description = value.description.slice(0, 600);
  return value;
}

export function compactCheck(check) {
  if (!check) return null;
  if (typeof check === 'string') return { summary: check.slice(0, 2000) };
  if (typeof check !== 'object') return check;
  const issues = Array.isArray(check.issues) ? check.issues.slice(0, 30).map(compactIssue) : undefined;
  const meta = check.meta && typeof check.meta === 'object' ? check.meta : null;
  const out = {
    ok: check.ok,
    blocking: check.blocking,
    summary: check.summary,
    warnings: Array.isArray(check.warnings) ? check.warnings.slice(0, 10) : check.warnings,
    issues,
    issueCount: Array.isArray(check.issues) ? check.issues.length : check.issue_count,
    meta: meta
      ? {
          coverage: meta.coverage,
          project_root: meta.project_root
        }
      : undefined
  };
  if (out.summary && typeof out.summary === 'string') out.summary = out.summary.slice(0, 2000);
  for (const key of Object.keys(out)) {
    if (out[key] === undefined || out[key] === null) delete out[key];
  }
  return out;
}

function compactNextAction(action) {
  if (!action || typeof action !== 'object') return action;
  const out = {
    step: action.step,
    action: action.action,
    command: action.command,
    when: action.when
  };
  for (const key of Object.keys(out)) {
    if (out[key] === undefined || out[key] === null) delete out[key];
  }
  return out;
}

export function compactPatchResult(result) {
  if (!result) return null;
  if (typeof result !== 'object') return { value: String(result).slice(0, 2000) };
  const out = {
    ok: result.ok,
    applied: result.applied,
    section: result.section,
    target: result.target,
    id: result.id,
    action: result.action,
    summary: result.summary,
    warnings: Array.isArray(result.warnings) ? result.warnings.slice(0, 12) : result.warnings,
    changes: Array.isArray(result.changes) ? result.changes.slice(0, 40) : result.changes,
    changeCount: Array.isArray(result.changes) ? result.changes.length : undefined,
    next_actions: Array.isArray(result.next_actions)
      ? result.next_actions.slice(0, 8).map(compactNextAction)
      : result.next_actions,
    meta: result.meta,
    check: compactCheck(result.check),
    post_apply_check: compactCheck(result.post_apply_check),
    error: result.error,
    diff: typeof result.diff === 'string' ? result.diff.slice(0, 6000) : result.diff
  };
  if (out.summary && typeof out.summary === 'string') out.summary = out.summary.slice(0, 2000);
  for (const key of Object.keys(out)) {
    if (out[key] === undefined || out[key] === null) delete out[key];
  }
  return out;
}

function patchArgs({ stage, target, id, apply, refreshDerived }) {
  const section = stage === 'write' ? 'chapter' : stage;
  const args = ['tool', 'patch', section];
  if (section === 'outline') {
    args.push('--target', target || (id && String(id).startsWith('volume:') ? 'volume' : 'chapter'));
  } else if (section === 'craft') {
    if (!target) throw new Error('craft patches need target: character|item|location|organization');
    args.push('--target', target);
  }
  const cleanId = String(id || '').trim();
  if (cleanId) args.push('--id', cleanId);
  if (apply) args.push('--apply');
  if (apply && section === 'chapter' && refreshDerived !== false) args.push('--refresh-derived');
  return args;
}

export async function applyPatch(options) {
  const {
    root,
    config,
    stage,
    target,
    id,
    payload,
    apply = true,
    refreshDerived = true,
    timeoutMs = 300000
  } = options;

  let body;
  if (typeof payload === 'string') {
    body = payload.trim();
  } else {
    body = JSON.stringify(payload);
  }
  if (!body || body === '{}' || body === 'null') {
    return { ok: false, phase: 'input', error: 'payload must be a non-empty JSON object' };
  }

  // `tool patch setup` merges into an existing artifact. A brand-new project
  // has no story_setup.json yet, so create the first setup through the same
  // mutation service with `tool replace setup`.
  if (stage === 'setup' && !fs.existsSync(path.join(root, 'story/setup/story_setup.json'))) {
    if (!apply) {
      return {
        ok: true,
        applied: false,
        dryRun: {
          ok: true,
          note: 'story_setup.json is missing; apply will create it through tool replace setup'
        }
      };
    }
    const replaced = await replaceArtifact(root, 'setup', { config, input: body, timeoutMs });
    if (!replaced.ok) {
      return {
        ok: false,
        phase: 'apply',
        error: replaced.error || 'tool replace rejected the setup artifact',
        result: compactPatchResult(replaced.json),
        stderr: replaced.stderr,
        stdout: replaced.stdout
      };
    }
    const checked = await toolCheck(root, ['all', '--target', 'setup'], { config, timeoutMs });
    return {
      ok: true,
      applied: true,
      phase: 'apply',
      result: compactPatchResult(replaced.json),
      check: compactCheck(checked.json),
      stderr: replaced.stderr
    };
  }

  // Outline generation owns the whole artifact. `tool replace outline` commits
  // it through MutationService, so a DSH subagent can produce the outline
  // without depending on the CLI's compose/LLM path. A targeted volume/chapter
  // improvement is a patch, not a whole-outline replacement.
  if (stage === 'outline') {
    const targeted = Boolean(target) || Boolean(String(id || '').trim());
    if (!targeted) {
      if (!apply) {
        return {
          ok: true,
          applied: false,
          dryRun: { ok: true, note: 'outline is a complete-artifact replace; apply commits it' }
        };
      }
      const replaced = await replaceArtifact(root, 'outline', { config, input: body, timeoutMs });
      if (!replaced.ok) {
        return {
          ok: false,
          phase: 'apply',
          error: replaced.error || 'tool replace rejected the outline artifact',
          result: compactPatchResult(replaced.json),
          stderr: replaced.stderr,
          stdout: replaced.stdout
        };
      }
      return {
        ok: true,
        applied: true,
        phase: 'apply',
        result: compactPatchResult(replaced.json),
        stderr: replaced.stderr
      };
    }
  }

  let args;
  try {
    args = patchArgs({ stage, target, id, apply: false, refreshDerived });
  } catch (error) {
    return { ok: false, phase: 'input', error: error instanceof Error ? error.message : String(error) };
  }

  const dry = await runNovelgen(root, [...args, '--dir', root], { config, input: body, timeoutMs });
  if (!dry.json) {
    return {
      ok: false,
      phase: 'dry-run',
      error: dry.error || 'novelgen returned no JSON',
      stderr: tail(dry.stderr),
      stdout: tail(dry.stdout)
    };
  }
  const dryJson = dry.json;
  const blocking = dryJson?.check?.blocking === true || dryJson?.blocking === true;
  if (dryJson.ok === false || blocking || dry.code !== 0) {
    return {
      ok: false,
      phase: 'dry-run',
      error: blocking ? 'dry-run reported blocking issues' : dry.error || 'dry-run rejected the patch',
      dryRun: compactPatchResult(dryJson),
      stderr: tail(dry.stderr)
    };
  }
  if (!apply) {
    return { ok: true, applied: false, dryRun: compactPatchResult(dryJson) };
  }

  args = patchArgs({ stage, target, id, apply: true, refreshDerived });
  const applied = await runNovelgen(root, [...args, '--dir', root], { config, input: body, timeoutMs });
  const appliedJson = applied.json;
  if (!appliedJson) {
    return {
      ok: false,
      phase: 'apply',
      error: applied.error || 'novelgen returned no JSON',
      dryRun: compactPatchResult(dryJson),
      stderr: tail(applied.stderr),
      stdout: tail(applied.stdout)
    };
  }
  return {
    ok: appliedJson.ok !== false && applied.code === 0,
    applied: appliedJson.applied !== false,
    phase: 'apply',
    dryRun: compactPatchResult(dryJson),
    result: compactPatchResult(appliedJson),
    stderr: applied.ok ? undefined : tail(applied.stderr)
  };
}

export async function replaceArtifact(root, section, options = {}) {
  const { config, input, id, target, timeoutMs = 300000 } = options;
  const args = ['tool', 'replace', section];
  if (target) args.push('--target', target);
  if (id) args.push('--id', id);
  args.push('--apply');
  args.push('--dir', root);
  const result = await runNovelgen(root, args, { config, input, timeoutMs });
  return {
    ok: result.ok && Boolean(result.json) && result.json.ok !== false,
    json: result.json,
    error: result.error || (result.json ? undefined : 'novelgen returned no JSON'),
    stderr: tail(result.stderr),
    stdout: tail(result.stdout)
  };
}

export async function toolRefresh(root, target, options = {}) {
  const { config, id, batchSize, timeoutMs = 600000 } = options;
  const args = ['tool', 'refresh', target];
  if (id) args.push('--id', id);
  if (batchSize) args.push('--batch-size', String(batchSize));
  args.push('--dir', root);
  const result = await runNovelgen(root, args, { config, timeoutMs });
  return {
    ok: result.ok && Boolean(result.json),
    json: result.json,
    error: result.error || (result.json ? undefined : 'novelgen returned no JSON'),
    stderr: tail(result.stderr)
  };
}

export async function toolQuery(root, args, options = {}) {
  const { config, timeoutMs = 180000 } = options;
  const result = await runNovelgen(root, ['tool', 'query', ...args, '--dir', root], { config, timeoutMs });
  return {
    ok: result.ok && Boolean(result.json),
    json: result.json,
    error: result.error || (result.json ? undefined : 'novelgen returned no JSON'),
    stderr: tail(result.stderr)
  };
}

export async function toolCheck(root, args, options = {}) {
  const { config, timeoutMs = 180000 } = options;
  const result = await runNovelgen(root, ['tool', 'check', ...args, '--dir', root], { config, timeoutMs });
  return {
    ok: result.ok && Boolean(result.json),
    json: result.json,
    error: result.error || (result.json ? undefined : 'novelgen returned no JSON'),
    stderr: tail(result.stderr)
  };
}

export async function toolSkill(root, name, options = {}) {
  const { config, timeoutMs = 60000 } = options;
  const result = await runNovelgen(root, ['tool', 'skill', 'read', name, '--dir', root], {
    config,
    timeoutMs
  });
  const content = result.json?.results?.[0]?.content;
  return {
    ok: result.ok && typeof content === 'string',
    content: content || '',
    error: result.error || (content ? undefined : 'skill not found')
  };
}
