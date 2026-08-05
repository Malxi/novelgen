#!/usr/bin/env python3
"""Claude Agent SDK runner for Novelgen.

The runner intentionally speaks a small JSON protocol over stdin/stdout so the
Go CLI stays in charge of project state and validation.
"""

from __future__ import annotations

import asyncio
import datetime as _datetime
import inspect
import json
import posixpath
import re
import os
import shutil
import sys
import time as _time
import urllib.error
import urllib.request
from typing import Any, Dict, Iterable, Optional

DEFAULT_MAX_FULL_OUTPUT_SCHEMA_CHARS = 8000


def configure_stdio() -> None:
    configure_text_stream(sys.stdin)
    configure_text_stream(sys.stdout)
    configure_text_stream(sys.stderr)


def configure_text_stream(stream: Any) -> None:
    reconfigure = getattr(stream, "reconfigure", None)
    if callable(reconfigure):
        reconfigure(encoding="utf-8", errors="strict")


def main() -> int:
    configure_stdio()
    try:
        invocation = json.load(sys.stdin)
        result = asyncio.run(run(invocation))
        sys.stdout.write(json.dumps(result, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001 - runner errors must cross process boundary.
        sys.stderr.write(str(exc))
        return 1


async def run(invocation: Dict[str, Any]) -> Dict[str, Any]:
    try:
        return await run_with_claude_agent_sdk(invocation)
    except ImportError:
        if invocation.get("require_sdk"):
            raise RuntimeError("claude_agent_sdk is required for this agent workflow")
        return run_with_anthropic_http(invocation)


async def run_with_claude_agent_sdk(invocation: Dict[str, Any]) -> Dict[str, Any]:
    from claude_agent_sdk import ClaudeAgentOptions, query  # type: ignore

    prompt = build_prompt(invocation)
    options_kwargs: Dict[str, Any] = {}
    accepted = set(inspect.signature(ClaudeAgentOptions).parameters)
    model = invocation.get("options", {}).get("model") or os.getenv("ANTHROPIC_MODEL")
    max_tokens = invocation.get("options", {}).get("max_tokens")
    max_turns = invocation.get("options", {}).get("max_turns") or int(os.getenv("NOVELGEN_AGENT_MAX_TURNS", "8"))
    system_prompt = invocation.get("system_prompt", "")
    cwd = invocation.get("workspace_root")
    settings = invocation.get("settings")
    setting_sources = invocation.get("setting_sources")
    cli_path = invocation.get("cli_path") or os.getenv("CLAUDE_CODE_PATH") or shutil.which("claude")
    sdk_skills = invocation.get("sdk_skills")
    add_dirs = invocation.get("add_dirs")
    tools = invocation.get("tools")
    allowed_tools = invocation.get("allowed_tools")
    permission_mode = invocation.get("permission_mode")
    requested_allowed_tools = list(allowed_tools or [])
    requested_permission_mode = permission_mode
    tool_allowlist = invocation.get("tool_allowlist") or []
    live_log = LiveLogger(invocation.get("live_log_path"))
    if tool_allowlist:
        allowed_tools = []
        if permission_mode == "dontAsk":
            permission_mode = "default"

    sdk_skill_prompt, loaded_sdk_skills, missing_sdk_skills = load_sdk_skill_prompt(sdk_skills or [], add_dirs or [])
    if missing_sdk_skills:
        live_log.write("error", {
            "type": "missing_sdk_skills",
            "detail": "requested SDK skill(s) not found in add_dirs",
            "missing_sdk_skills": missing_sdk_skills,
            "add_dirs": add_dirs or [],
        })
        raise RuntimeError(
            "requested SDK skill(s) not found in add_dirs: "
            + ", ".join(missing_sdk_skills)
        )
    if sdk_skill_prompt:
        system_prompt = f"{sdk_skill_prompt}\n\n{system_prompt}"
    if cwd:
        system_prompt = (
            "=== NOVELGEN WORKSPACE ===\n"
            f"Project workspace root: {cwd}\n"
            "Bash tools already start in this project workspace. Do not cd outside it.\n"
            "Use `novelgen tool ...` directly; if an absolute executable is needed, use the NOVELGEN_CLI_PATH environment variable.\n"
            "Do not search the filesystem for novel.json or source files.\n"
            "=== END NOVELGEN WORKSPACE ===\n\n"
            f"{system_prompt}"
        )
    system_prompt = sanitize_model_text(system_prompt)

    for key, value in {
        "system_prompt": system_prompt,
        "model": model,
        "max_tokens": max_tokens,
        "max_turns": max_turns,
        "cwd": cwd,
        "settings": settings,
        "setting_sources": setting_sources,
        "cli_path": cli_path,
        "skills": sdk_skills,
        "add_dirs": add_dirs,
        "tools": tools,
        "allowed_tools": allowed_tools,
        "permission_mode": permission_mode,
    }.items():
        if key in accepted and value not in (None, "", []):
            options_kwargs[key] = value

    if tool_allowlist and "can_use_tool" in accepted:
        options_kwargs["can_use_tool"] = build_tool_permission_callback(tool_allowlist, live_log, cwd)
    if tool_allowlist and "hooks" in accepted:
        options_kwargs["hooks"] = build_tool_hooks(tool_allowlist, live_log, cwd, invocation.get("tool_evidence"))

    schema = invocation.get("output_json_schema")
    output_schema_chars = schema_json_length(schema)
    output_format_schema_mode = ""
    output_format_skipped_reason = ""
    if schema and invocation.get("disable_sdk_output_format"):
        output_format_skipped_reason = "disabled by invocation; Go will parse and validate JSON"
    elif schema and "output_format" in accepted:
        if invocation.get("compact_output_schema"):
            output_schema = compact_top_level_schema(schema)
            output_format_schema_mode = "compact_top_level_forced"
            output_format_skipped_reason = "compact output schema requested by invocation"
        else:
            output_schema, output_format_schema_mode, output_format_skipped_reason = select_sdk_output_schema(schema)
        options_kwargs["output_format"] = {"type": "json_schema", "schema": output_schema}
    elif schema and "output_format" not in accepted:
        output_format_skipped_reason = "ClaudeAgentOptions does not support output_format"

    options = ClaudeAgentOptions(**options_kwargs)
    sdk_prompt: str | Any = prompt
    if "can_use_tool" in options_kwargs:
        sdk_prompt = single_user_message_stream(prompt)
    assistant_text = ""
    final_result: Optional[str] = None
    final_structured: Any = None
    usage: Dict[str, int] = {}
    live_log.write("start", {
        "agent_name": invocation.get("agent_name", ""),
        "command": invocation.get("command", ""),
        "model": model or "",
        "workspace_root": cwd or "",
        "cli_path": cli_path or "",
        "sdk_skills": sdk_skills or [],
        "loaded_sdk_skills": loaded_sdk_skills,
        "missing_sdk_skills": missing_sdk_skills,
        "sdk_skill_prompt_chars": len(sdk_skill_prompt),
        "tools": tools or [],
        "allowed_tools": allowed_tools or [],
        "permission_mode": permission_mode or "",
        "requested_allowed_tools": requested_allowed_tools,
        "requested_permission_mode": requested_permission_mode or "",
        "tool_gate": "can_use_tool+hooks" if tool_allowlist else "sdk_options",
        "tool_allowlist": tool_allowlist,
        "sdk_output_format": "output_format" in options_kwargs,
        "sdk_output_format_schema_mode": output_format_schema_mode,
        "sdk_output_format_skipped_reason": output_format_skipped_reason,
        "output_schema_chars": output_schema_chars,
        "max_turns": max_turns,
        "can_use_tool": "can_use_tool" in options_kwargs,
        "hooks": list((options_kwargs.get("hooks") or {}).keys()),
    })

    try:
        async for message in query(prompt=sdk_prompt, options=options):
            structured = extract_message_value(message, "structured_output")
            if structured is not None:
                structured = repair_encoding_value(structured)
                final_structured = structured
            result = extract_message_value(message, "result")
            if result not in (None, ""):
                result = repair_encoding_value(result)
                final_result = encode_content(result)
            message_text = extract_message_text(message)
            if message_text:
                message_text = repair_text_encoding(message_text)
                assistant_text = message_text
            message_usage = extract_message_value(message, "usage")
            if message_usage:
                usage = normalize_usage(message_usage)
            live_log.write("message", summarize_live_message(message, message_text, structured, result, message_usage))

        if final_structured is not None:
            text = encode_content(final_structured)
        elif final_result is not None:
            text = final_result
        else:
            text = assistant_text
        text = repair_text_encoding(text)
        live_log.write("final", {"content": text.strip(), "model": model or "", "usage": usage})

        return {
            "content": text.strip(),
            "model": model or "",
            "usage": usage,
        }
    except Exception as exc:  # noqa: BLE001 - preserve SDK exception across process boundary.
        detail = sdk_exception_detail(exc, assistant_text, final_result)
        live_log.write("error", {
            "type": type(exc).__name__,
            "message": detail,
            "repr": repr(exc),
        })
        raise RuntimeError(f"{type(exc).__name__}: {detail}") from exc
    finally:
        live_log.close()


def sdk_exception_detail(exc: Exception, assistant_text: str = "", final_result: Optional[str] = None) -> str:
    streamed = first_streamed_api_error(assistant_text, final_result)
    if streamed:
        return streamed
    return str(exc) or repr(exc)


def first_streamed_api_error(*values: Any) -> str:
    for value in values:
        if value in (None, ""):
            continue
        text = str(value)
        lower = text.lower()
        marker = lower.find("api error:")
        if marker >= 0:
            line = text[marker:].splitlines()[0].strip()
            return line
    return ""


def run_with_anthropic_http(invocation: Dict[str, Any]) -> Dict[str, Any]:
    model = invocation.get("options", {}).get("model") or os.getenv("ANTHROPIC_MODEL") or "sonnet"
    max_tokens = invocation.get("options", {}).get("max_tokens") or 4096
    temperature = invocation.get("options", {}).get("temperature")
    base_url = (os.getenv("ANTHROPIC_BASE_URL") or "https://api.anthropic.com").rstrip("/")
    token = os.getenv("ANTHROPIC_AUTH_TOKEN") or os.getenv("ANTHROPIC_API_KEY")
    if not token:
        raise RuntimeError("ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY is required")
    live_log = LiveLogger(invocation.get("live_log_path"))
    live_log.write("start", {
        "agent_name": invocation.get("agent_name", ""),
        "command": invocation.get("command", ""),
        "model": model,
        "workspace_root": invocation.get("workspace_root", ""),
        "transport": "anthropic_http",
    })

    body: Dict[str, Any] = {
        "model": model,
        "max_tokens": max_tokens,
        "system": invocation.get("system_prompt", ""),
        "messages": [{"role": "user", "content": build_prompt(invocation)}],
    }
    if temperature not in (None, 0, 0.0):
        body["temperature"] = temperature

    data = json.dumps(body, ensure_ascii=False).encode("utf-8")
    errors = []
    for url in anthropic_urls(base_url):
        req = urllib.request.Request(url, data=data, method="POST")
        req.add_header("content-type", "application/json")
        req.add_header("anthropic-version", "2023-06-01")
        req.add_header("x-api-key", token)
        req.add_header("authorization", f"Bearer {token}")
        try:
            live_log.write("http_request", {"url": url, "model": model, "max_tokens": max_tokens})
            with urllib.request.urlopen(req, timeout=invocation.get("options", {}).get("timeout") or 120) as resp:
                parsed = json.loads(resp.read().decode("utf-8"))
                result = parse_anthropic_response(parsed, model)
                live_log.write("final", result)
                live_log.close()
                return result
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            errors.append(f"{url}: HTTP {exc.code}: {detail}")
            live_log.write("error", {"url": url, "status": exc.code, "detail": detail})
        except urllib.error.URLError as exc:
            errors.append(f"{url}: {exc}")
            live_log.write("error", {"url": url, "detail": str(exc)})
    live_log.close()
    raise RuntimeError("; ".join(errors))


def schema_json_length(schema: Any) -> int:
    if not schema:
        return 0
    try:
        return len(json.dumps(schema, ensure_ascii=False, separators=(",", ":")))
    except (TypeError, ValueError):
        return 0


def max_full_output_schema_chars() -> int:
    raw = os.getenv("NOVELGEN_SDK_FULL_OUTPUT_SCHEMA_MAX_CHARS", "")
    if raw:
        try:
            value = int(raw)
        except ValueError:
            value = 0
        if value > 0:
            return value
    return DEFAULT_MAX_FULL_OUTPUT_SCHEMA_CHARS


def select_sdk_output_schema(schema: Dict[str, Any]) -> tuple[Dict[str, Any], str, str]:
    schema_chars = schema_json_length(schema)
    max_chars = max_full_output_schema_chars()
    if schema_chars <= max_chars:
        return schema, "full", ""
    compact = compact_top_level_schema(schema)
    compact_chars = schema_json_length(compact)
    return (
        compact,
        "compact_top_level",
        f"full JSON schema is {schema_chars} chars, above {max_chars}; using compact top-level schema ({compact_chars} chars) to avoid long CLI args",
    )


def compact_top_level_schema(schema: Dict[str, Any]) -> Dict[str, Any]:
    if not isinstance(schema, dict) or schema.get("type") != "object":
        return {"type": "object"}
    properties = schema.get("properties")
    if not isinstance(properties, dict) or not properties:
        return {"type": "object", "additionalProperties": True}

    compact_properties: Dict[str, Any] = {}
    for key, value in properties.items():
        if not isinstance(key, str):
            continue
        compact_properties[key] = compact_property_schema(value)

    compact: Dict[str, Any] = {
        "type": "object",
        "properties": compact_properties,
        "required": list(compact_properties.keys()),
        "additionalProperties": False,
    }
    return compact


def compact_property_schema(value: Any) -> Dict[str, Any]:
    if not isinstance(value, dict):
        return {}
    value_type = value.get("type")
    if isinstance(value_type, list):
        non_null = [item for item in value_type if item != "null"]
        value_type = non_null[0] if non_null else "object"
    if value_type == "array":
        items = value.get("items")
        item_type = "object"
        if isinstance(items, dict):
            raw_item_type = items.get("type")
            if isinstance(raw_item_type, str):
                item_type = raw_item_type
        return {"type": "array", "items": {"type": item_type}}
    if value_type in ("string", "number", "integer", "boolean"):
        return {"type": value_type}
    return {"type": "object", "additionalProperties": True}


def build_prompt(invocation: Dict[str, Any]) -> str:
    user_prompt = invocation.get("user_prompt", "")
    schema_text = invocation.get("output_schema_text", "")
    if schema_text:
        return sanitize_model_text(f"{user_prompt}\n\nReturn only JSON matching this schema:\n{schema_text}")
    return sanitize_model_text(user_prompt)


def load_sdk_skill_prompt(skill_names: list[str], add_dirs: list[str]) -> tuple[str, list[str], list[str]]:
    if not skill_names:
        return "", [], []
    sections: list[str] = []
    loaded: list[str] = []
    missing: list[str] = []
    for name in skill_names:
        if not isinstance(name, str) or not name.strip():
            continue
        clean_name = name.strip()
        content = read_materialized_skill(clean_name, add_dirs)
        if content:
            loaded.append(clean_name)
            sections.append(f"=== SDK WORKFLOW SKILL: {clean_name} ===\n\n{content}")
        else:
            missing.append(clean_name)
    if not sections:
        return "", loaded, missing
    return "=== SDK WORKFLOW SKILLS ===\n\n" + "\n\n".join(sections) + "\n\n=== END SDK WORKFLOW SKILLS ===", loaded, missing


def read_materialized_skill(name: str, add_dirs: list[str]) -> str:
    for root in add_dirs:
        if not isinstance(root, str) or not root:
            continue
        path = os.path.join(root, name, "SKILL.md")
        try:
            with open(path, "r", encoding="utf-8") as f:
                return f.read()
        except OSError:
            continue
    return ""


def sanitize_model_text(value: str) -> str:
    if not isinstance(value, str) or not value:
        return value
    cleaned: list[str] = []
    for ch in value:
        code = ord(ch)
        if 0xD800 <= code <= 0xDFFF:
            cleaned.append("\uFFFD")
            continue
        if code > 0xFFFF:
            cleaned.append("?")
            continue
        cleaned.append(ch)
    return "".join(cleaned)


MOJIBAKE_MARKERS = (
    "\uFFFD", "????", "锟斤", "锟竭", "锛", "銆", "鈫", "€",
    "鍘", "鏋", "鐏", "绉", "搴", "璇", "宸", "淇",
    "绔", "闂", "敓", "嬫", "勫", "堕", "熷", "荤",
)


def repair_text_encoding(value: str) -> str:
	if not isinstance(value, str) or not value:
		return value
	score = mojibake_score(value)
	if score < 2:
		return value
	for source_encoding in ("gbk", "cp936"):
		try:
			repaired = value.encode(source_encoding).decode("utf-8")
		except UnicodeError:
			continue
		repaired_score = mojibake_score(repaired)
		if repaired_score > 0 and repaired_score >= score // 2:
			continue
		if cjk_ratio(repaired) + 0.15 < cjk_ratio(value):
			continue
		if replacement_char_count(repaired) > replacement_char_count(value):
			continue
		return repaired
	return value


def repair_encoding_value(value: Any) -> Any:
    if isinstance(value, str):
        return repair_text_encoding(value)
    if isinstance(value, list):
        return [repair_encoding_value(item) for item in value]
    if isinstance(value, dict):
        return {key: repair_encoding_value(item) for key, item in value.items()}
    return value


def mojibake_score(value: str) -> int:
	return sum(value.count(marker) for marker in MOJIBAKE_MARKERS)


def replacement_char_count(value: str) -> int:
	return value.count("\uFFFD")


def cjk_ratio(value: str) -> float:
	if not value:
		return 0.0
	letters = [ch for ch in value if ch.isalpha()]
	if not letters:
		return 0.0
	cjk = sum(1 for ch in letters if is_cjk_char(ch))
	return cjk / len(letters)


def is_cjk_char(ch: str) -> bool:
	code = ord(ch)
	return (
		0x3400 <= code <= 0x4DBF
		or 0x4E00 <= code <= 0x9FFF
		or 0xF900 <= code <= 0xFAFF
		or 0x20000 <= code <= 0x2A6DF
		or 0x2A700 <= code <= 0x2B73F
		or 0x2B740 <= code <= 0x2B81F
		or 0x2B820 <= code <= 0x2CEAF
	)


def build_tool_permission_callback(allowlist: list[str], live_log: Optional["LiveLogger"] = None, workspace_root: str = ""):
    from claude_agent_sdk.types import PermissionResultAllow, PermissionResultDeny  # type: ignore

    async def can_use_tool(tool_name: str, tool_input: Dict[str, Any], context: Any):
        command = extract_tool_command(tool_input)
        reason = tool_call_denial_reason(tool_name, tool_input, allowlist, workspace_root)
        allowed = reason == ""
        if live_log is not None:
            live_log.write("tool_permission", {
                "tool_name": tool_name,
                "command": summarize_live_tool_command(command),
                "allowed": allowed,
                "reason": reason,
            })
        if allowed:
            return PermissionResultAllow()
        return PermissionResultDeny(message=reason)

    return can_use_tool


def hook_safe(hook_name: str, live_log: Optional["LiveLogger"], fallback: Dict[str, Any], callback: Any):
    """Wrap a hook callback so an exception is recorded instead of crashing the
    whole agent turn. The fallback keeps the workflow moving; Go-side evidence
    validation still runs after the turn as the final safety net."""

    async def wrapped(input_data: Any, tool_use_id: Optional[str], context: Any):
        try:
            return await callback(input_data, tool_use_id, context)
        except Exception as exc:  # noqa: BLE001 - hook errors must not kill the run.
            if live_log is not None:
                live_log.write("hook_error", {
                    "hook": hook_name,
                    "error": f"{type(exc).__name__}: {exc}",
                    "repr": repr(exc),
                })
            return fallback

    return wrapped


def build_tool_hooks(
    allowlist: list[str],
    live_log: Optional["LiveLogger"] = None,
    workspace_root: str = "",
    tool_evidence: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    from claude_agent_sdk.types import HookMatcher  # type: ignore

    required_queries = exact_required_queries(allowlist)
    stop_after_required_queries = stop_after_required_queries_enabled(allowlist)
    deny_repeated_required_queries = stop_after_required_queries
    evidence_minimums_map = evidence_minimums(tool_evidence)
    evidence_required_commands = [c for c in ((tool_evidence or {}).get("required_tool_commands") or []) if c]
    evidence_counts: Dict[str, int] = {"query": 0, "context": 0, "check": 0, "patch_apply": 0}
    observed_commands: set[str] = set()
    deny_guard_enabled = bool((tool_evidence or {}).get("require_no_denied_tools"))
    tool_call_counter = 0
    last_permission_denial_call = -1
    denials_resolved = False
    completed_queries: set[str] = set()
    utf8_retry_queries: set[str] = set()
    followup_checks_allowed: set[str] = set()
    patch_dry_run_state: Dict[str, str] = {}
    patch_dry_run_fingerprints: Dict[str, str] = {}
    patch_dry_runs: Dict[str, int] = {}
    patch_applies: Dict[str, int] = {}
    clean_check_targets: set[str] = set()
    pending_post_apply_checks: set[str] = set()
    post_apply_clean_targets: set[str] = set()
    completed_check_targets: set[str] = set()
    pending_patch_after_precheck: set[str] = set()
    refreshed_by_apply_targets: set[str] = set()
    exact_log_brief_queries: set[str] = set()
    tool_started_at: Dict[str, float] = {}

    async def pre_tool_use(input_data: Any, tool_use_id: Optional[str], context: Any):
        nonlocal tool_call_counter, last_permission_denial_call, denials_resolved
        tool_name = extract_message_value(input_data, "tool_name") or ""
        tool_input = extract_message_value(input_data, "tool_input") or {}
        command = extract_tool_command(tool_input)
        allowlist_reason = tool_call_denial_reason(tool_name, tool_input, allowlist, workspace_root)
        reason = allowlist_reason
        allowed = reason == ""
        matched_query = matching_required_query(command, required_queries)
        canonical_command = canonical_novelgen_command(normalize_command(command))
        exact_log_brief_key = exact_log_brief_query_key(canonical_command)
        is_allowed_followup_check = canonical_command in followup_checks_allowed
        is_utf8_required_query_retry = bool(
            matched_query
            and matched_query in completed_queries
            and matched_query not in utf8_retry_queries
            and is_utf8_powershell_readonly_wrapper(command)
        )
        if not allowed and is_allowed_followup_check and is_followup_allowlist_denial(reason):
            allowed = True
            reason = ""
        if (
            not allowed
            and stop_after_required_queries
            and required_queries
            and completed_queries.issuperset(required_queries)
            and not is_allowed_followup_check
            and not is_utf8_required_query_retry
        ):
            allowed = False
            reason = (
                "All required_queries have already been executed. Do not use Bash, printf, echo, cd, "
                "patch, or more novelgen tools. Return only the final JSON in the assistant response now."
            )
        clean_target_key = clean_checked_detail_query_target(command)
        if allowed and clean_target_key and clean_target_is_closed(clean_target_key, pending_post_apply_checks):
            allowed = False
            reason = "The patch apply for this target has not had its required focused follow-up check yet. Run the focused check before querying more target content."
        if allowed and clean_target_key and clean_target_is_closed(clean_target_key, clean_check_targets):
            allowed = False
            reason = "The focused check for this target returned no issues. Use the existing tool results and return final JSON instead of expanding context."
        refresh_target = refresh_target_key(command)
        if allowed and refresh_target and refresh_target in refreshed_by_apply_targets:
            allowed = False
            reason = "The patch apply already used --refresh-derived for this chapter. Do not refresh again; run the focused check or return final JSON."
        if allowed and refresh_target and clean_target_is_closed(refresh_target, pending_post_apply_checks) and not is_allowed_followup_check:
            allowed = False
            reason = "The patch apply for this target has not had its required focused follow-up check yet. Run the focused check before refreshing or querying more."
        if allowed and refresh_target and clean_target_is_closed(refresh_target, clean_check_targets):
            allowed = False
            reason = "The focused chapter check for this target already returned no issues. Do not refresh derived DSL again; return final JSON now."
        patch_key = patch_target_key(command)
        patch_buffer_key = patch_buffer_target_key(command)
        patch_clean_key = patch_key or patch_buffer_key
        if (
            allowed
            and patch_clean_key
            and patch_applies.get(patch_clean_key, 0) > 0
        ):
            if patch_clean_key in pending_post_apply_checks:
                allowed = False
                reason = "The patch apply for this target has not had its required focused follow-up check yet. Run the focused check before starting another patch cycle."
            elif clean_target_is_closed(patch_clean_key, post_apply_clean_targets):
                allowed = False
                reason = "The focused post-apply check for this target is clean. Do not start another patch cycle; return final JSON now."
        precheck_key = required_chapter_patch_precheck_key(command, allowlist)
        if allowed and precheck_key and precheck_key not in completed_check_targets:
            allowed = False
            pending_patch_after_precheck.add(precheck_key)
            reason = (
                "Run the required focused chapter check before patching this chapter: "
                "`novelgen tool check all --target chapter --scope chapter --id "
                f"\"{precheck_key[len('chapter:') :]}\" --min-priority low --max-issues 12`."
            )
        if (
            not allowed
            and patch_key
            and not command_uses_apply(command)
            and patch_dry_run_state.get(patch_key, "") == "validated"
        ):
            reason = (
                "This patch target already has a successful dry-run. "
                "Repeat the same stdin-piped patch command with --apply now, or return final JSON. "
                "Do not call the patch tool again without --apply."
            )
        if allowed and deny_repeated_required_queries and matched_query and matched_query in completed_queries and not is_allowed_followup_check and not is_utf8_required_query_retry:
            allowed = False
            reason = "This required query has already been executed. Use the previous tool result and return final JSON."
        if allowed and exact_log_brief_key and exact_log_brief_queries and exact_log_brief_key not in exact_log_brief_queries:
            allowed = False
            reason = "An exact history log brief has already been queried in this agent run. Use that result and return final JSON instead of reading more logs."
        if allowed and is_utf8_required_query_retry:
            utf8_retry_queries.add(matched_query)
        if allowed and is_allowed_followup_check:
            followup_checks_allowed.discard(canonical_command)
        if allowed and patch_buffer_key and patch_buffer_is_mutation(command) and patch_dry_run_state.get(patch_buffer_key, "") == "validated":
            allowed = False
            if workflow_allows_apply_for_patch_command(command, allowlist):
                reason = (
                    "This patch target already has a successful dry-run. Apply the same validated patch with --apply "
                    "or return final JSON now. Do not clear or append the patch buffer again."
                )
            else:
                reason = (
                    "This patch target already has a successful dry-run and this workflow does not allow --apply. "
                    "Return final JSON now with the same patch content. Do not clear or append the patch buffer again."
                )
        if allowed and patch_key:
            if command_uses_apply(command):
                attempts = patch_applies.get(patch_key, 0)
                dry_run_state = patch_dry_run_state.get(patch_key, "")
                if dry_run_state == "repair_required":
                    allowed = False
                    reason = "This patch target's last dry-run reported blocking issues. Repair the patch content and dry-run again before --apply."
                elif dry_run_state == "":
                    allowed = False
                    reason = "This patch target has not had a successful dry-run in this agent run. Run the same patch without --apply first."
                elif patch_dry_run_fingerprints.get(patch_key, "") != patch_command_fingerprint(command):
                    allowed = False
                    reason = "This patch apply does not match the last successful dry-run for the same target. Repeat the exact patch dry-run first."
                apply_limit = patch_apply_limit(command, patch_key)
                if not allowed:
                    pass
                elif attempts >= apply_limit:
                    allowed = False
                    reason = f"This target already had {apply_limit} apply attempt(s). Return final JSON or explain the concrete validation issue."
                else:
                    patch_applies[patch_key] = attempts + 1
                    patch_dry_run_state.pop(patch_key, None)
                    patch_dry_run_fingerprints.pop(patch_key, None)
            else:
                dry_run_state = patch_dry_run_state.get(patch_key, "")
                if dry_run_state == "validated":
                    allowed = False
                    if workflow_allows_apply_for_patch_command(command, allowlist):
                        reason = (
                            "This patch target already has a successful dry-run. "
                            "Repeat the same stdin-piped patch command with --apply now, or return final JSON. "
                            "Do not run another dry-run for this target."
                        )
                    else:
                        reason = (
                            "This patch target already has a successful dry-run and this workflow does not allow --apply. "
                            "Return final JSON now. Do not run another dry-run for this target."
                        )
                else:
                    apply_limit = patch_apply_limit(command, patch_key)
                    if patch_applies.get(patch_key, 0) >= apply_limit:
                        allowed = False
                        reason = f"This target already had {apply_limit} apply attempt(s). Return final JSON instead of starting another patch cycle."
                    else:
                        attempts = patch_dry_runs.get(patch_key, 0)
                        max_dry_runs = patch_dry_run_limit(patch_key)
                        if attempts >= max_dry_runs:
                            allowed = False
                            reason = f"This target already had {max_dry_runs} patch dry-runs. Return final JSON or use a different validated target."
                        else:
                            patch_dry_runs[patch_key] = attempts + 1
        patch_attempts = patch_dry_runs.get(patch_key, 0) + patch_applies.get(patch_key, 0) if patch_key else 0
        patch_apply = command_uses_apply(command) if patch_key else False
        workflow_denial = (not allowed) and (allowlist_reason == "")
        tool_call_counter += 1
        if not allowed and not workflow_denial:
            last_permission_denial_call = tool_call_counter
            denials_resolved = False
        elif allowed and last_permission_denial_call >= 0:
            denials_resolved = True
        if allowed:
            tool_started_at[tool_timing_key(tool_use_id, command)] = _time.monotonic()
        if live_log is not None:
            live_log.write("tool_hook", {
                "hook": "PreToolUse",
                "tool_name": tool_name,
                "command": summarize_live_tool_command(command),
                "allowed": allowed,
                "matched_query": matched_query,
                "patch_key": patch_key,
                "patch_attempts": patch_attempts,
                "patch_apply": patch_apply,
                "reason": reason,
                "workflow_denial": workflow_denial,
            })
        if allowed:
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "allow",
                    "permissionDecisionReason": "approved novelgen tool command",
                },
            }
        return {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            },
        }

    async def post_tool_use(input_data: Any, tool_use_id: Optional[str], context: Any):
        tool_name = extract_message_value(input_data, "tool_name") or ""
        tool_input = extract_message_value(input_data, "tool_input") or {}
        command = extract_tool_command(tool_input)
        matched_query = matching_required_query(command, required_queries)
        canonical_command = canonical_novelgen_command(normalize_command(command))
        count_evidence_tool_call(command, evidence_counts, observed_commands)
        exact_log_brief_key = exact_log_brief_query_key(canonical_command)
        patch_key = patch_target_key(command)
        duration_ms = tool_elapsed_ms(tool_started_at, tool_timing_key(tool_use_id, command))
        next_action_names = patch_next_action_names(input_data)
        next_action_commands = patch_next_action_commands(input_data)
        if matched_query:
            completed_queries.add(matched_query)
        if exact_log_brief_key:
            exact_log_brief_queries.add(exact_log_brief_key)
        check_key = check_target_key(command)
        if check_key:
            completed_check_targets.add(check_key)
            pending_post_apply_checks.discard(check_key)
        if check_key and check_output_is_clean(input_data):
            clean_check_targets.add(check_key)
            if patch_applies.get(check_key, 0) > 0:
                post_apply_clean_targets.add(check_key)
            if check_key.startswith("chapter:"):
                clean_check_targets.add("craft-context")
        final_json_target_keys = context_final_json_target_keys(command, input_data)
        if final_json_target_keys:
            clean_check_targets.update(final_json_target_keys)
        if patch_key and not command_uses_apply(command):
            dry_run_state = patch_dry_run_state_from_output(input_data)
            if dry_run_state == "repair_required":
                patch_dry_run_state[patch_key] = "repair_required"
                patch_dry_run_fingerprints.pop(patch_key, None)
            elif dry_run_state == "validated":
                patch_dry_run_state[patch_key] = "validated"
                patch_dry_run_fingerprints[patch_key] = patch_command_fingerprint(command)
        if live_log is not None:
            live_log.write("tool_hook", {
                "hook": "PostToolUse",
                "tool_name": tool_name,
                "command": summarize_live_tool_command(command),
                "matched_query": matched_query,
                "completed_queries": sorted(completed_queries),
                "patch_key": patch_key,
                "patch_attempts": patch_dry_runs.get(patch_key, 0) + patch_applies.get(patch_key, 0) if patch_key else 0,
                "patch_apply": command_uses_apply(command) if patch_key else False,
                "next_action_names": next_action_names,
                "next_action_commands": next_action_commands,
                "duration_ms": duration_ms,
            })
        if check_key and check_key in pending_patch_after_precheck:
            pending_patch_after_precheck.discard(check_key)
            target_id = check_key[len("chapter:") :] if check_key.startswith("chapter:") else check_key
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": (
                        "The required focused chapter check has completed. Because an explicit edit request is still pending, "
                        f"continue with a minimal patch for chapter {target_id}: use the patch buffer, run the dry-run, "
                        "then apply the same validated patch with --apply --refresh-derived before final JSON. "
                        "Do not stop only because the check was clean."
                    ),
                },
            }
        if patch_key:
            if command_uses_apply(command):
                if "return_final_json" in next_action_names or "focused_check_clean" in next_action_names:
                    pending_post_apply_checks.discard(patch_key)
                    post_apply_clean_targets.add(patch_key)
                    clean_check_targets.add(patch_key)
                    if patch_key.startswith("chapter:"):
                        clean_check_targets.add("craft-context")
                elif "repair_remaining_issues" in next_action_names or "repair_patch_content" in next_action_names:
                    pending_post_apply_checks.discard(patch_key)
                else:
                    pending_post_apply_checks.add(patch_key)
                refreshed_target = refresh_target_key_from_patch_apply(command)
                if refreshed_target:
                    refreshed_by_apply_targets.add(refreshed_target)
                check_command = post_patch_check_command(command)
                refresh_command = post_patch_refresh_command(command)
                apply_count = patch_applies.get(patch_key, 0)
                check_instruction = "run the focused novelgen tool check for this same target"
                followup_commands = next_action_commands or []
                if not followup_commands and check_command:
                    followup_commands.append(check_command)
                for followup_command in followup_commands:
                    followup_checks_allowed.add(canonical_novelgen_command(normalize_command(followup_command)))
                if next_action_commands:
                    check_instruction = "follow the tool-returned next_actions: " + format_command_sequence(next_action_commands)
                elif check_command:
                    check_instruction = f"run `{check_command}`"
                refresh_instruction = ""
                if not next_action_commands and refresh_command and tool_call_denial_reason("Bash", {"command": refresh_command}, allowlist, workspace_root) == "":
                    followup_checks_allowed.add(canonical_novelgen_command(normalize_command(refresh_command)))
                    refresh_instruction = f"run `{refresh_command}` to refresh derived RPG DSL, then "
                    check_instruction = check_instruction[0].lower() + check_instruction[1:]
                if "return_final_json" in next_action_names or "focused_check_clean" in next_action_names:
                    return {
                        "hookSpecificOutput": {
                            "hookEventName": "PostToolUse",
                            "additionalContext": (
                                "The patch apply has completed and the tool result says the post-apply check is clean. "
                                "Do not run more tools, do not read Claude tool-results temporary files, do not query the saved chapter content, "
                                "and do not refresh derived DSL again. Return the final JSON in the assistant response now."
                            ),
                        },
                    }
                apply_limit = patch_apply_limit(command, patch_key)
                if apply_count >= apply_limit:
                    next_action = (
                        "Do not start another patch-buffer or patch cycle for this target in this agent run. "
                        "If the check still reports issues, return final JSON using the current saved chapter content; "
                        "Go will surface the remaining issues for the next run."
                    )
                else:
                    next_action = "Fix any blocking or high issue with one more focused dry-run/apply cycle if the workflow allows it."
                return {
                    "hookSpecificOutput": {
                        "hookEventName": "PostToolUse",
                        "additionalContext": (
                            f"The patch apply has completed. Before returning final JSON, {refresh_instruction}{check_instruction} "
                            f"and inspect the result. {next_action}"
                        ),
                    },
                }
            check_command = post_patch_check_command(command)
            check_instruction = "run the focused novelgen tool check for the same target"
            if check_command:
                check_instruction = f"run `{check_command}`"
            if not workflow_allows_apply_for_patch_command(command, allowlist):
                canonical_patch_command = canonical_novelgen_command(normalize_command(command))
                go_action = (
                    "Go will save the chapter, refresh derived RPG DSL, and run post-save checks."
                    if "novelgen tool patch chapter" in canonical_patch_command
                    else f"Go will merge/save through the caller workflow and run validation. The focused verification command is {check_instruction}."
                )
                return {
                    "hookSpecificOutput": {
                        "hookEventName": "PostToolUse",
                        "additionalContext": (
                            "The patch dry-run has completed. This workflow does not allow --apply. "
                            f"Return the final JSON now with the same patch content. {go_action} Do not call --apply or repeat "
                            "patch dry-runs for the same target unless the tool output reported a concrete validation error."
                        ),
                    },
                }
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": (
                        "The patch dry-run has completed. If the dry-run result is valid and this workflow "
                        "allows --apply for the target, repeat the same patch command with --apply"
                        f"{patch_apply_refresh_hint(command, allowlist)}, then run "
                        f"{check_instruction}. If --apply is not allowed, "
                        "return the final JSON now with the same patch content. Do not repeat patch dry-runs "
                        "for the same target unless the tool output reported a concrete validation error."
                    ),
                },
            }
        if check_key and check_output_is_clean(input_data) and not clean_check_has_focused_detail_override(check_key, allowlist):
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": clean_check_final_instruction(check_key),
                },
            }
        if final_json_target_keys:
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": tool_returned_final_json_instruction(command),
                },
            }
        if stop_after_required_queries and required_queries and completed_queries.issuperset(required_queries):
            return {
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": "All required_queries have been executed. Do not call more tools. Return only the final JSON now.",
                },
            }
        return {}

    async def stop_hook(input_data: Any, tool_use_id: Optional[str], context: Any):
        missing = missing_evidence_instruction(
            evidence_minimums_map,
            evidence_required_commands,
            evidence_counts,
            observed_commands,
        )
        denial_instruction = denial_guard_instruction(
            deny_guard_enabled,
            last_permission_denial_call >= 0,
            denials_resolved,
        )
        instruction = combine_guard_instructions(missing, denial_instruction)
        guard_active = (
            deny_guard_enabled
            or any(value > 0 for value in evidence_minimums_map.values())
            or bool(evidence_required_commands)
        )
        if not instruction:
            if guard_active and live_log is not None:
                live_log.write("stop_guard", {
                    "decision": "allow",
                    "counts": dict(evidence_counts),
                    "minimums": dict(evidence_minimums_map),
                    "denials_resolved": denials_resolved,
                })
            return {}
        if live_log is not None:
            live_log.write("stop_guard", {
                "decision": "block",
                "reason": instruction,
                "counts": dict(evidence_counts),
                "minimums": dict(evidence_minimums_map),
                "denials_resolved": denials_resolved,
            })
        return {
            "hookSpecificOutput": {
                "hookEventName": "Stop",
                "decision": "block",
                "reason": instruction,
            },
        }

    return {
        "PreToolUse": [HookMatcher(matcher="Bash", hooks=[hook_safe("PreToolUse", live_log, {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "allow",
                "permissionDecisionReason": "hook error fallback; allowlist still enforced by can_use_tool",
            },
        }, pre_tool_use)])],
        "PostToolUse": [HookMatcher(matcher="Bash", hooks=[hook_safe("PostToolUse", live_log, {}, post_tool_use)])],
        "Stop": [HookMatcher(matcher=None, hooks=[hook_safe("Stop", live_log, {}, stop_hook)])],
    }


def evidence_minimums(tool_evidence: Optional[Dict[str, Any]]) -> Dict[str, int]:
    evidence = tool_evidence or {}
    return {
        "query": int(evidence.get("min_query_calls") or 0),
        "context": int(evidence.get("min_context_query_calls") or 0),
        "check": int(evidence.get("min_check_calls") or 0),
        "patch_apply": int(evidence.get("min_patch_apply_calls") or 0),
    }


def count_evidence_tool_call(
    command: str,
    counts: Dict[str, int],
    observed: Optional[set] = None,
) -> None:
    canonical = canonical_novelgen_command(normalize_command(command))
    if not canonical:
        return
    if observed is not None:
        observed.add(canonical)
    if " tool query " in canonical:
        counts["query"] += 1
        if canonical.startswith("novelgen tool query context"):
            counts["context"] += 1
    if " tool check " in canonical:
        counts["check"] += 1
    if " --apply" in canonical and "novelgen tool patch" in canonical:
        counts["patch_apply"] += 1


def missing_evidence_instruction(
    minimums: Dict[str, int],
    required_commands: list[str],
    counts: Dict[str, int],
    observed: Optional[set] = None,
) -> str:
    observed = observed or set()
    missing: list[str] = []
    if minimums.get("check", 0) > counts.get("check", 0):
        missing.append("run a `novelgen tool check` for the target scope")
    if minimums.get("context", 0) > counts.get("context", 0):
        missing.append("run the required `novelgen tool query context` command")
    if minimums.get("query", 0) > counts.get("query", 0):
        missing.append("run the required `novelgen tool query` command")
    if minimums.get("patch_apply", 0) > counts.get("patch_apply", 0):
        missing.append("apply the validated patch with `--apply`")
    for required in required_commands:
        canonical = canonical_novelgen_command(normalize_command(required))
        if canonical and canonical not in observed:
            missing.append(f"run exactly: {required.strip()}")
    if not missing:
        return ""
    return (
        "Your turn cannot end yet because required tool evidence is missing: "
        + "; ".join(missing)
        + ". Complete the missing tool call(s) now, then return your final answer. "
        "Do not repeat already-completed work."
    )


def denial_guard_instruction(enabled: bool, had_denial: bool, resolved: bool) -> str:
    if not enabled or not had_denial or resolved:
        return ""
    return (
        "One or more of your earlier tool command(s) were denied because they were outside the "
        "workflow allowlist. Before ending the turn, re-run the denied `novelgen tool` command(s) "
        "using only the allowed forms from the skill, then return your final answer."
    )


def combine_guard_instructions(*parts: str) -> str:
    joined = [part for part in parts if part]
    if not joined:
        return ""
    return " ".join(joined)


def tool_timing_key(tool_use_id: Optional[str], command: str) -> str:
    if tool_use_id:
        return f"id:{tool_use_id}"
    return f"command:{canonical_novelgen_command(normalize_command(command))}"


def tool_elapsed_ms(started_at: Dict[str, float], key: str) -> int:
    start = started_at.pop(key, None)
    if start is None:
        return 0
    elapsed = int((_time.monotonic() - start) * 1000)
    if elapsed < 0:
        return 0
    return elapsed


async def single_user_message_stream(prompt: str):
    yield {
        "type": "user",
        "message": {"role": "user", "content": prompt},
        "parent_tool_use_id": None,
        "session_id": "novelgen-agent-runtime",
    }


def is_allowed_tool_call(tool_name: str, tool_input: Dict[str, Any], allowlist: list[str], workspace_root: str = "") -> bool:
    return tool_call_denial_reason(tool_name, tool_input, allowlist, workspace_root) == ""


def tool_call_denial_reason(tool_name: str, tool_input: Dict[str, Any], allowlist: list[str], workspace_root: str = "") -> str:
    tool = str(tool_name or "").lower()
    if "bash" not in tool:
        return f"Only Bash tool calls are allowed for Novelgen workflows; denied {tool_name}."
    command = extract_tool_command(tool_input)
    if not command:
        return "Bash command is empty. Use a concrete novelgen tool query/check/patch/refresh command."
    normalized = normalize_command(command)
    if is_patch_tool_command(normalized) and has_patch_placeholder_payload(normalized):
        return patch_payload_denial_reason()
    if is_patch_tool_command(normalized) and patch_json_payload_contains_raw_non_ascii(command):
        return (
            "Denied raw non-ASCII text inside --patch-json. Pipe compact JSON on stdin instead, "
            "for example: printf '%s' '<compact-json>' | novelgen tool patch ... . "
            "Do not run Python/Node/PowerShell/help commands to encode JSON. "
            "Use --patch-json only for small ASCII-only patches. For long chapter prose, use: "
            "printf '%s' '<content chunk>' | novelgen tool patch-buffer append --id <chapter_id>-draft --stdin."
        )
    if has_forbidden_shell_token(normalized):
        hint = exact_allowed_command_hint(normalized, allowlist)
        if hint:
            return (
                "Denied shell wrapper/redirection around an otherwise allowed Novelgen command. "
                f"Run the exact command without fallback shell syntax: `{hint}`. "
                "Do not add 2>&1, || echo, pipes, redirection, cd, or status probes."
            )
        return (
            "Denied shell syntax or command. Do not read/write temporary files, use redirection, or run general shell commands. "
            "To inspect chapter prose, run novelgen tool query chapter --id <chapter_id> --content --view brief. "
            "Use direct novelgen tool query/check/refresh calls, pass short patch JSON as a literal string pipe, "
            "or append long chapter prose with: printf '%s' '<content chunk>' | novelgen tool patch-buffer append --id <chapter_id>-draft --stdin."
        )
    if workspace_root and not command_stays_in_workspace(command, workspace_root):
        return "Denied cd/chdir outside the project workspace. Run novelgen tool commands from the provided workspace."
    if is_final_json_shell_output_command(normalized):
        return "Denied shell output of final JSON. Return only the final JSON directly as the assistant response; do not use Bash, echo, printf, temp files, or redirection."
    if not contains_allowed_tool_command(normalized):
        return "Only novelgen tool query/check/patch/refresh commands are allowed in this workflow."
    if "novelgen" not in normalized:
        return "Command must invoke novelgen tool query/check/patch/refresh."
    if "--view full" in normalized:
        return "Denied --view full. Use --view brief or --view index, then query a precise ID/name if needed."
    if has_chained_command_after_tool(normalized):
        hint = exact_allowed_command_hint(normalized, allowlist)
        if hint:
            return (
                "Denied chained shell syntax after an otherwise allowed Novelgen command. "
                f"Run the exact command by itself: `{hint}`. "
                "Do not add &&, ||, pipes, echo, redirection, or fallback commands."
            )
        return "Denied chained command after novelgen tool call. Run one novelgen tool command at a time."
    if is_patch_tool_command(normalized) and not patch_command_has_payload(normalized):
        return patch_payload_denial_reason()
    if not allowlist:
        return ""
    canonical = canonical_novelgen_command(normalized)
    normalized_allowlist = [canonical_novelgen_command(normalize_command(item)) for item in allowlist]
    if " --apply" in canonical and not allowlist_allows_apply(canonical, normalized_allowlist):
        return "Denied --apply for this patch target. First dry-run the patch, and only apply when this workflow explicitly allows apply."
    if any(item and canonical.endswith(item) for item in normalized_allowlist):
        return ""
    if target_scoped_allowlist_allows(canonical, normalized_allowlist):
        return ""
    broad_prefixes = broad_allowed_tool_prefixes(normalized_allowlist)
    if broad_prefixes:
        if any(prefix in canonical for prefix in broad_prefixes):
            return ""
        return "Command target is outside the workflow allowlist. Use only the allowed novelgen tool section/target."
    return "Command does not match the workflow allowlist. Use the exact required query/check/patch shape."


def exact_allowed_command_hint(normalized_command: str, allowlist: list[str]) -> str:
    if not allowlist:
        return ""
    canonical_command = canonical_novelgen_command(normalized_command)
    allowlist_items = [
        (canonical_novelgen_command(normalize_command(item)), str(item).strip())
        for item in allowlist
        if item
    ]
    exact_items = [
        (canonical, original)
        for canonical, original in allowlist_items
        if canonical and not is_prefix_allowlist_entry(canonical) and not broad_allowed_tool_prefixes([canonical])
    ]
    matches = [original for canonical, original in exact_items if canonical in canonical_command]
    if len(matches) != 1:
        return ""
    return matches[0]


def allowlist_allows_apply(canonical_command: str, normalized_allowlist: list[str]) -> bool:
    command_requires_refresh = " --refresh-derived" in canonical_command
    command_without_apply = canonical_command.replace(" --apply", "").replace(" --refresh-derived", "")
    for item in normalized_allowlist:
        if " --apply" not in item:
            continue
        item_requires_refresh = " --refresh-derived" in item
        if item_requires_refresh and not command_requires_refresh:
            continue
        allowed_without_apply = item.replace(" --apply", "").replace(" --refresh-derived", "")
        if allowed_without_apply and allowed_without_apply in command_without_apply:
            return True
    return False


def workflow_allows_apply_for_patch_command(command: str, allowlist: list[str]) -> bool:
    if not allowlist:
        return True
    canonical = canonical_novelgen_command(normalize_command(command))
    canonical = re.sub(r" --dry-run(?= |$)", "", canonical)
    if " --apply" not in canonical:
        canonical = canonical.rstrip() + " --apply"
        if patch_apply_refresh_hint(command, allowlist):
            canonical = canonical.rstrip() + " --refresh-derived"
    normalized_allowlist = [canonical_novelgen_command(normalize_command(item)) for item in allowlist]
    return allowlist_allows_apply(canonical, normalized_allowlist)


def patch_apply_refresh_hint(command: str, allowlist: list[str]) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool patch chapter" not in canonical:
        return ""
    normalized_allowlist = [canonical_novelgen_command(normalize_command(item)) for item in allowlist]
    if any("novelgen tool patch chapter" in item and " --apply" in item and " --refresh-derived" in item for item in normalized_allowlist):
        return " --refresh-derived"
    return ""


def is_final_json_shell_output_command(normalized: str) -> bool:
    lowered = normalized.strip().lower()
    if not (lowered.startswith("printf ") or lowered.startswith("echo ")):
        return False
    if "novelgen tool " in lowered:
        return False
    return any(marker in lowered for marker in [
        '"content"',
        "'content'",
        "{content",
        '"chapter_id"',
        "'chapter_id'",
        '"word_count"',
        "'word_count'",
        '"title"',
        "'title'",
    ])


def target_scoped_allowlist_allows(canonical_command: str, normalized_allowlist: list[str]) -> bool:
    for item in normalized_allowlist:
        if story_setup_query_target_matches(canonical_command, item):
            return True
        if chapter_query_target_matches(canonical_command, item):
            return True
        if outline_query_target_matches(canonical_command, item):
            return True
        if log_query_target_matches(canonical_command, item):
            return True
        if outline_check_target_matches(canonical_command, item):
            return True
        if scoped_check_target_matches(canonical_command, item):
            return True
        if scoped_patch_buffer_matches(canonical_command, item):
            return True
        if scoped_patch_target_matches(canonical_command, item):
            return True
    return False


def story_setup_query_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    marker = "novelgen tool query story-setup --type "
    if marker not in allowlist_item or marker not in canonical_command:
        return False
    allowed_type = command_flag_value(allowlist_item, "--type")
    command_type = command_flag_value(canonical_command, "--type")
    if not (allowed_type and command_type and allowed_type == command_type):
        return False
    if " --name" in allowlist_item and not command_flag_value(canonical_command, "--name"):
        return False
    return view_constraint_matches(canonical_command, allowlist_item)


def chapter_query_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    marker = "novelgen tool query chapter"
    if marker not in allowlist_item or marker not in canonical_command:
        return False
    allowed_id = command_flag_value(allowlist_item, "--id")
    command_id = command_flag_value(canonical_command, "--id")
    if not (allowed_id and command_id and allowed_id == command_id):
        return False
    if "--content" in canonical_command and "--content" not in allowlist_item:
        return False
    return view_constraint_matches(canonical_command, allowlist_item)


def outline_query_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    refs_marker = "novelgen tool query outline --type refs"
    if refs_marker in allowlist_item and refs_marker in canonical_command:
        allowed_entity_type = command_flag_value(allowlist_item, "--entity-type")
        command_entity_type = command_flag_value(canonical_command, "--entity-type")
        if allowed_entity_type and command_entity_type and allowed_entity_type != command_entity_type:
            return False
        if " --name" in allowlist_item and not command_flag_value(canonical_command, "--name"):
            return False
        return view_constraint_matches(canonical_command, allowlist_item)

    query_shapes = (
        ("novelgen tool query outline --type volume", "--id"),
        ("novelgen tool query outline --type chapter", "--id"),
        ("novelgen tool query outline --type events", "--chapter-id"),
        ("novelgen tool query outline --type events", "--volume-id"),
    )
    for marker, flag in query_shapes:
        if marker not in allowlist_item or marker not in canonical_command:
            continue
        allowed_id = command_flag_value(allowlist_item, flag)
        command_id = command_flag_value(canonical_command, flag)
        if allowed_id and command_id and allowed_id == command_id:
            return view_constraint_matches(canonical_command, allowlist_item)
    return False


def log_query_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    marker = "novelgen tool query logs"
    if marker not in allowlist_item or marker not in canonical_command:
        return False
    if " --id" in allowlist_item:
        command_id = command_flag_value(canonical_command, "--id")
        if not command_id:
            return False
        allowed_id = command_flag_value(allowlist_item, "--id")
        if allowed_id and allowed_id != command_id:
            return False
        return view_constraint_matches(canonical_command, allowlist_item)
    allowed_type = command_flag_value(allowlist_item, "--type")
    command_type = command_flag_value(canonical_command, "--type")
    if allowed_type and command_type and allowed_type == command_type:
        return view_constraint_matches(canonical_command, allowlist_item)
    return False


def exact_log_brief_query_key(canonical_command: str) -> str:
    marker = "novelgen tool query logs"
    if marker not in canonical_command:
        return ""
    log_id = command_flag_value(canonical_command, "--id")
    if not log_id:
        return ""
    view = command_flag_value(canonical_command, "--view")
    if view != "brief":
        return ""
    return log_id


def view_constraint_matches(canonical_command: str, allowlist_item: str) -> bool:
    allowed_view = command_flag_value(allowlist_item, "--view")
    if not allowed_view:
        return True
    command_view = command_flag_value(canonical_command, "--view")
    return bool(command_view and command_view == allowed_view)


def outline_check_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    marker = "novelgen tool check all --target outline --scope "
    if marker not in allowlist_item or marker not in canonical_command:
        return False
    allowed_scope = command_flag_value(allowlist_item, "--scope")
    command_scope = command_flag_value(canonical_command, "--scope")
    allowed_id = command_flag_value(allowlist_item, "--id")
    command_id = command_flag_value(canonical_command, "--id")
    return bool(allowed_scope and command_scope and allowed_scope == command_scope and allowed_id and command_id and allowed_id == command_id)


def scoped_check_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    marker = "novelgen tool check "
    if marker not in allowlist_item or marker not in canonical_command:
        return False
    allowed_target = command_flag_value(allowlist_item, "--target")
    command_target = command_flag_value(canonical_command, "--target")
    allowed_scope = command_flag_value(allowlist_item, "--scope")
    command_scope = command_flag_value(canonical_command, "--scope")
    allowed_id = command_flag_value(allowlist_item, "--id")
    command_id = command_flag_value(canonical_command, "--id")
    if not target_words_constraint_matches(canonical_command, allowlist_item):
        return False
    if not (allowed_target and command_target and allowed_target == command_target):
        return False
    if not (allowed_scope and command_scope and allowed_scope == command_scope):
        return False
    return bool(allowed_id and command_id and allowed_id == command_id)


def scoped_patch_target_matches(canonical_command: str, allowlist_item: str) -> bool:
    if "novelgen tool patch " not in allowlist_item or "novelgen tool patch " not in canonical_command:
        return False
    if "novelgen tool patch recap" in allowlist_item and "novelgen tool patch recap" in canonical_command:
        allowed_id = command_flag_value(allowlist_item, "--id")
        command_id = command_flag_value(canonical_command, "--id")
        return bool(allowed_id and command_id and allowed_id == command_id)
    if "novelgen tool patch chapter" in allowlist_item and "novelgen tool patch chapter" in canonical_command:
        allowed_id = command_flag_value(allowlist_item, "--id")
        command_id = command_flag_value(canonical_command, "--id")
        if not (allowed_id and command_id and allowed_id == command_id):
            return False
        if not target_words_constraint_matches(canonical_command, allowlist_item):
            return False
        command_buffer = command_flag_value(canonical_command, "--patch-buffer")
        if not command_buffer:
            return True
        allowed_buffer = command_flag_value(allowlist_item, "--patch-buffer")
        return bool(allowed_buffer and allowed_buffer == command_buffer)
    if "novelgen tool patch craft" in allowlist_item and "novelgen tool patch craft" in canonical_command:
        allowed_target = command_flag_value(allowlist_item, "--target")
        command_target = command_flag_value(canonical_command, "--target")
        allowed_id = command_flag_value(allowlist_item, "--id")
        command_id = command_flag_value(canonical_command, "--id")
        return bool(allowed_target and command_target and allowed_target == command_target and allowed_id and command_id and allowed_id == command_id)
    if "novelgen tool patch outline" in allowlist_item and "novelgen tool patch outline" in canonical_command:
        allowed_target = command_flag_value(allowlist_item, "--target")
        command_target = command_flag_value(canonical_command, "--target")
        allowed_id = command_flag_value(allowlist_item, "--id")
        command_id = command_flag_value(canonical_command, "--id")
        return bool(allowed_target and command_target and allowed_target == command_target and allowed_id and command_id and allowed_id == command_id)
    return False


def target_words_constraint_matches(canonical_command: str, allowlist_item: str) -> bool:
    allowed_target_words = command_flag_value(allowlist_item, "--target-words")
    if not allowed_target_words:
        return True
    command_target_words = command_flag_value(canonical_command, "--target-words")
    return bool(command_target_words and command_target_words == allowed_target_words)


def scoped_patch_buffer_matches(canonical_command: str, allowlist_item: str) -> bool:
    if "novelgen tool patch-buffer" not in allowlist_item or "novelgen tool patch-buffer" not in canonical_command:
        return False
    allowed_id = command_flag_value(allowlist_item, "--id")
    command_id = command_flag_value(canonical_command, "--id")
    return bool(allowed_id and command_id and allowed_id == command_id)


def command_flag_value(command: str, flag: str) -> str:
    match = re.search(rf"(?:^| ){re.escape(flag)} ([^ ]+)", command)
    return match.group(1).strip() if match else ""


def command_stays_in_workspace(command: str, workspace_root: str) -> bool:
    cd_target = extract_initial_cd_target(command)
    if not cd_target:
        return True
    return path_is_in_workspace(cd_target, workspace_root)


def extract_initial_cd_target(command: str) -> str:
    match = re.match(
        r"(?is)^\s*(?:cd|chdir)\s+(?:/d\s+)?(?P<quote>['\"]?)(?P<path>.*?)(?P=quote)\s*(?:&&|\Z)",
        command.strip(),
    )
    if not match:
        return ""
    return match.group("path").strip()


def path_is_in_workspace(path: str, workspace_root: str) -> bool:
    root = normalize_workspace_path(workspace_root, "")
    candidate = normalize_workspace_path(path, root)
    return candidate == root or candidate.startswith(root + "/")


def normalize_workspace_path(path: str, base: str) -> str:
    value = path.strip().strip('"').strip("'").replace("\\", "/")
    if re.match(r"^/[a-zA-Z]/", value):
        value = f"{value[1]}:{value[2:]}"
    if re.match(r"^[a-zA-Z]:/", value):
        normalized = posixpath.normpath(value)
    elif base:
        normalized = posixpath.normpath(posixpath.join(base, value))
    else:
        normalized = posixpath.normpath(value)
    return normalized.rstrip("/").lower()


def canonical_novelgen_command(command: str) -> str:
    canonical = command.replace("\\", "/").replace('"', "").replace("'", "")
    canonical = canonical.replace("novelgen.exe", "novelgen")
    return " ".join(canonical.split())


def exact_required_queries(allowlist: list[str]) -> set[str]:
    queries = {
        canonical_novelgen_command(normalize_command(item))
        for item in allowlist
        if item and not is_prefix_allowlist_entry(canonical_novelgen_command(normalize_command(item)))
    }
    if any(item in (
        "novelgen tool query",
        "novelgen tool query story-setup --type search",
        "novelgen tool query story-setup --type core-cast",
        "novelgen tool query story-setup --type storyline",
        "novelgen tool query story-setup --type premise",
        "novelgen tool query story-setup --type resource",
        "novelgen tool query story-setup --type timeline",
        "novelgen tool check",
        "novelgen tool check all --target setup --category",
        "novelgen tool patch outline",
        "novelgen tool patch craft",
        "novelgen tool patch setup",
        "novelgen tool patch recap",
        "novelgen tool patch chapter",
        "novelgen tool patch-buffer",
        "novelgen tool refresh chapter-dsl",
    ) for item in queries):
        return set()
    return queries


def stop_after_required_queries_enabled(allowlist: list[str]) -> bool:
    for item in allowlist:
        canonical = canonical_novelgen_command(normalize_command(item))
        if not canonical:
            continue
        if canonical.startswith("novelgen tool check "):
            return False
        if canonical.startswith("novelgen tool patch "):
            return False
        if canonical.startswith("novelgen tool patch-buffer "):
            return False
        if canonical.startswith("novelgen tool refresh "):
            return False
    return True


def is_prefix_allowlist_entry(item: str) -> bool:
    if item.startswith("novelgen tool query context --type ") and " --view " not in item:
        return True
    if item in story_setup_query_type_prefixes():
        return True
    if item == "novelgen tool check all --target setup --category":
        return True
    if item.startswith("novelgen tool query outline --type volume --id "):
        return True
    if item.startswith("novelgen tool query outline --type chapter --id "):
        return True
    if item.startswith("novelgen tool query outline --type events --chapter-id "):
        return True
    if item.startswith("novelgen tool check all --target outline --scope "):
        return True
    if item.startswith("novelgen tool patch recap --id "):
        return True
    if item.startswith("novelgen tool patch chapter --id "):
        return True
    if item.startswith("novelgen tool patch-buffer --id "):
        return True
    if item.startswith("novelgen tool patch craft --target ") and " --id " in item:
        return True
    if item.startswith("novelgen tool patch outline --target ") and " --id " in item:
        return True
    return False


def matching_required_query(command: str, required_queries: set[str]) -> str:
    if not required_queries:
        return ""
    canonical = canonical_novelgen_command(normalize_command(command))
    for query in required_queries:
        if canonical.endswith(query):
            return query
    return ""


def patch_target_key(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool patch setup" in canonical:
        return "setup:story_setup"
    if "novelgen tool patch recap" in canonical:
        target_id = re.search(r"(?:^| )--id (.*?)(?= --[a-z0-9_-]+(?: |$)|$)", canonical)
        if not target_id:
            return ""
        normalized_id = target_id.group(1).strip()
        if not normalized_id:
            return ""
        return f"recap:{normalized_id}"
    if "novelgen tool patch chapter" in canonical:
        target_id = re.search(r"(?:^| )--id (.*?)(?= --[a-z0-9_-]+(?: |$)|$)", canonical)
        if not target_id:
            return ""
        normalized_id = target_id.group(1).strip()
        if not normalized_id:
            return ""
        return f"chapter:{normalized_id}"
    is_outline_patch = "novelgen tool patch outline" in canonical
    is_craft_patch = "novelgen tool patch craft" in canonical
    if not is_outline_patch and not is_craft_patch:
        return ""
    target = re.search(r"(?:^| )--target (\S+)(?: |$)", canonical)
    target_id = re.search(r"(?:^| )--id (.*?)(?= --[a-z0-9_-]+(?: |$)|$)", canonical)
    if not target or not target_id:
        return ""
    normalized_id = target_id.group(1).strip()
    if not normalized_id:
        return ""
    if is_outline_patch:
        return f"outline-{target.group(1)}:{normalized_id}"
    return f"{target.group(1)}:{normalized_id}"


def patch_buffer_target_key(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool patch-buffer" not in canonical:
        return ""
    buffer_id = extract_flag_value(canonical, "--id")
    if not buffer_id or not buffer_id.endswith("-draft"):
        return ""
    chapter_id = buffer_id[:-len("-draft")]
    if not chapter_id:
        return ""
    return f"chapter:{chapter_id}"


def patch_buffer_is_mutation(command: str) -> bool:
    canonical = canonical_novelgen_command(normalize_command(command))
    return "novelgen tool patch-buffer clear" in canonical or "novelgen tool patch-buffer append" in canonical


def required_chapter_patch_precheck_key(command: str, allowlist: list[str]) -> str:
    patch_key = patch_target_key(command) or patch_buffer_target_key(command)
    if not patch_key.startswith("chapter:"):
        return ""
    target_id = patch_key[len("chapter:") :]
    if not target_id:
        return ""
    for item in allowlist:
        canonical = canonical_novelgen_command(normalize_command(item))
        if "novelgen tool check all --target chapter" not in canonical:
            continue
        if extract_flag_value(canonical, "--scope").lower() != "chapter":
            continue
        if extract_flag_value(canonical, "--id") == target_id:
            return patch_key
    return ""


def check_target_key(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool check all --target outline" in canonical:
        scope = extract_flag_value(canonical, "--scope").lower()
        target_id = extract_flag_value(canonical, "--id")
        if scope in ("volume", "chapter") and target_id:
            return f"outline-{scope}:{target_id}"
    if "novelgen tool check all --target chapter" in canonical:
        scope = extract_flag_value(canonical, "--scope").lower()
        target_id = extract_flag_value(canonical, "--id")
        if scope == "chapter" and target_id:
            return f"chapter:{target_id}"
    return ""


def refresh_target_key(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if not canonical.startswith("novelgen tool refresh chapter-dsl"):
        return ""
    target_id = extract_flag_value(canonical, "--id")
    if target_id:
        return f"chapter:{target_id}"
    return ""


def refresh_target_key_from_patch_apply(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool patch chapter" not in canonical:
        return ""
    if " --apply" not in canonical or " --refresh-derived" not in canonical:
        return ""
    target_id = extract_flag_value(canonical, "--id")
    if target_id:
        return f"chapter:{target_id}"
    return ""


def clean_checked_detail_query_target(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool query context --type outline-global-repair" in canonical:
        view = extract_flag_value(canonical, "--view").lower()
        if view in ("brief", "full"):
            name = extract_flag_value(canonical, "--name") or "all"
            return f"outline-global-repair:{name}"
    if "novelgen tool query context --type outline-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        view = extract_flag_value(canonical, "--view").lower()
        if target_id and view in ("brief", "full"):
            return outline_repair_target_key(target_id)
    if "novelgen tool query context --type chapter-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        view = extract_flag_value(canonical, "--view").lower()
        if target_id and view in ("brief", "full"):
            return f"chapter:{target_id}"
    if "novelgen tool query context --type recap-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        view = extract_flag_value(canonical, "--view").lower()
        if target_id and view in ("brief", "full"):
            return f"recap:{target_id}"
    if "novelgen tool query context --type outline-volume" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        view = extract_flag_value(canonical, "--view").lower()
        if target_id and view in ("brief", "full"):
            return f"outline-volume:{target_id}"
    if "novelgen tool query outline --type volume" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f"outline-volume:{target_id}"
    if "novelgen tool query outline --type chapter" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f"chapter:{target_id}"
    if "novelgen tool query outline --type events" in canonical:
        target_id = extract_flag_value(canonical, "--chapter-id")
        if target_id:
            return f"chapter:{target_id}"
    if "novelgen tool query chapter" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f"chapter:{target_id}"
    if "novelgen tool query context --type craft-" in canonical:
        return "craft-context"
    return ""


def clean_target_is_closed(target_key: str, clean_check_targets: set[str]) -> bool:
    if target_key in clean_check_targets:
        return True
    if target_key.startswith("outline-global-repair:") and "outline-global-repair:*" in clean_check_targets:
        return True
    return False


def outline_repair_target_key(target_id: str) -> str:
    target_id = str(target_id or "").strip()
    if not target_id:
        return ""
    if re.search(r"-c\d+$", target_id.lower()):
        return f"chapter:{target_id}"
    return f"outline-volume:{target_id}"


def context_final_json_target_keys(command: str, input_data: Any) -> set[str]:
    actions = patch_next_action_names(input_data)
    if "return_final_json" not in actions and "focused_check_clean" not in actions:
        return set()
    canonical = canonical_novelgen_command(normalize_command(command))
    keys: set[str] = set()
    if "novelgen tool query context --type outline-global-repair" in canonical:
        name = extract_flag_value(canonical, "--name")
        if name:
            keys.add(f"outline-global-repair:{name}")
        else:
            keys.add("outline-global-repair:*")
    elif "novelgen tool query context --type outline-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        key = outline_repair_target_key(target_id)
        if key:
            keys.add(key)
    elif "novelgen tool query context --type chapter-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            keys.add(f"chapter:{target_id}")
            keys.add("craft-context")
    elif "novelgen tool query context --type recap-repair" in canonical:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            keys.add(f"recap:{target_id}")
    return keys


def check_output_is_clean(input_data: Any) -> bool:
    parsed = patch_next_actions_payload(input_data)
    if not isinstance(parsed, dict):
        return False
    if bool(parsed.get("blocking")):
        return False
    issues = parsed.get("issues")
    if isinstance(issues, list) and len(issues) > 0:
        return False
    summary = parsed.get("summary")
    if isinstance(summary, dict):
        total = summary.get("total")
        if isinstance(total, (int, float)):
            return total == 0
    if "ok" in parsed and isinstance(parsed.get("ok"), bool):
        return bool(parsed.get("ok")) and not bool(parsed.get("blocking")) and not issues
    return False


def clean_check_has_focused_detail_override(check_key: str, allowlist: list[str]) -> bool:
    if not check_key.startswith("outline-volume:"):
        return False
    normalized_allowlist = [canonical_novelgen_command(normalize_command(item)) for item in allowlist]
    return any(
        item.startswith("novelgen tool query outline --type chapter --id ")
        or item.startswith("novelgen tool query outline --type events --chapter-id ")
        for item in normalized_allowlist
    )


def tool_returned_final_json_instruction(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    target = canonical
    return (
        f"The tool output for `{target}` returned next_actions ending in return_final_json or focused_check_clean. "
        "Do not fetch brief/full context, do not run more queries, and do not patch this target. "
        "Return only the workflow JSON now using the existing route/check result."
    )


def clean_check_final_instruction(check_key: str) -> str:
    suffix = " Do not run any more Bash commands, including echo/status/check/query commands."
    if check_key.startswith("outline-volume:"):
        target = check_key.split(":", 1)[1]
        return (
            f"The focused outline volume check for `{target}` returned no issues. "
            "Do not query chapter/event/detail context for this target. Return final JSON now using the existing route/check results."
            + suffix
        )
    if check_key.startswith("outline-chapter:"):
        target = check_key.split(":", 1)[1]
        return (
            f"The focused outline chapter check for `{target}` returned no issues. "
            "Do not query additional outline detail for this target. Return final JSON now using the existing route/check results."
            + suffix
        )
    if check_key.startswith("chapter:"):
        target = check_key.split(":", 1)[1]
        return (
            f"The focused final chapter check for `{target}` returned no issues. "
            "Do not query outline chapter/events or expand context for this target. Return final JSON now using the existing chapter context and check results."
            + suffix
        )
    return "The focused check returned no issues. Do not expand context for this target. Return final JSON now." + suffix


def patch_dry_run_limit(patch_key: str) -> int:
    if patch_key.startswith("recap:"):
        return 3
    return 6


def patch_apply_limit(command: str, patch_key: str) -> int:
    canonical = canonical_novelgen_command(normalize_command(command))
    if "novelgen tool patch outline" in canonical and re.search(r"(?:^| )--target volume(?: |$)", canonical):
        return 1
    return 2


def post_patch_check_command(command: str) -> str:
    canonical = canonical_novelgen_command(" ".join(str(command or "").replace("\\", "/").split()))
    lower = canonical.lower()
    if "novelgen tool patch setup" in lower:
        return "novelgen tool check all --target setup --min-priority medium --max-issues 12"
    if "novelgen tool patch recap" in lower:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f'novelgen tool check quality --target recap --scope chapter --id "{target_id}" --min-priority low --max-issues 8'
        return ""
    if "novelgen tool patch chapter" in lower:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f'novelgen tool check all --target chapter --scope chapter --id "{target_id}" --min-priority low --max-issues 12'
        return ""
    if "novelgen tool patch outline" in lower:
        target = (extract_flag_value(canonical, "--target") or "chapter").lower()
        target_id = extract_flag_value(canonical, "--id")
        if target in ("chapter", "volume") and target_id:
            min_priority = "medium" if target == "volume" else "low"
            max_issues = "12" if target == "volume" else "8"
            return f'novelgen tool check all --target outline --scope {target} --id "{target_id}" --min-priority {min_priority} --max-issues {max_issues}'
        return ""
    if "novelgen tool patch craft" in lower:
        target = extract_flag_value(canonical, "--target").lower()
        target_id = extract_flag_value(canonical, "--id")
        if target and target_id:
            return f'novelgen tool check schema --target craft --scope {target} --id "{target_id}"'
    return ""


def post_patch_refresh_command(command: str) -> str:
    canonical = canonical_novelgen_command(" ".join(str(command or "").replace("\\", "/").split()))
    if " --refresh-derived" in canonical:
        return ""
    lower = canonical.lower()
    if "novelgen tool patch chapter" in lower:
        target_id = extract_flag_value(canonical, "--id")
        if target_id:
            return f'novelgen tool refresh chapter-dsl --id "{target_id}"'
    return ""


def patch_next_action_commands(input_data: Any) -> list[str]:
    parsed = patch_next_actions_payload(input_data)
    if not isinstance(parsed, dict):
        return []
    actions = parsed.get("next_actions")
    if not isinstance(actions, list):
        return []
    commands: list[str] = []
    seen: set[str] = set()
    for action in actions:
        if not isinstance(action, dict):
            continue
        command = str(action.get("command") or "").strip()
        if not command or not is_safe_patch_next_action_command(command):
            continue
        canonical = canonical_novelgen_command(normalize_command(command))
        if canonical in seen:
            continue
        seen.add(canonical)
        commands.append(command)
    return commands


def patch_next_action_names(input_data: Any) -> list[str]:
    parsed = patch_next_actions_payload(input_data)
    if not isinstance(parsed, dict):
        return []
    actions = parsed.get("next_actions")
    if not isinstance(actions, list):
        return []
    names: list[str] = []
    for action in actions:
        if not isinstance(action, dict):
            continue
        name = str(action.get("action") or "").strip()
        if name:
            names.append(name)
    return names


def patch_dry_run_state_from_output(input_data: Any) -> str:
    parsed = patch_next_actions_payload(input_data)
    if not isinstance(parsed, dict):
        return "validated"
    actions = patch_next_action_names(input_data)
    if "repair_patch_content" in actions:
        return "repair_required"
    if "apply_validated_patch" in actions:
        return "validated"
    check = parsed.get("check")
    if isinstance(check, dict) and "blocking" in check:
        return "repair_required" if bool(check.get("blocking")) else "validated"
    if "ok" in parsed and isinstance(parsed.get("ok"), bool):
        return "validated" if parsed.get("ok") else "repair_required"
    return "validated"


def patch_next_actions_payload(input_data: Any) -> Any:
    text = extract_tool_output_text(input_data)
    if not text:
        return None
    return parse_first_json_object(text)


def extract_tool_output_text(input_data: Any) -> str:
    for key in ("tool_response", "tool_result", "result", "output", "stdout", "stderr", "content"):
        value = extract_message_value(input_data, key)
        text = tool_output_value_to_text(value)
        if text:
            return text
    return ""


def tool_output_value_to_text(value: Any) -> str:
    if value in (None, ""):
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        parts = [tool_output_value_to_text(item) for item in value]
        return "\n".join(part for part in parts if part)
    if isinstance(value, dict):
        for key in ("content", "text", "stdout", "stderr", "output", "result"):
            text = tool_output_value_to_text(value.get(key))
            if text:
                return text
        return encode_content(value)
    return str(value)


def parse_first_json_object(text: str) -> Any:
    text = str(text or "").strip()
    if not text:
        return None
    try:
        return json.loads(text)
    except (TypeError, ValueError):
        pass
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end <= start:
        return None
    try:
        return json.loads(text[start:end + 1])
    except (TypeError, ValueError):
        return None


def is_safe_patch_next_action_command(command: str) -> bool:
    canonical = canonical_novelgen_command(normalize_command(command))
    if has_forbidden_shell_token(canonical) or has_chained_command_after_tool(canonical):
        return False
    return canonical.startswith("novelgen tool refresh chapter-dsl") or canonical.startswith("novelgen tool check ")


def format_command_sequence(commands: list[str]) -> str:
    if not commands:
        return "run the focused follow-up check"
    return ", then ".join(f"`{command}`" for command in commands)


def is_followup_allowlist_denial(reason: str) -> bool:
    reason = str(reason or "")
    return reason.startswith("Command target is outside the workflow allowlist") or reason.startswith("Command does not match the workflow allowlist")


def extract_flag_value(command: str, flag: str) -> str:
    pattern = rf"(?:^| ){re.escape(flag)} (.*?)(?= --[a-zA-Z0-9_-]+(?: |$)|$)"
    match = re.search(pattern, command)
    if not match:
        return ""
    return match.group(1).strip().strip('"').strip("'")


def command_uses_apply(command: str) -> bool:
    canonical = canonical_novelgen_command(normalize_command(command))
    return " --apply" in canonical


def patch_command_fingerprint(command: str) -> str:
    canonical = canonical_novelgen_command(normalize_command(command))
    canonical = re.sub(r" --apply(?= |$)", "", canonical)
    canonical = re.sub(r" --dry-run(?= |$)", "", canonical)
    canonical = re.sub(r" --refresh-derived(?= |$)", "", canonical)
    return " ".join(canonical.split())


def is_broad_novelgen_query_allowlist(allowlist: list[str]) -> bool:
    return any(canonical_novelgen_command(normalize_command(item)) == "novelgen tool query" for item in allowlist)


def is_legacy_broad_novelgen_query(normalized: str) -> bool:
    return (
        "novelgen tool query" in normalized
        or "novelgen.exe tool query" in normalized
        or "novelgen.exe' tool query" in normalized
        or 'novelgen.exe" tool query' in normalized
    )


def contains_allowed_tool_command(normalized: str) -> bool:
    return any(prefix in normalized for prefix in [
        "novelgen tool query",
        "novelgen.exe tool query",
        "novelgen.exe' tool query",
        'novelgen.exe" tool query',
        "novelgen tool check",
        "novelgen.exe tool check",
        "novelgen.exe' tool check",
        'novelgen.exe" tool check',
        "novelgen tool patch outline",
        "novelgen.exe tool patch outline",
        "novelgen.exe' tool patch outline",
        'novelgen.exe" tool patch outline',
        "novelgen tool patch craft",
        "novelgen.exe tool patch craft",
        "novelgen.exe' tool patch craft",
        'novelgen.exe" tool patch craft',
        "novelgen tool patch setup",
        "novelgen.exe tool patch setup",
        "novelgen.exe' tool patch setup",
        'novelgen.exe" tool patch setup',
        "novelgen tool patch recap",
        "novelgen.exe tool patch recap",
        "novelgen.exe' tool patch recap",
        'novelgen.exe" tool patch recap',
        "novelgen tool patch chapter",
        "novelgen.exe tool patch chapter",
        "novelgen.exe' tool patch chapter",
        'novelgen.exe" tool patch chapter',
        "novelgen tool patch-buffer",
        "novelgen.exe tool patch-buffer",
        "novelgen.exe' tool patch-buffer",
        'novelgen.exe" tool patch-buffer',
        "novelgen tool refresh chapter-dsl",
        "novelgen.exe tool refresh chapter-dsl",
        "novelgen.exe' tool refresh chapter-dsl",
        'novelgen.exe" tool refresh chapter-dsl',
    ])


def is_utf8_powershell_readonly_wrapper(command: str) -> bool:
    normalized = normalize_command(command)
    if not normalized.startswith(("powershell ", "powershell.exe ", "pwsh ", "pwsh.exe ")):
        return False
    if "[system.text.utf8encoding]::new()" not in normalized:
        return False
    if any(token in normalized for token in [
        " tool patch ",
        " tool patch-buffer ",
        " tool refresh ",
        " get-content ",
        " set-content ",
        " out-file ",
        " add-content ",
    ]):
        return False
    return "novelgen tool query" in normalized or "novelgen tool check" in normalized


def is_patch_tool_command(normalized: str) -> bool:
    return (
        "novelgen tool patch outline" in normalized
        or "novelgen tool patch craft" in normalized
        or "novelgen tool patch setup" in normalized
        or "novelgen tool patch recap" in normalized
        or "novelgen tool patch chapter" in normalized
    )


def patch_command_has_payload(normalized: str) -> bool:
    if re.search(r"(?:^| )--task(?:[ =]|$)", normalized):
        return True
    if re.search(r"(?:^| )--patch-json(?:[ =]|$)", normalized):
        return True
    if re.search(r"(?:^| )--patch-buffer(?:[ =]|$)", normalized):
        return True
    return has_literal_json_pipe_to_patch(normalized)


def patch_json_payload_contains_raw_non_ascii(command: str) -> bool:
    payload = extract_patch_json_payload(command)
    if payload == "":
        return False
    return any(ord(ch) > 127 for ch in payload)


def extract_patch_json_payload(command: str) -> str:
    if not isinstance(command, str) or "--patch-json" not in command.lower():
        return ""
    match = re.search(r"--patch-json(?:=|\s+)(['\"`])(?P<payload>.*?)(?<!\\)\1", command, flags=re.IGNORECASE)
    if match:
        return match.group("payload")
    match = re.search(r"--patch-json(?:=|\s+)(?P<payload>\S+)", command, flags=re.IGNORECASE)
    if match:
        return match.group("payload")
    return ""


def has_patch_placeholder_payload(normalized: str) -> bool:
    if "<json>" in normalized or "<compact-json>" in normalized or "<patch-json>" in normalized:
        return True
    payload = extract_patch_json_payload(normalized)
    if not payload:
        return False
    payload = payload.strip().lower()
    return payload in {"<json>", "<compact-json>", "<patch-json>"} or bool(re.fullmatch(r"<[^>\s]+>", payload))


def patch_payload_denial_reason() -> str:
    return (
        "Patch commands must include real compact JSON, not an empty command or placeholder. "
        "Pipe a literal compact JSON string into the patch tool, for example: "
        """printf '%s' '{"notes":"brief note"}' | novelgen tool patch craft --target character --id "Name". """
        "Use --patch-json only for small ASCII-only patches. For long chapter prose, first append chunks with "
        "printf '%s' '<content chunk>' | novelgen tool patch-buffer append --id <chapter_id>-draft --stdin, then apply --patch-buffer. "
        "Do not use <json>, temp files, Get-Content, shell redirection, or Python/Node/PowerShell/help commands to encode JSON."
    )


def has_literal_json_pipe_to_patch(normalized: str) -> bool:
    markers = ["novelgen tool patch outline", "novelgen tool patch craft", "novelgen tool patch setup", "novelgen tool patch recap", "novelgen tool patch chapter"]
    positions = [normalized.find(marker) for marker in markers if marker in normalized]
    if not positions:
        return False
    prefix = normalized[:min(positions)].strip()
    if prefix.endswith("&"):
        prefix = prefix[:-1].strip()
    if not prefix.endswith("|"):
        return False
    producer = prefix[:-1].strip()
    if ";" in producer:
        producer = producer.rsplit(";", 1)[-1].strip()
    producer = producer.strip('"')
    if producer.startswith("echo "):
        producer = producer[5:].strip()
    elif producer.startswith("printf "):
        producer = producer[7:].strip()
        for fmt in ("'%s'", '"%s"', "`%s`", "%s"):
            if producer.startswith(fmt + " "):
                producer = producer[len(fmt):].strip()
                break
    return producer.startswith(("'{", "\"{", "`{", "{"))


def broad_allowed_tool_prefixes(normalized_allowlist: list[str]) -> list[str]:
    prefixes: list[str] = []
    if "novelgen tool query" in normalized_allowlist:
        prefixes.append("novelgen tool query")
    for item in story_setup_query_type_prefixes():
        if item in normalized_allowlist:
            prefixes.append(item)
    for item in normalized_allowlist:
        if item.startswith("novelgen tool query context --type ") and " --view " not in item:
            prefix = item
            if prefix not in prefixes:
                prefixes.append(prefix)
    if "novelgen tool check" in normalized_allowlist:
        prefixes.append("novelgen tool check")
    if "novelgen tool check all --target outline" in normalized_allowlist:
        prefixes.append("novelgen tool check all --target outline")
    if "novelgen tool check all --target setup" in normalized_allowlist:
        prefixes.append("novelgen tool check all --target setup")
    if "novelgen tool check all --target setup --category" in normalized_allowlist:
        prefixes.append("novelgen tool check all --target setup --category")
    if "novelgen tool patch outline" in normalized_allowlist:
        prefixes.append("novelgen tool patch outline")
    if "novelgen tool patch craft" in normalized_allowlist:
        prefixes.append("novelgen tool patch craft")
    if "novelgen tool patch setup" in normalized_allowlist or "novelgen tool patch setup --apply" in normalized_allowlist:
        prefixes.append("novelgen tool patch setup")
    if "novelgen tool patch recap" in normalized_allowlist or "novelgen tool patch recap --apply" in normalized_allowlist:
        prefixes.append("novelgen tool patch recap")
    if "novelgen tool patch chapter" in normalized_allowlist or "novelgen tool patch chapter --apply" in normalized_allowlist:
        prefixes.append("novelgen tool patch chapter")
    if (
        "novelgen tool patch-buffer" in normalized_allowlist
        or "novelgen tool patch chapter" in normalized_allowlist
        or "novelgen tool patch chapter --apply" in normalized_allowlist
    ):
        prefixes.append("novelgen tool patch-buffer")
    if "novelgen tool refresh chapter-dsl" in normalized_allowlist:
        prefixes.append("novelgen tool refresh chapter-dsl")
    for item in normalized_allowlist:
        if item.startswith("novelgen tool patch outline --target ") or item.startswith("novelgen tool patch craft --target "):
            prefix = item.replace(" --apply", "")
            if prefix not in prefixes:
                prefixes.append(prefix)
    return prefixes


def story_setup_query_type_prefixes() -> set[str]:
    return {
        "novelgen tool query story-setup --type search",
        "novelgen tool query story-setup --type core-cast",
        "novelgen tool query story-setup --type storyline",
        "novelgen tool query story-setup --type premise",
        "novelgen tool query story-setup --type resource",
        "novelgen tool query story-setup --type timeline",
    }


def extract_tool_command(tool_input: Any) -> str:
    if isinstance(tool_input, dict):
        for key in ("command", "cmd", "script"):
            value = tool_input.get(key)
            if isinstance(value, str):
                return value
    return ""


def summarize_live_tool_command(command: str) -> str:
    command = (command or "").strip()
    if not command:
        return ""
    lower = command.lower()
    if is_claude_temp_output_read_command(lower):
        return "powershell Get-Content <claude-temp-tool-output>"
    idx = first_novelgen_tool_index(lower)
    if idx >= 0:
        command = command[idx:].strip()
        lower = command.lower()
    if "--patch-json" in lower:
        idx = lower.find("--patch-json")
        suffix = " --apply" if " --apply" in lower[idx:] else ""
        command = command[:idx].strip() + " --patch-json <json>" + suffix
        lower = command.lower()
    if " tool patch-buffer " in lower and "--text" in lower:
        idx = lower.find("--text")
        command = command[:idx].strip() + " --text <text>"
        lower = command.lower()
    if " tool patch-buffer " in lower and "--stdin" in lower:
        idx = lower.find("--stdin")
        command = command[:idx].strip() + " --stdin <stdin>"
    return clip_live_text(command, 240)


def is_claude_temp_output_read_command(lower_command: str) -> bool:
    return (
        "get-content" in lower_command
        and "\\temp\\claude\\" in lower_command
        and "\\tasks\\" in lower_command
        and ".output" in lower_command
    )


def first_novelgen_tool_index(lower_command: str) -> int:
    best = -1
    for marker in (
        "novelgen tool",
        "novelgen.exe tool",
        "novelgen.exe' tool",
        'novelgen.exe" tool',
    ):
        idx = lower_command.find(marker)
        if idx >= 0 and (best < 0 or idx < best):
            best = idx
    return best


def clip_live_text(value: str, limit: int) -> str:
    value = (value or "").strip()
    if limit <= 0 or len(value) <= limit:
        return value
    return value[:limit] + "..."


def normalize_command(command: str) -> str:
    normalized = " ".join(command.replace("\\", "/").lower().split())
    return normalize_novelgen_cli_path(normalized)


def normalize_novelgen_cli_path(command: str) -> str:
    for marker in [
        '"novelgen"',
        "'novelgen'",
        '"${novelgen_cli_path}"',
        "'${novelgen_cli_path}'",
        "${novelgen_cli_path}",
        '"$novelgen_cli_path"',
        "'$novelgen_cli_path'",
        "$novelgen_cli_path",
        '"$env:novelgen_cli_path"',
        "'$env:novelgen_cli_path'",
        "$env:novelgen_cli_path",
        '"%novelgen_cli_path%"',
        "'%novelgen_cli_path%'",
        "%novelgen_cli_path%",
    ]:
        command = command.replace(marker, "novelgen")
    return command


def has_forbidden_shell_token(command: str) -> bool:
    unquoted = mask_quoted_shell_segments(command)
    forbidden = [
        " remove-item ", " rm ", " del ", " erase ", " rmdir ", " rd ",
        " set-content ", " add-content ", " out-file ", " new-item ",
        " copy-item ", " move-item ", " cp ", " mv ", " mkdir ",
        " go test ", " go build ", " python ", " pip ", " npm ", " git ",
        " get-content ", " cat ", " type ", " invoke-webrequest ", " curl ", " wget ",
        ">", ">>", "<", "| set-content", "| out-file",
    ]
    padded = f" {unquoted} "
    return any(token in padded for token in forbidden)


def mask_quoted_shell_segments(command: str) -> str:
    chars = list(command or "")
    quote = ""
    escaped = False
    for i, ch in enumerate(chars):
        if escaped:
            if quote:
                chars[i] = " "
            escaped = False
            continue
        if ch == "\\" and quote != "'":
            if quote:
                chars[i] = " "
            escaped = True
            continue
        if quote:
            if ch == quote:
                quote = ""
            else:
                chars[i] = " "
            continue
        if ch in ("'", '"', "`"):
            quote = ch
    return "".join(chars)


def has_chained_command_after_tool(command: str) -> bool:
    for marker in ("tool query", "tool check", "tool patch outline", "tool patch craft", "tool patch setup", "tool patch recap", "tool patch chapter", "tool patch-buffer", "tool refresh chapter-dsl"):
        if marker not in command:
            continue
        _, _, tail = command.partition(marker)
        return any(token in tail for token in [";", "&&", "||", "|", "\n", "\r"])
    return False


def anthropic_urls(base_url: str) -> Iterable[str]:
    if base_url.endswith("/messages"):
        yield base_url
    if base_url.endswith("/v1"):
        yield f"{base_url}/messages"
    else:
        yield f"{base_url}/v1/messages"
        yield f"{base_url}/messages"


def parse_anthropic_response(parsed: Dict[str, Any], model: str) -> Dict[str, Any]:
    content = parsed.get("content", "")
    text_parts = []
    if isinstance(content, list):
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                text_parts.append(block.get("text", ""))
    elif isinstance(content, str):
        text_parts.append(content)

    usage = parsed.get("usage") or {}
    input_tokens = int(usage.get("input_tokens") or usage.get("prompt_tokens") or 0)
    output_tokens = int(usage.get("output_tokens") or usage.get("completion_tokens") or 0)
    return {
        "content": "\n".join(part for part in text_parts if part).strip(),
        "model": parsed.get("model") or model,
        "usage": {
            "prompt_tokens": input_tokens,
            "completion_tokens": output_tokens,
            "total_tokens": input_tokens + output_tokens,
        },
    }


def extract_message_text(message: Any) -> str:
    content = getattr(message, "content", None)
    if content is None and isinstance(message, dict):
        content = message.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, dict):
                parts.append(str(block.get("text") or ""))
            else:
                parts.append(str(getattr(block, "text", "") or ""))
        return "\n".join(part for part in parts if part)
    return ""


class LiveLogger:
    """Append JSONL records for the agent run.

    Keeps the file handle open for the run (per-write open/close on Windows is
    slow and turns any transient IO/serialization error into a permanently
    silent logger). Failures are isolated per record: an unserializable payload
    falls back to a repr record, and a transient IO error retries once before
    falling back to stderr for that record. The logger stays alive so later
    events (especially final/error) are never dropped silently.
    """

    def __init__(self, path: Optional[str]) -> None:
        self.path = path
        self._file: Optional[object] = None
        self._disabled = not path
        if self.path:
            try:
                os.makedirs(os.path.dirname(self.path), exist_ok=True)
                self._file = open(self.path, "a", encoding="utf-8")
            except Exception as exc:
                self._disabled = True
                sys.stderr.write(f"[novelgen] live log disabled: {exc}\n")

    def write(self, event: str, payload: Dict[str, Any]) -> None:
        if self._disabled or self._file is None:
            return
        record = {
            "ts": _datetime.datetime.now(_datetime.UTC).isoformat(timespec="milliseconds").replace("+00:00", "Z"),
            "event": event,
            **payload,
        }
        line = self._serialize(record, event, payload)
        if line is None:
            return
        try:
            self._file.write(line)
            self._file.write("\n")
            self._file.flush()
        except Exception as exc:
            # Transient IO errors (antivirus scans, editors, sharing locks)
            # must not permanently silence the run log: reopen once and retry
            # this record, then keep the logger alive for later writes.
            try:
                self._file = open(self.path, "a", encoding="utf-8")
                self._file.write(line)
                self._file.write("\n")
                self._file.flush()
            except Exception as exc2:
                sys.stderr.write(f"[novelgen] live log write failed: {exc2}\n")

    @staticmethod
    def _serialize(record: Dict[str, Any], event: str, payload: Dict[str, Any]) -> Optional[str]:
        try:
            return json.dumps(record, ensure_ascii=False, default=json_default)
        except Exception:
            # A single unserializable record must not kill the log stream:
            # fall back to a repr so the event is still visible.
            try:
                fallback = {
                    "ts": record.get("ts"),
                    "event": event,
                    "fallback": True,
                    "payload_repr": repr(payload),
                }
                return json.dumps(fallback, ensure_ascii=False)
            except Exception:
                return None

    def close(self) -> None:
        if self._file is not None:
            try:
                self._file.close()
            except Exception:
                pass
            self._file = None


def summarize_live_message(message: Any, text: str, structured: Any, result: Any, usage: Any) -> Dict[str, Any]:
    payload: Dict[str, Any] = {
        "message_type": type(message).__name__,
    }
    subtype = extract_message_value(message, "type")
    if subtype not in (None, ""):
        payload["type"] = subtype
    if text:
        payload["text"] = text
    if structured is not None:
        payload["structured_output"] = structured
    if result not in (None, ""):
        payload["result"] = result
    if usage:
        payload["usage"] = normalize_usage(usage)
    return payload


def extract_message_value(message: Any, key: str) -> Any:
    if isinstance(message, dict):
        return message.get(key)
    return getattr(message, key, None)


def encode_content(value: Any) -> str:
    if isinstance(value, str):
        return value
    return json.dumps(value, ensure_ascii=False, default=json_default)


def json_default(value: Any) -> Any:
    if hasattr(value, "model_dump"):
        return value.model_dump()
    if hasattr(value, "dict"):
        return value.dict()
    if hasattr(value, "__dict__"):
        return value.__dict__
    return str(value)


def normalize_usage(usage: Any) -> Dict[str, int]:
    if isinstance(usage, dict):
        input_tokens = int(usage.get("input_tokens") or usage.get("prompt_tokens") or 0)
        output_tokens = int(usage.get("output_tokens") or usage.get("completion_tokens") or 0)
    else:
        input_tokens = int(getattr(usage, "input_tokens", 0) or getattr(usage, "prompt_tokens", 0) or 0)
        output_tokens = int(getattr(usage, "output_tokens", 0) or getattr(usage, "completion_tokens", 0) or 0)
    return {
        "prompt_tokens": input_tokens,
        "completion_tokens": output_tokens,
        "total_tokens": input_tokens + output_tokens,
    }


if __name__ == "__main__":
    raise SystemExit(main())
