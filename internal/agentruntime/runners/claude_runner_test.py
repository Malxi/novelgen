import importlib.util
import json
import os
import sys
import tempfile
import types
import unittest
import unittest.mock
from dataclasses import dataclass


def load_runner():
    path = os.path.join(os.path.dirname(__file__), "claude_runner.py")
    spec = importlib.util.spec_from_file_location("claude_runner", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


@dataclass
class FakeOptions:
    system_prompt: str = ""
    model: str = ""
    max_tokens: int = 0
    max_turns: int = 1
    cli_path: str = ""
    output_format: dict | None = None
    can_use_tool: object | None = None


class FakeMessage:
    def __init__(self, content=None, result=None, structured_output=None, usage=None):
        self.content = content
        self.result = result
        self.structured_output = structured_output
        self.usage = usage


class ClaudeRunnerTests(unittest.IsolatedAsyncioTestCase):
    def test_tool_allowlist_rejects_patch_json_placeholder_payload(self):
        runner = load_runner()
        reason = runner.tool_call_denial_reason(
            "Bash",
            {"command": "novelgen tool patch outline --target volume --id P1-V1 --patch-json <json> --apply"},
            ["novelgen tool patch outline --target volume --id --apply"],
        )
        self.assertIn("placeholder", reason)

    async def test_live_log_records_stream_messages_and_final_output(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        class PermissionResultAllow:
            pass

        class PermissionResultDeny:
            def __init__(self, message=""):
                self.message = message

        fake_types.PermissionResultAllow = PermissionResultAllow
        fake_types.PermissionResultDeny = PermissionResultDeny

        async def fake_query(prompt, options):
            self.assertEqual(options.model, "sonnet")
            self.assertEqual(options.max_turns, 8)
            self.assertEqual(options.output_format, {"type": "json_schema", "schema": {"type": "object"}})
            yield FakeMessage(content=[{"text": "thinking"}], usage={"input_tokens": 1, "output_tokens": 2})
            yield FakeMessage(structured_output={"name": "Lin", "role": "protagonist"})

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                result = await runner.run_with_claude_agent_sdk({
                    "agent_name": "CraftAgent",
                    "command": "generate protagonist craft",
                    "user_prompt": "Generate.",
                    "system_prompt": "System.",
                    "live_log_path": live_log,
                    "tools": ["Bash"],
                    "allowed_tools": ["Bash"],
                    "permission_mode": "dontAsk",
                    "tool_allowlist": ["novelgen tool query"],
                    "output_json_schema": {"type": "object"},
                    "options": {"model": "sonnet", "max_tokens": 100},
                })
                self.assertEqual(json.loads(result["content"])["name"], "Lin")
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertEqual(records[0]["event"], "start")
                self.assertEqual(records[0]["tools"], ["Bash"])
                self.assertIn("cli_path", records[0])
                self.assertEqual(records[0]["allowed_tools"], [])
                self.assertEqual(records[0]["permission_mode"], "default")
                self.assertEqual(records[0]["requested_allowed_tools"], ["Bash"])
                self.assertEqual(records[0]["requested_permission_mode"], "dontAsk")
                self.assertEqual(records[0]["tool_gate"], "can_use_tool+hooks")
                self.assertEqual(records[0]["tool_allowlist"], ["novelgen tool query"])
                self.assertTrue(records[0]["sdk_output_format"])
                self.assertEqual(records[0]["sdk_output_format_schema_mode"], "full")
                self.assertEqual(records[0]["model"], "sonnet")
                self.assertEqual(records[0]["max_turns"], 8)
                self.assertTrue(records[0]["can_use_tool"])
                self.assertTrue(any(record["event"] == "message" for record in records))
                self.assertEqual(records[-1]["event"], "final")
                self.assertEqual(records[-1]["model"], "sonnet")
                self.assertIn("Lin", records[-1]["content"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_large_output_schema_uses_compact_top_level_output_format(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            self.assertEqual(options.output_format, {
                "type": "json_schema",
                "schema": {
                    "type": "object",
                    "properties": {
                        "review_result": {"type": "object", "additionalProperties": True},
                        "volume_patch": {"type": "object", "additionalProperties": True},
                    },
                    "required": ["review_result", "volume_patch"],
                    "additionalProperties": False,
                },
            })
            yield FakeMessage(structured_output={"review_result": {"overall_score": 90}, "volume_patch": {"id": "P1-V1"}})

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                result = await runner.run_with_claude_agent_sdk({
                    "agent_name": "ComposeAgent",
                    "command": "review volume",
                    "user_prompt": "Review.",
                    "system_prompt": "System.",
                    "live_log_path": live_log,
                    "output_json_schema": {
                        "type": "object",
                        "description": "x" * 9000,
                        "properties": {
                            "review_result": {"type": "object", "description": "x" * 9000},
                            "volume_patch": {"type": "object", "description": "x" * 9000},
                        },
                    },
                    "options": {"model": "sonnet"},
                })
                self.assertEqual(json.loads(result["content"])["review_result"]["overall_score"], 90)
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertTrue(records[0]["sdk_output_format"])
                self.assertEqual(records[0]["sdk_output_format_schema_mode"], "compact_top_level")
                self.assertIn("above", records[0]["sdk_output_format_skipped_reason"])
                self.assertGreater(records[0]["output_schema_chars"], 8000)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    async def test_invocation_can_force_compact_output_schema(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            self.assertEqual(options.output_format, {
                "type": "json_schema",
                "schema": {
                    "type": "object",
                    "properties": {
                        "recap": {"type": "object", "additionalProperties": True},
                    },
                    "required": ["recap"],
                    "additionalProperties": False,
                },
            })
            yield FakeMessage(structured_output={"recap": {"chapter_id": "P1-V1-C1"}})

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                result = await runner.run_with_claude_agent_sdk({
                    "agent_name": "RecapAgent",
                    "command": "extract recap",
                    "user_prompt": "Extract.",
                    "system_prompt": "System.",
                    "live_log_path": live_log,
                    "compact_output_schema": True,
                    "output_json_schema": {
                        "type": "object",
                        "properties": {
                            "recap": {
                                "type": "object",
                                "properties": {
                                    "chapter_id": {"type": "string"},
                                    "plot_beats": {"type": "array", "items": {"type": "string"}},
                                },
                            },
                        },
                    },
                    "options": {"model": "sonnet"},
                })
                self.assertEqual(json.loads(result["content"])["recap"]["chapter_id"], "P1-V1-C1")
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertEqual(records[0]["sdk_output_format_schema_mode"], "compact_top_level_forced")
                self.assertIn("compact output schema requested", records[0]["sdk_output_format_skipped_reason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    async def test_disable_sdk_output_format_uses_plain_text_json(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            self.assertIsNone(options.output_format)
            yield FakeMessage(content='{"recap":{"chapter_id":"P1-V2-C1"}}')

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                result = await runner.run_with_claude_agent_sdk({
                    "agent_name": "RecapAgent",
                    "command": "extract recap",
                    "user_prompt": "Extract.",
                    "system_prompt": "System.",
                    "live_log_path": live_log,
                    "disable_sdk_output_format": True,
                    "output_json_schema": {"type": "object"},
                    "options": {"model": "sonnet"},
                })
                self.assertEqual(json.loads(result["content"])["recap"]["chapter_id"], "P1-V2-C1")
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertFalse(records[0]["sdk_output_format"])
                self.assertIn("disabled by invocation", records[0]["sdk_output_format_skipped_reason"])
                self.assertEqual(records[-1]["content"], '{"recap":{"chapter_id":"P1-V2-C1"}}')
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    async def test_sdk_skill_prompt_is_injected_from_add_dirs(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            self.assertIn("SDK WORKFLOW SKILL: novel-tools", options.system_prompt)
            self.assertIn("Use query tool", options.system_prompt)
            yield FakeMessage(structured_output={"name": "Lin"})

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                skill_dir = os.path.join(tmp, "novel-tools")
                os.makedirs(skill_dir)
                with open(os.path.join(skill_dir, "SKILL.md"), "w", encoding="utf-8") as f:
                    f.write("Use query tool")
                live_log = os.path.join(tmp, "live.jsonl")
                result = await runner.run_with_claude_agent_sdk({
                    "agent_name": "ComposeAgent",
                    "command": "generate",
                    "user_prompt": "Generate.",
                    "system_prompt": "System.",
                    "sdk_skills": ["novel-tools"],
                    "add_dirs": [tmp],
                    "live_log_path": live_log,
                    "output_json_schema": {"type": "object"},
                    "options": {"model": "sonnet"},
                })
                self.assertEqual(json.loads(result["content"])["name"], "Lin")
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertEqual(records[0]["loaded_sdk_skills"], ["novel-tools"])
                self.assertEqual(records[0]["missing_sdk_skills"], [])
                self.assertGreater(records[0]["sdk_skill_prompt_chars"], 0)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    async def test_requested_missing_sdk_skill_fails(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            raise AssertionError("query should not run when requested SDK skill is missing")

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                with self.assertRaises(RuntimeError) as raised:
                    await runner.run_with_claude_agent_sdk({
                        "agent_name": "ComposeAgent",
                        "command": "generate",
                        "user_prompt": "Generate.",
                        "system_prompt": "System.",
                        "sdk_skills": ["missing-skill"],
                        "add_dirs": [tmp],
                        "live_log_path": live_log,
                        "output_json_schema": {"type": "object"},
                        "options": {"model": "sonnet"},
                    })
                self.assertIn("missing-skill", str(raised.exception))
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                self.assertEqual(records[0]["event"], "error")
                self.assertEqual(records[0]["missing_sdk_skills"], ["missing-skill"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    async def test_sdk_exception_prefers_streamed_api_error_detail(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")

        async def fake_query(prompt, options):
            yield FakeMessage(content=[{"text": "API Error: 402 Insufficient Balance"}])
            yield FakeMessage(result="API Error: 402 Insufficient Balance")
            raise Exception("Claude Code returned an error result: success")

        fake_sdk.ClaudeAgentOptions = FakeOptions
        fake_sdk.query = fake_query
        previous = sys.modules.get("claude_agent_sdk")
        sys.modules["claude_agent_sdk"] = fake_sdk
        try:
            with tempfile.TemporaryDirectory() as tmp:
                live_log = os.path.join(tmp, "live.jsonl")
                with self.assertRaises(RuntimeError) as raised:
                    await runner.run_with_claude_agent_sdk({
                        "agent_name": "WriteAgent",
                        "command": "improve",
                        "user_prompt": "Improve.",
                        "system_prompt": "System.",
                        "live_log_path": live_log,
                        "output_json_schema": {"type": "object"},
                        "options": {"model": "sonnet"},
                    })
                self.assertIn("API Error: 402 Insufficient Balance", str(raised.exception))
                self.assertNotIn("error result: success", str(raised.exception))
                with open(live_log, "r", encoding="utf-8") as f:
                    records = [json.loads(line) for line in f]
                error_records = [record for record in records if record["event"] == "error"]
                self.assertEqual(error_records[-1]["message"], "API Error: 402 Insufficient Balance")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous

    def test_tool_allowlist_allows_only_approved_novelgen_tools(self):
        runner = load_runner()
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "D:\\Code\\nolvegen\\bin\\novelgen.exe tool query outline --type chapter --id P1-V1-C1"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "\"${NOVELGEN_CLI_PATH}\" tool query context --type craft-character --name \"虫族工虫\" --view brief"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch craft --target character --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "$env:NOVELGEN_CLI_PATH tool check schema --target craft --scope character --id \"虫族工虫\""},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch craft --target character --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "powershell -NoProfile -Command \"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); & 'D:\\Code\\nolvegen\\bin\\novelgen.exe' tool query story-setup\""},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "powershell -NoProfile -Command \"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); & 'novelgen' tool query story-setup\""},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target outline --scope volume --id P1-V1"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target chapter --scope chapter --id P1-V1-C1 --max-issues 8 --min-priority low"},
            ["novelgen tool check all --target chapter --scope chapter --id P1-V1-C1 --max-issues 8"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target chapter --scope chapter --id P1-V1-C2 --max-issues 8 --min-priority low"},
            ["novelgen tool check all --target chapter --scope chapter --id P1-V1-C1 --max-issues 8"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}'"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target volume --id P1-V1 --patch-json '{\"changed_chapters\":[{\"id\":\"P1-V1-C1\",\"summary\":\"fix\"}]}'"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline --target volume"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json '{\"notes\":\"fix\"}'"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch craft"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json '{\"notes\":\"fix\"}'"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"fix\"}'"},
            ["novelgen tool patch setup"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"新的主题\"}'"},
            ["novelgen tool patch setup"],
        ))
        denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"新的主题\"}'"},
            ["novelgen tool patch setup"],
        )
        self.assertIn("Pipe compact JSON on stdin", denial)
        self.assertIn("printf '%s'", denial)
        self.assertIn("Do not run Python/Node/PowerShell/help", denial)
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '%s' '{\"theme\":\"新的主题\"}' | novelgen tool patch setup"},
            ["novelgen tool patch setup"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '%s' '{\"notes\":\"新的备注\"}' | novelgen tool patch craft --target character --id 林野"},
            ["novelgen tool patch craft"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": r"novelgen tool patch setup --patch-json '{\"theme\":\"\u65b0\u7684\u4e3b\u9898\"}'"},
            ["novelgen tool patch setup"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id 林野 --patch-json '{\"notes\":\"fix\"}'"},
            ["novelgen tool patch craft"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}'"},
            ["novelgen tool patch recap"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}'"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer clear --id P1-V1-C1-draft"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer append --id P1-V1-C1-draft --text '# Opening\\n\\nLin repairs the scene.'"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '%s' '# Opening\\n\\nLin repairs the scene.' | novelgen tool patch-buffer append --id P1-V1-C1-draft"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '%s' '# Opening\\n\\nLin repairs the scene.' | novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply --refresh-derived"},
            ["novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply --refresh-derived"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool refresh chapter-dsl --id P1-V1-C1"},
            ["novelgen tool refresh chapter-dsl"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool refresh chapter-dsl --id P1-V1-C1"},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query logs --id "prompts/ComposeAgent_20260620_135538.md" --content --view brief'},
            ["novelgen tool query logs --id"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query logs --type agent-live --view index --limit 5"},
            ["novelgen tool query logs --type agent-live --view index"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query logs --id "prompts/ComposeAgent_20260620_135538.md" --content --view brief ; Get-Content logs\\prompts\\x.md'},
            ["novelgen tool query logs --id"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer append --id P1-V1-C1-draft --text '# Opening' ; Get-Content story\\compose\\outline.json"},
            ["novelgen tool patch chapter"],
        ))

    def test_scoped_chapter_patch_allowlist_restricts_patch_buffer_id(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
            'novelgen tool patch chapter --id "P1-V1-C1" --apply',
            'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply',
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer clear --id P1-V1-C1-draft"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer append --id P1-V1-C1-draft --text '# Opening'"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch-buffer append --id other-draft --text '# Opening'"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\"}'"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer other-draft --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C2 --patch-json '{\"content\":\"# Opening\"}'"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "'{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' | novelgen tool patch recap --id P1-V1-C1"},
            ["novelgen tool patch recap"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup"},
            ["novelgen tool patch setup"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1"},
            ["novelgen tool patch recap"],
        ))

    def test_scoped_chapter_allowlist_requires_target_words_when_specified(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool check all --target chapter --scope chapter --id "P1-V2-C2" --min-priority low --max-issues 12 --target-words 1200',
            'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft" --target-words 1200',
            'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft" --apply --refresh-derived --target-words 1200',
        ]
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V2-C2" --min-priority low --max-issues 12'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V2-C2" --min-priority low --max-issues 12 --target-words 1200'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft"'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft" --target-words 1200'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft" --apply --refresh-derived'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch chapter --id "P1-V2-C2" --patch-buffer "P1-V2-C2-draft" --apply --refresh-derived --target-words 1200'},
            allowlist,
        ))

    def test_scoped_chapter_patch_apply_can_require_refresh_derived(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
            'novelgen tool patch chapter --id "P1-V1-C1"',
            'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"',
            'novelgen tool patch chapter --id "P1-V1-C1" --apply --refresh-derived',
            'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\"}'"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\"}' --apply --refresh-derived"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply --refresh-derived"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1"},
            ["novelgen tool patch chapter"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "'{\"notes\":\"fix\"}' | novelgen tool patch craft --target character --id Lin"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "echo '{\"notes\":\"fix\"}' | novelgen tool patch craft --target character --id Lin"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type refs --entity-type character --name \"李攸\" --view brief"},
            ["novelgen tool query outline --type refs --entity-type character --name"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type refs --entity-type item --name \"李攸\" --view brief"},
            ["novelgen tool query outline --type refs --entity-type character --name"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type refs --entity-type character --view brief"},
            ["novelgen tool query outline --type refs --entity-type character --name"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --type core-cast --name 林野 --view brief"},
            ["novelgen tool query story-setup --type core-cast --name"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --type storyline --name 林野 --view brief"},
            ["novelgen tool query story-setup --type core-cast --name"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --type core-cast --view brief"},
            ["novelgen tool query story-setup --type core-cast --name"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '%s' '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' | novelgen tool patch chapter --id P1-V1-C1"},
            ["novelgen tool patch chapter"],
        ))
        final_json_denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": "printf '%s' '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}'"},
            ["novelgen tool query context --type chapter-write --id P1-V1-C1 --view brief"],
        )
        self.assertIn("Return only the final JSON directly as the assistant response", final_json_denial)
        metadata_json_denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": "printf '%s' '{\"chapter_id\":\"P1-V1-C1\",\"title\":\"Opening\",\"word_count\":800}'"},
            ["novelgen tool query context --type chapter-write --id P1-V1-C1 --view brief"],
        )
        self.assertIn("Return only the final JSON directly as the assistant response", metadata_json_denial)
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf \"%s\" '{\"notes\":\"fix\"}' | novelgen tool patch craft --target character --id Lin"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "powershell -NoProfile -Command \"$OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::InputEncoding = [System.Text.UTF8Encoding]::new(); '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' | & 'novelgen' tool patch chapter --id P1-V1-C1\""},
            ["novelgen tool patch chapter"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "$OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::InputEncoding = [System.Text.UTF8Encoding]::new(); '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' | novelgen tool patch chapter --id P1-V1-C1"},
            ["novelgen tool patch chapter"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "Get-Content patch.json | novelgen tool patch craft --target character --id Lin"},
            ["novelgen tool patch craft --target character"],
        ))
        type_denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": "type chapters\\chapter-P1-V1-C1.md"},
            ["novelgen tool query chapter"],
        )
        self.assertIn("novelgen tool query chapter --id <chapter_id> --content --view brief", type_denial)
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin < /tmp/patch.json"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json <json>"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertIn(
            "real compact JSON",
            runner.tool_call_denial_reason(
                "Bash",
                {"command": "novelgen tool patch craft --target character --id Lin --patch-json <json>"},
                ["novelgen tool patch craft --target character"],
            ),
        )
        self.assertIn(
            "real compact JSON",
            runner.tool_call_denial_reason(
                "Bash",
                {"command": "novelgen tool patch craft --target character --id Lin"},
                ["novelgen tool patch craft --target character"],
            ),
        )
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target item --id Core --patch-json '{\"notes\":\"fix\"}'"},
            ["novelgen tool patch craft --target character"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json '{\"notes\":\"fix\"}' --apply"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch craft"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json '{\"notes\":\"fix\"}' --apply"},
            ["novelgen tool query", "novelgen tool patch craft --target character --apply"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target item --id Core --patch-json '{\"notes\":\"fix\"}' --apply"},
            ["novelgen tool query", "novelgen tool patch craft --target character --apply"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id Lin --patch-json '{\"notes\":\"fix\"}' && Get-Content story\\craft\\characters.json"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch craft"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}'"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline --target volume"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}' --apply"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target volume --id P1-V1 --patch-json '{\"changed_chapters\":[{\"id\":\"P1-V1-C1\",\"summary\":\"fix\"}]}' --apply"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline --target volume"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"fix\"}' --apply"},
            ["novelgen tool patch setup"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' --apply"},
            ["novelgen tool patch recap"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"fix\"}' --apply"},
            ["novelgen tool patch setup --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' --apply"},
            ["novelgen tool patch recap --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' --apply"},
            ["novelgen tool patch chapter --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
            ["novelgen tool patch chapter --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}'"},
            ["novelgen tool patch chapter --apply"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target volume --id P1-V1 --patch-json '{\"changed_chapters\":[{\"id\":\"P1-V1-C1\",\"summary\":\"fix\"}]}' --apply"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline --target volume --apply"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' --apply"},
            ["novelgen tool patch chapter"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "D:\\Code\\nolvegen\\bin\\novelgen.exe compose gen"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "D:\\Code\\nolvegen\\bin\\novelgen.exe tool query outline; Get-Content story\\compose\\outline.json"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view full"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view brief"},
            ['novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view full"},
            ['novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1"},
            ['novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view brief 2>&1"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view brief 2>&1"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        )
        self.assertIn("redirection", denial)
        self.assertIn("direct novelgen tool", denial)
        focused_denial = runner.tool_call_denial_reason(
            "Bash",
            {"command": 'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief 2>&1 || echo "CONTEXT_UNAVAILABLE"'},
            ['novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief'],
        )
        self.assertIn('novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief', focused_denial)
        self.assertIn("Do not add 2>&1", focused_denial)
        self.assertNotIn("patch-buffer", focused_denial)
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --type chapter --id P1-V1-C1 --view brief > out.txt"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "printf '{\"long_form_plan\":{\"main_loop\":\"探索 -> 遭遇 -> 升级\"}}' | novelgen tool patch setup"},
            ["novelgen tool patch setup"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}' && Get-Content story\\compose\\outline.json"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "Remove-Item story\\compose\\outline.json"},
            ["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"],
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "cd \"D:\\Code\\nolvegen\" && \"D:\\Code\\nolvegen\\bin\\novelgen.exe\" tool query story-setup"},
            ["novelgen tool query story-setup"],
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "cd \"D:\\Code\\nolvegen\" && \"D:\\Code\\nolvegen\\bin\\novelgen.exe\" tool query story-setup"},
            ["novelgen tool query story-setup"],
            "D:\\Code\\nolvegen\\books\\fire-galaxy",
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "cd \"D:\\Code\\nolvegen\\books\\fire-galaxy\" && \"D:\\Code\\nolvegen\\bin\\novelgen.exe\" tool query story-setup"},
            ["novelgen tool query story-setup"],
            "D:\\Code\\nolvegen\\books\\fire-galaxy",
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "cd \"story\" && novelgen tool query story-setup"},
            ["novelgen tool query story-setup"],
            "D:\\Code\\nolvegen\\books\\fire-galaxy",
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query chapter --id P1-V1-C1"},
            ["novelgen tool query story-setup"],
        ))

    def test_tool_allowlist_mixes_exact_recap_queries_with_patch_apply(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief',
            'novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8',
            'novelgen tool patch recap --id "P1-V1-C1" --apply',
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}'"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch recap --id P1-V1-C2 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief --fields current'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query chapter --id "P1-V1-C1" --fields recap --view brief'},
            allowlist,
        ))

    def test_tool_allowlist_uses_exact_craft_context_queries(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool query context --type craft-character --name "Lin" --view brief',
            'novelgen tool check schema --target craft --scope character --id "Lin"',
            'novelgen tool patch craft --target character --id "Lin" --apply',
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type craft-character --name "Lin" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check schema --target craft --scope character --id "Lin"'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id \"Lin\" --patch-json '{\"notes\":\"fix\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch craft --target character --id \"Other\" --patch-json '{\"notes\":\"fix\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query story-setup --type search --name "Lin" --view brief'},
            allowlist,
        ))
    def test_tool_allowlist_allows_limited_write_context_prefixes(self):
        runner = load_runner()
        allowlist = [
            'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief',
            'novelgen tool query context --type chapter-repair --id "P1-V1-C1"',
            "novelgen tool query context --type craft-character",
            "novelgen tool query context --type craft-item",
            'novelgen tool query chapter --id "P1-V1-C1" --content',
            'novelgen tool query chapter --id "P1-V1-C1" --content --view brief',
            'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12',
            "novelgen tool patch chapter --apply",
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name "logic" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query chapter --id "P1-V1-C1" --content --fields content'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query chapter --id "P1-V1-C1" --content --fields content --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query chapter --id "P1-V1-C2" --content --fields content'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type craft-character --name "陆沉" --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query story-setup --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief'},
            allowlist,
        ))

    def test_tool_allowlist_scopes_craft_patch_by_id(self):
        runner = load_runner()
        allowlist = ['novelgen tool patch craft --target character --id "Lin" --apply']
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch craft --target character --id "Lin" --patch-json \'{"notes":"fix"}\''},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch craft --target character --id "Lin" --patch-json \'{"notes":"fix"}\' --apply'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch craft --target character --id "Other" --patch-json \'{"notes":"fix"}\' --apply'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch craft --target item --id "Lin" --patch-json \'{"notes":"fix"}\' --apply'},
            allowlist,
        ))

    def test_tool_allowlist_allows_setup_only_prefixes(self):
        runner = load_runner()
        allowlist = [
            "novelgen tool query story-setup --view brief",
            "novelgen tool query story-setup --type search",
            "novelgen tool query story-setup --type core-cast",
            "novelgen tool query story-setup --type storyline",
            "novelgen tool query story-setup --type premise",
            "novelgen tool check all --target setup --min-priority medium --max-issues 12",
            "novelgen tool check all --target setup --category",
            "novelgen tool patch setup --apply",
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --view brief"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query story-setup --type search --name "theme" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --type core-cast --view brief"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query story-setup --type storyline --name "Main" --view index'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target setup --category theme --min-priority low --max-issues 12"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool patch setup --patch-json '{\"theme\":\"specific\"}' --apply"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query outline --view brief"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target outline --scope chapter --id P1-V1-C1"},
            allowlist,
        ))

    def test_tool_allowlist_allows_target_scoped_global_checks(self):
        runner = load_runner()
        allowlist = [
            "novelgen tool check all --target outline",
            "novelgen tool check all --target setup",
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target outline --category faction_tier --min-priority low --max-issues 12"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target setup --category theme --min-priority low --max-issues 12"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --category faction_tier --min-priority low --max-issues 12"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool check all --target chapter --scope chapter --id P1-V1-C1"},
            allowlist,
        ))

    def test_tool_allowlist_allows_targeted_compose_outline_prefixes(self):
        runner = load_runner()
        allowlist = [
            "novelgen tool query context --type outline-volume --id P1-V2 --view index",
            "novelgen tool query context --type outline-volume --id P1-V2",
            'novelgen tool query outline --type volume --id "P1-V2"',
            'novelgen tool query outline --type chapter --id "P1-V2-C1"',
            'novelgen tool query outline --type events --chapter-id "P1-V2-C1"',
            'novelgen tool query outline --type events --volume-id "P1-V2"',
            "novelgen tool query context --type outline-repair --id P1-V2 --name",
            "novelgen tool query context --type outline-repair --id P1-V2-C1 --name",
            'novelgen tool check all --target outline --scope volume --id "P1-V2"',
            'novelgen tool check all --target outline --scope chapter --id "P1-V2-C1"',
            'novelgen tool patch outline --target volume --id "P1-V2" --apply',
        ]
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query context --type outline-volume --id P1-V2 --view index"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query context --type outline-volume --id P1-V2 --view brief"},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type outline-repair --id "P1-V2-C1" --name "logic" --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type chapter --id P1-V2-C1 --fields summary --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type events --chapter-id P1-V2-C1 --fields result,details --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type events --volume-id P1-V2 --fields action,actor,target --view brief'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check all --target outline --scope chapter --id "P1-V2-C1" --category logic --min-priority low --max-issues 8'},
            allowlist,
        ))
        self.assertTrue(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool patch outline --target volume --id P1-V2 --patch-json "{\"changed_chapters\":[]}" --apply'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query context --type outline-volume --id P1-V3 --view brief"},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query context --type outline-repair --id "P1-V3-C1" --name "logic" --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type chapter --id P1-V2-C10 --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type events --chapter-id P1-V2-C10 --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool query outline --type events --volume-id P1-V3 --view brief'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check all --target outline --scope chapter --id "P1-V3-C1" --category logic'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": 'novelgen tool check all --target outline --scope chapter --id P1-V2-C10 --category logic'},
            allowlist,
        ))
        self.assertFalse(runner.is_allowed_tool_call(
            "Bash",
            {"command": "novelgen tool query story-setup --view brief"},
            allowlist,
        ))

    def test_post_patch_check_command_preserves_targets(self):
        runner = load_runner()
        cases = [
            (
                'novelgen tool patch chapter --id P1-V1-C1 --patch-json "{}" --apply',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12',
            ),
            (
                'novelgen tool patch outline --target volume --id P1-V1 --patch-json "{}" --apply',
                'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12',
            ),
            (
                'novelgen tool patch craft --target character --id "Lin" --patch-json "{}" --apply',
                'novelgen tool check schema --target craft --scope character --id "Lin"',
            ),
            (
                'novelgen tool patch recap --id P1-V1-C1 --patch-json "{}" --apply',
                'novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8',
            ),
            (
                'novelgen tool patch setup --patch-json "{}" --apply',
                'novelgen tool check all --target setup --min-priority medium --max-issues 12',
            ),
        ]
        for command, want in cases:
            self.assertEqual(runner.post_patch_check_command(command), want)

    def test_post_patch_refresh_command_preserves_chapter_target(self):
        runner = load_runner()
        self.assertEqual(
            runner.post_patch_refresh_command('novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft --apply'),
            'novelgen tool refresh chapter-dsl --id "P1-V1-C1"',
        )
        self.assertEqual(
            runner.post_patch_refresh_command('novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json "{}" --apply'),
            "",
        )

    def test_patch_next_action_commands_extracts_safe_refresh_and_check_only(self):
        runner = load_runner()
        payload = {
            "next_actions": [
                {"action": "refresh_derived_dsl", "command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C1"'},
                {"action": "post_refresh_check", "command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                {"action": "bad_patch", "command": 'novelgen tool patch chapter --id "P1-V1-C2" --apply'},
                {"action": "bad_shell", "command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" | cat'},
            ],
        }
        got = runner.patch_next_action_commands({"tool_response": json.dumps(payload)})
        self.assertEqual(got, [
            'novelgen tool refresh chapter-dsl --id "P1-V1-C1"',
            'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8',
        ])

    def test_patch_dry_run_state_falls_back_to_check_blocking(self):
        runner = load_runner()
        self.assertEqual(
            runner.patch_dry_run_state_from_output({"tool_response": json.dumps({"check": {"blocking": False}})}),
            "validated",
        )
        self.assertEqual(
            runner.patch_dry_run_state_from_output({"tool_response": json.dumps({"check": {"blocking": True}})}),
            "repair_required",
        )
        self.assertEqual(
            runner.patch_dry_run_state_from_output({"tool_response": json.dumps({"ok": False})}),
            "repair_required",
        )
        self.assertEqual(
            runner.patch_dry_run_state_from_output({"tool_response": ""}),
            "validated",
        )

    def test_check_output_is_clean_requires_zero_issues(self):
        runner = load_runner()
        self.assertTrue(runner.check_output_is_clean({
            "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
        }))
        self.assertFalse(runner.check_output_is_clean({
            "tool_response": json.dumps({"blocking": False, "summary": {"total": 1}, "issues": [{"priority": "low"}]}),
        }))
        self.assertFalse(runner.check_output_is_clean({
            "tool_response": json.dumps({"blocking": True, "summary": {"total": 0}, "issues": []}),
        }))

    def test_patch_command_fingerprint_ignores_apply_and_dry_run_flags(self):
        runner = load_runner()
        dry_run = 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer draft --dry-run'
        apply = "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft --apply"
        apply_refresh = "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft --apply --refresh-derived"
        other = "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer other --apply"
        self.assertEqual(runner.patch_command_fingerprint(dry_run), runner.patch_command_fingerprint(apply))
        self.assertEqual(runner.patch_command_fingerprint(dry_run), runner.patch_command_fingerprint(apply_refresh))
        self.assertNotEqual(runner.patch_command_fingerprint(dry_run), runner.patch_command_fingerprint(other))

    async def test_tool_hooks_allow_only_approved_novelgen_tools(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool query", "novelgen tool check", "novelgen tool patch outline"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            allowed = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query story-setup"},
            }, None, None)
            extra = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool check quality --target outline --scope chapter --id P1-V1-C1"},
            }, None, None)
            patch = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}'"},
            }, None, None)
            patch_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix\"}'"},
            }, None, None)
            patch_again = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix again\"}'"},
            }, None, None)
            patch_third = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix third\"}'"},
            }, None, None)
            patch_fourth = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix fourth\"}'"},
            }, None, None)
            patch_fifth = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix fifth\"}'"},
            }, None, None)
            patch_sixth = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix sixth\"}'"},
            }, None, None)
            patch_seventh = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target chapter --id P1-V1-C1 --patch-json '{\"summary\":\"fix seventh\"}'"},
            }, None, None)
            denied = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query outline; Remove-Item story\\compose\\outline.json"},
            }, None, None)
            self.assertEqual(allowed["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(extra["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(patch["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertIn("--apply", patch_context["hookSpecificOutput"]["additionalContext"])
            self.assertIn("tool check", patch_context["hookSpecificOutput"]["additionalContext"])
            self.assertIn("novelgen tool check all --target outline --scope chapter --id \"P1-V1-C1\"", patch_context["hookSpecificOutput"]["additionalContext"])
            self.assertEqual(patch_again["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("successful dry-run", patch_again["hookSpecificOutput"]["permissionDecisionReason"])
            self.assertEqual(patch_third["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(patch_fourth["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(patch_fifth["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(patch_sixth["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(patch_seventh["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_allow_post_apply_followup_check_repeat(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief',
                'novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8',
                'novelgen tool patch recap --id "P1-V1-C1" --apply',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief'},
            }
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8'},
            }
            patch_apply = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}' --apply"},
            }
            patch_dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch recap --id P1-V1-C1 --patch-json '{\"last_line\":\"fix\",\"next_opening_hint\":\"fix continues\"}'"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }

            self.assertEqual((await pre_hook(query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(query, None, None)
            self.assertEqual((await pre_hook(query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")

            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(check, None, None)
            self.assertEqual((await pre_hook(patch_dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(patch_dry_run, None, None)
            self.assertEqual((await pre_hook(patch_apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(patch_apply, None, None)
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_mark_workflow_denials(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types

        class FakeLiveLog:
            def __init__(self):
                self.records = []

            def write(self, event, payload):
                self.records.append((event, payload))

        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            live_log = FakeLiveLog()
            hooks = runner.build_tool_hooks(["novelgen tool patch outline"], live_log)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            patch = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch outline --target volume --id P1-V1 --patch-json '{\"summary\":\"fix\"}'"},
            }
            await pre_hook(patch, None, None)
            await post_hook(patch, None, None)
            denied = await pre_hook(patch, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("successful dry-run", denied["hookSpecificOutput"]["permissionDecisionReason"])
            denied_records = [
                payload
                for event, payload in live_log.records
                if event == "tool_hook"
                and payload.get("hook") == "PreToolUse"
                and not payload.get("allowed")
            ]
            self.assertEqual(len(denied_records), 1)
            self.assertTrue(denied_records[0].get("workflow_denial"))
            allowed_records = [
                payload
                for event, payload in live_log.records
                if event == "tool_hook"
                and payload.get("hook") == "PreToolUse"
                and payload.get("allowed")
            ]
            self.assertEqual(len(allowed_records), 1)
            self.assertFalse(allowed_records[0].get("workflow_denial"))
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_guard_blocks_until_denial_resolved(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types

        class FakeLiveLog:
            def __init__(self):
                self.records = []

            def write(self, event, payload):
                self.records.append((event, payload))

        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            live_log = FakeLiveLog()
            allowlist = ['novelgen tool check all --target outline --scope volume --id "P1-V3"']
            hooks = runner.build_tool_hooks(
                allowlist,
                live_log,
                tool_evidence={"require_no_denied_tools": True},
            )
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            stop_hook = hooks["Stop"][0].hooks[0]
            denied_command = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query outline --type chapter --id P1-V3-C1 --view brief"},
            }
            denied = await pre_hook(denied_command, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            blocked = await stop_hook(None, None, None)
            self.assertEqual(blocked["hookSpecificOutput"]["decision"], "block")
            self.assertIn("denied", blocked["hookSpecificOutput"]["reason"])

            allowed_command = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope volume --id "P1-V3" --min-priority low --max-issues 12'},
            }
            allowed = await pre_hook(allowed_command, None, None)
            self.assertEqual(allowed["hookSpecificOutput"]["permissionDecision"], "allow")
            passed = await stop_hook(None, None, None)
            self.assertEqual(passed, {})
            guard_records = [
                payload
                for event, payload in live_log.records
                if event == "stop_guard"
            ]
            self.assertEqual(len(guard_records), 2)
            self.assertEqual(guard_records[0]["decision"], "block")
            self.assertFalse(guard_records[0]["denials_resolved"])
            self.assertEqual(guard_records[1]["decision"], "allow")
            self.assertTrue(guard_records[1]["denials_resolved"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_hook_safe_records_errors_and_returns_fallback(self):
        runner = load_runner()

        class FakeLiveLog:
            def __init__(self):
                self.records = []

            def write(self, event, payload):
                self.records.append((event, payload))

        async def boom(input_data, tool_use_id, context):
            raise RuntimeError("hook exploded")

        live_log = FakeLiveLog()
        wrapped = runner.hook_safe("Stop", live_log, {}, boom)
        result = await wrapped(None, None, None)
        self.assertEqual(result, {})
        self.assertEqual(len(live_log.records), 1)
        event, payload = live_log.records[0]
        self.assertEqual(event, "hook_error")
        self.assertEqual(payload["hook"], "Stop")
        self.assertIn("hook exploded", payload["error"])

        # A non-crashing callback passes through untouched.
        async def fine(input_data, tool_use_id, context):
            return {"ok": True}

        wrapped_ok = runner.hook_safe("Stop", live_log, {}, fine)
        self.assertEqual(await wrapped_ok(None, None, None), {"ok": True})
        self.assertEqual(len(live_log.records), 1)

    async def test_tool_hooks_limit_exact_history_log_briefs(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool query logs --view index --limit 5',
                'novelgen tool query logs --id',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            first = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query logs --id "prompts/WriteAgent_a.md" --view brief'},
            }
            second = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query logs --id "responses/WriteAgent_b.md" --view brief'},
            }

            self.assertEqual((await pre_hook(first, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(first, None, None)
            denied = await pre_hook(second, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("exact history log brief", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_deny_redundant_refresh_after_refresh_derived_apply(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool patch-buffer clear --key "P1-V1-C1-draft"',
                'novelgen tool patch-buffer append --key "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
                'novelgen tool refresh chapter-dsl --id "P1-V1-C1"',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"'},
                "tool_response": json.dumps({"ok": True, "next_actions": [{"action": "apply_validated_patch"}]}),
            }
            apply = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived'},
                "tool_response": json.dumps({"ok": True, "next_actions": [{"action": "return_final_json"}]}),
            }
            refresh = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C1"'},
            }
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12'},
            }

            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(check, None, None)
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            self.assertEqual((await pre_hook(apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            apply_post = await post_hook(apply, None, None)
            self.assertIn("Do not run more tools", apply_post["hookSpecificOutput"]["additionalContext"])
            self.assertNotIn("run `novelgen tool check", apply_post["hookSpecificOutput"]["additionalContext"])
            denied = await pre_hook(refresh, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("--refresh-derived", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_require_chapter_check_before_patch_buffer(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --view brief',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12',
                'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"',
                'novelgen tool patch-buffer append --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            patch_clear = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"'},
            }
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12'},
            }

            denied = await pre_hook(patch_clear, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("required focused chapter check", denied["hookSpecificOutput"]["permissionDecisionReason"])
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            post_check = await post_hook(check, None, None)
            self.assertIn("explicit edit request is still pending", post_check["hookSpecificOutput"]["additionalContext"])
            self.assertEqual((await pre_hook(patch_clear, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_limit_outline_volume_to_one_apply_cycle(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool patch outline --target volume --id "P1-V1" --apply',
                'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "printf '%s' '{\"summary\":\"one\"}' | novelgen tool patch outline --target volume --id \"P1-V1\""},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            apply = {
                "tool_name": "Bash",
                "tool_input": {"command": "printf '%s' '{\"summary\":\"one\"}' | novelgen tool patch outline --target volume --id \"P1-V1\" --apply"},
            }
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12'},
                "tool_response": json.dumps({"blocking": True, "summary": {"total": 1}, "issues": [{"priority": "high"}]}),
            }

            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            repeat_dry_run = await pre_hook(dry_run, None, None)
            self.assertEqual(repeat_dry_run["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("successful dry-run", repeat_dry_run["hookSpecificOutput"]["permissionDecisionReason"])
            self.assertEqual((await pre_hook(apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(apply, None, None)
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(check, None, None)
            denied = await pre_hook(dry_run, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("Return final JSON", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_limit_recap_patch_dry_runs_to_three(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch recap --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]

            async def dry_run(i: int):
                return await pre_hook({
                    "tool_name": "Bash",
                    "tool_input": {"command": f"novelgen tool patch recap --id P1-V1-C1 --patch-json '{{\"last_line\":\"fix {i}\",\"next_opening_hint\":\"fix continues {i}\"}}'"},
                }, None, None)

            self.assertEqual((await dry_run(1))["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual((await dry_run(2))["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual((await dry_run(3))["hookSpecificOutput"]["permissionDecision"], "allow")
            denied = await dry_run(4)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("3 patch dry-runs", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_record_post_tool_duration(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        class FakeLiveLog:
            def __init__(self):
                self.records = []

            def write(self, event, payload):
                item = {"event": event}
                item.update(payload)
                self.records.append(item)

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        previous_monotonic = runner._time.monotonic
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        ticks = iter([10.0, 10.456])
        runner._time.monotonic = lambda: next(ticks)
        try:
            live_log = FakeLiveLog()
            hooks = runner.build_tool_hooks(["novelgen tool query"], live_log)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type chapter-repair --id P1-V1-C1 --view brief"},
            }, "tool-1", None)
            await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type chapter-repair --id P1-V1-C1 --view brief"},
            }, "tool-1", None)
            post_records = [record for record in live_log.records if record.get("hook") == "PostToolUse"]
            self.assertEqual(post_records[-1]["duration_ms"], 455)
        finally:
            runner._time.monotonic = previous_monotonic
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_live_tool_command_summary_redacts_patch_buffer_stdin(self):
        runner = load_runner()
        command = (
            "printf '%s' 'SECRET_CHAPTER_BODY 很长的正文' | "
            "novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin"
        )
        got = runner.summarize_live_tool_command(command)
        self.assertEqual(got, "novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin <stdin>")
        self.assertNotIn("SECRET_CHAPTER_BODY", got)
        self.assertNotIn("printf", got)

    async def test_live_tool_command_summary_redacts_claude_temp_output_reads(self):
        runner = load_runner()
        command = (
            "powershell -Command \"Get-Content 'C:\\Users\\me\\AppData\\Local\\Temp\\claude\\project\\tasks\\abc.output' "
            "-Tail 20 -Wait\" 2>$null"
        )
        got = runner.summarize_live_tool_command(command)
        self.assertEqual(got, "powershell Get-Content <claude-temp-tool-output>")
        self.assertNotIn("AppData", got)
        self.assertNotIn("abc.output", got)

    async def test_tool_hooks_live_log_redacts_patch_buffer_stdin_command(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        class FakeLiveLog:
            def __init__(self):
                self.records = []

            def write(self, event, payload):
                item = {"event": event}
                item.update(payload)
                self.records.append(item)

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            live_log = FakeLiveLog()
            hooks = runner.build_tool_hooks(["novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin"], live_log)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            await pre_hook({
                "tool_name": "Bash",
                "tool_input": {
                    "command": (
                        "printf '%s' 'SECRET_CHAPTER_BODY 很长的正文' | "
                        "novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin"
                    ),
                },
            }, "tool-stdin", None)
            pre_records = [record for record in live_log.records if record.get("hook") == "PreToolUse"]
            self.assertEqual(pre_records[-1]["command"], "novelgen tool patch-buffer append --id P1-V1-C1-draft --stdin <stdin>")
            self.assertTrue(pre_records[-1]["allowed"])
            self.assertNotIn("SECRET_CHAPTER_BODY", json.dumps(pre_records[-1], ensure_ascii=False))
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_count_chinese_patch_targets(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch craft --target character --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            for idx in range(6):
                command = f"novelgen tool patch craft --target character --id \"虫族工虫\" --patch-json '{{\"notes\":\"fix {idx}\"}}'"
                allowed = await pre_hook({
                    "tool_name": "Bash",
                    "tool_input": {"command": command},
                }, None, None)
                self.assertEqual(allowed["hookSpecificOutput"]["permissionDecision"], "allow")
                await post_hook({
                    "tool_name": "Bash",
                    "tool_input": {"command": command},
                    "tool_response": json.dumps({"next_actions": [{"action": "repair_patch_content"}]}),
                }, None, None)
            denied = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch craft --target character --id \"虫族工虫\" --patch-json '{\"notes\":\"fix 6\"}'"},
            }, None, None)
            apply_once = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch craft --target character --id \"虫族工虫\" --patch-json '{\"notes\":\"fix 0\"}' --apply"},
            }, None, None)
            apply_twice = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch craft --target character --id \"虫族工虫\" --patch-json '{\"notes\":\"fix 0\"}' --apply"},
            }, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(apply_once["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(apply_twice["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("reported blocking issues", apply_once["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_apply_patch_requires_followup_check(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch chapter --apply", "novelgen tool check"], None)
            post_hook = hooks["PostToolUse"][0].hooks[0]
            apply_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs the scene.\"}' --apply"},
            }, None, None)
            message = apply_context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("tool check", message)
            self.assertIn("novelgen tool check all --target chapter --scope chapter --id \"P1-V1-C1\"", message)
            self.assertIn("Before returning final JSON", message)
            self.assertNotIn("Return only the final JSON now", message)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_require_successful_dry_run_before_apply(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch chapter --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            direct_apply = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs.\"}' --apply"},
            }, None, None)
            self.assertEqual(direct_apply["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("successful dry-run", direct_apply["hookSpecificOutput"]["permissionDecisionReason"])

            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs.\"}'"},
                "tool_response": json.dumps({"next_actions": [{"action": "repair_patch_content"}]}),
            }
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            blocked_apply = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-json '{\"content\":\"# Opening\\n\\nLin repairs.\"}' --apply"},
            }, None, None)
            self.assertEqual(blocked_apply["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("blocking issues", blocked_apply["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_outline_volume_detail_after_clean_check(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V1 --view index",
                "novelgen tool query context --type outline-volume --id P1-V1",
                'novelgen tool check all --target outline --scope volume --id "P1-V1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(check, None, None)
            self.assertIn("Return final JSON now", complete["hookSpecificOutput"]["additionalContext"])
            self.assertIn("outline volume check", complete["hookSpecificOutput"]["additionalContext"])
            self.assertIn("including echo/status", complete["hookSpecificOutput"]["additionalContext"])

            detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-volume --id "P1-V1" --view brief'},
            }, None, None)
            self.assertEqual(detail["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", detail["hookSpecificOutput"]["permissionDecisionReason"])

            hooks_with_issue = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V2 --view index",
                "novelgen tool query context --type outline-volume --id P1-V2",
                'novelgen tool check all --target outline --scope volume --id "P1-V2"',
            ], None)
            pre_hook_with_issue = hooks_with_issue["PreToolUse"][0].hooks[0]
            post_hook_with_issue = hooks_with_issue["PostToolUse"][0].hooks[0]
            check_with_issue = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope volume --id "P1-V2" --min-priority medium --max-issues 12'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 1}, "issues": [{"priority": "medium"}]}),
            }
            self.assertEqual((await pre_hook_with_issue(check_with_issue, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(await post_hook_with_issue(check_with_issue, None, None), {})
            detail_with_issue = await pre_hook_with_issue({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-volume --id "P1-V2" --view brief'},
            }, None, None)
            self.assertEqual(detail_with_issue["hookSpecificOutput"]["permissionDecision"], "allow")

            hooks_with_focused_detail = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V3 --view index",
                "novelgen tool query context --type outline-volume --id P1-V3",
                'novelgen tool check all --target outline --scope volume --id "P1-V3"',
                'novelgen tool query outline --type chapter --id "P1-V3-C1" --view brief',
            ], None)
            pre_hook_with_focused_detail = hooks_with_focused_detail["PreToolUse"][0].hooks[0]
            post_hook_with_focused_detail = hooks_with_focused_detail["PostToolUse"][0].hooks[0]
            focused_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope volume --id "P1-V3" --min-priority medium --max-issues 12'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook_with_focused_detail(focused_check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(await post_hook_with_focused_detail(focused_check, None, None), {})
            chapter_detail_after_clean_volume = await pre_hook_with_focused_detail({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type chapter --id "P1-V3-C1" --view brief'},
            }, None, None)
            self.assertEqual(chapter_detail_after_clean_volume["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_chapter_detail_after_clean_check(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query outline --type chapter --id "P1-V1-C1"',
                'novelgen tool query outline --type events --chapter-id "P1-V1-C1"',
                "novelgen tool query context --type craft-character",
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"',
                'novelgen tool refresh chapter-dsl --id "P1-V1-C1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(check, None, None)
            self.assertIn("Return final JSON now", complete["hookSpecificOutput"]["additionalContext"])
            self.assertIn("final chapter check", complete["hookSpecificOutput"]["additionalContext"])
            self.assertIn("including echo/status", complete["hookSpecificOutput"]["additionalContext"])

            chapter_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'},
            }, None, None)
            self.assertEqual(chapter_detail["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", chapter_detail["hookSpecificOutput"]["permissionDecisionReason"])

            events_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type events --chapter-id "P1-V1-C1" --view brief'},
            }, None, None)
            self.assertEqual(events_detail["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", events_detail["hookSpecificOutput"]["permissionDecisionReason"])

            craft_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type craft-character --name "林野" --view brief'},
            }, None, None)
            self.assertEqual(craft_detail["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", craft_detail["hookSpecificOutput"]["permissionDecisionReason"])

            redundant_refresh = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C1"'},
            }, None, None)
            self.assertEqual(redundant_refresh["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("Do not refresh derived DSL again", redundant_refresh["hookSpecificOutput"]["permissionDecisionReason"])

            hooks_with_issue = runner.build_tool_hooks([
                'novelgen tool query outline --type chapter --id "P1-V1-C2"',
                'novelgen tool query outline --type events --chapter-id "P1-V1-C2"',
                "novelgen tool query context --type craft-character",
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C2"',
                'novelgen tool refresh chapter-dsl --id "P1-V1-C2"',
            ], None)
            pre_hook_with_issue = hooks_with_issue["PreToolUse"][0].hooks[0]
            post_hook_with_issue = hooks_with_issue["PostToolUse"][0].hooks[0]
            check_with_issue = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C2" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 1}, "issues": [{"priority": "medium"}]}),
            }
            self.assertEqual((await pre_hook_with_issue(check_with_issue, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(await post_hook_with_issue(check_with_issue, None, None), {})
            detail_with_issue = await pre_hook_with_issue({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type events --chapter-id "P1-V1-C2" --view brief'},
            }, None, None)
            self.assertEqual(detail_with_issue["hookSpecificOutput"]["permissionDecision"], "allow")
            craft_with_issue = await pre_hook_with_issue({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type craft-character --name "林野" --view brief'},
            }, None, None)
            self.assertEqual(craft_with_issue["hookSpecificOutput"]["permissionDecision"], "allow")
            refresh_with_issue = await pre_hook_with_issue({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C2"'},
            }, None, None)
            self.assertEqual(refresh_with_issue["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_user_prompt_driven_clean_check_does_not_stop_patch_cycle(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief',
                'novelgen tool check all --target outline --scope chapter --id "P1-V1-C1"',
                'novelgen tool patch outline --target volume --id "P1-V1"',
            ], None, user_prompt_driven=True)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target outline --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(check, None, None)
            context = complete["hookSpecificOutput"]["additionalContext"]
            self.assertNotIn("Return final JSON now", context)
            self.assertIn("user-prompt-driven", context)
            self.assertIn("patch dry-run/apply cycle", context)

            # The clean check must not close the target: detail queries and the
            # patch cycle remain allowed in user-prompt-driven mode.
            chapter_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'},
            }, None, None)
            self.assertEqual(chapter_detail["hookSpecificOutput"]["permissionDecision"], "allow")

            patch_dry_run = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "printf '%s' '{\"summary\":\"fixed\"}' | novelgen tool patch outline --target volume --id \"P1-V1\""},
            }, None, None)
            self.assertEqual(patch_dry_run["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_user_prompt_driven_disables_stop_after_required_queries(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V1 --view index",
                'novelgen tool query outline --type refs --entity-type storyline --name "主线" --view brief',
            ], None, user_prompt_driven=True)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            setup_input = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type outline-volume --id P1-V1 --view index"},
            }
            volume_input = {
                "tool_name": "Bash",
                "tool_input": {"command": "D:/Code/nolvegen/bin/novelgen.exe tool query outline --type refs --entity-type storyline --name \"主线\" --view brief"},
            }
            self.assertEqual((await pre_hook(setup_input, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(setup_input, None, None)
            self.assertEqual((await pre_hook(volume_input, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(volume_input, None, None)
            self.assertEqual(complete, {})
            duplicate_query = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type outline-volume --id P1-V1 --view index"},
            }, None, None)
            self.assertEqual(duplicate_query["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    def test_user_prompt_driven_flag_prefers_explicit_marker(self):
        runner = load_runner()
        self.assertTrue(runner.user_prompt_driven_flag({"user_prompt_driven": True}))
        self.assertFalse(runner.user_prompt_driven_flag({"user_prompt_driven": False, "user_prompt": "task"}))
        self.assertTrue(runner.user_prompt_driven_flag({"user_prompt": "task"}))
        self.assertFalse(runner.user_prompt_driven_flag({"user_prompt": ""}))
        self.assertFalse(runner.user_prompt_driven_flag({}))

    async def test_tool_hooks_stop_patch_cycle_after_post_apply_clean_check(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            initial_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(initial_check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(initial_check, None, None)

            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"'},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)

            apply = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived'},
            }
            self.assertEqual((await pre_hook(apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(apply, None, None)

            before_followup_check = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"'},
            }, None, None)
            self.assertEqual(before_followup_check["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("follow-up check", before_followup_check["hookSpecificOutput"]["permissionDecisionReason"])

            clean_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(clean_check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(clean_check, None, None)

            second_cycle = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"'},
            }, None, None)
            self.assertEqual(second_cycle["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("post-apply check", second_cycle["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_allow_second_patch_cycle_after_post_apply_check_with_issues(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            initial_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            self.assertEqual((await pre_hook(initial_check, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(initial_check, None, None)

            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"'},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            apply = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived'},
            }
            self.assertEqual((await pre_hook(apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(apply, None, None)

            check_with_issue = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": True, "summary": {"total": 1}, "issues": [{"priority": "critical"}]}),
            }
            self.assertEqual((await pre_hook(check_with_issue, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(check_with_issue, None, None)

            second_cycle = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"'},
            }, None, None)
            self.assertEqual(second_cycle["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_content_query_after_apply_tool_reports_clean_check(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
                'novelgen tool query chapter --id "P1-V1-C1" --content --view brief',
                'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            initial_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8'},
                "tool_response": json.dumps({"blocking": False, "summary": {"total": 0}, "issues": []}),
            }
            await pre_hook(initial_check, None, None)
            await post_hook(initial_check, None, None)
            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"'},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            apply = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived'},
                "tool_response": json.dumps({"next_actions": [{"action": "return_final_json"}]}),
            }
            self.assertEqual((await pre_hook(apply, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(apply, None, None)

            content_query = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query chapter --id "P1-V1-C1" --content --view brief'},
            }, None, None)
            self.assertEqual(content_query["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", content_query["hookSpecificOutput"]["permissionDecisionReason"])

            second_patch = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool patch-buffer clear --id "P1-V1-C1-draft"'},
            }, None, None)
            self.assertEqual(second_patch["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("post-apply check", second_patch["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_use_check_blocking_as_dry_run_state(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch chapter --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            blocked_dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-a"},
                "tool_response": json.dumps({"check": {"blocking": True}}),
            }
            self.assertEqual((await pre_hook(blocked_dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(blocked_dry_run, None, None)
            blocked_apply = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-a --apply"},
            }, None, None)
            self.assertEqual(blocked_apply["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("blocking issues", blocked_apply["hookSpecificOutput"]["permissionDecisionReason"])

            valid_dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-b"},
                "tool_response": json.dumps({"check": {"blocking": False}}),
            }
            self.assertEqual((await pre_hook(valid_dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(valid_dry_run, None, None)
            valid_apply = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-b --apply --refresh-derived"},
            }, None, None)
            self.assertEqual(valid_apply["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_deny_repeated_required_query_only_for_query_only_workflow(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief'},
            }

            self.assertEqual((await pre_hook(query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(query, None, None)
            denied = await pre_hook(query, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("already been executed", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_allow_one_utf8_wrapper_retry_for_required_query(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            allowlist = [
                'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief',
            ]
            hooks = runner.build_tool_hooks(allowlist, None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief'},
            }
            utf8_retry = {
                "tool_name": "Bash",
                "tool_input": {"command": "powershell -NoProfile -Command \"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); & 'novelgen' tool query context --type chapter-write --id 'P1-V1-C1' --view brief\""},
            }

            self.assertEqual((await pre_hook(query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(query, None, None)
            self.assertEqual((await pre_hook(utf8_retry, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            denied = await pre_hook(utf8_retry, None, None)
            self.assertEqual(denied["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("already been executed", denied["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_deny_buffer_mutation_after_validated_dry_run(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            self.assertEqual((await pre_hook(dry_run, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run, None, None)
            blocked_clear = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch-buffer clear --id P1-V1-C1-draft"},
            }, None, None)
            self.assertEqual(blocked_clear["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("already has a successful dry-run", blocked_clear["hookSpecificOutput"]["permissionDecisionReason"])
            apply = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
            }, None, None)
            self.assertEqual(apply["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_dry_run_without_apply_returns_json_instruction(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch-buffer --id "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"',
            ], None)
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft"},
                "tool_response": json.dumps({"next_actions": [{"action": "return_final_json"}]}),
            }
            context = await post_hook(dry_run, None, None)
            message = context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("does not allow --apply", message)
            self.assertIn("Return the final JSON now", message)
            self.assertIn("Go will save the chapter", message)
            self.assertNotIn("repeat the same patch command with --apply", message)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_require_apply_to_match_last_dry_run(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch chapter --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run_a = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-a"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            self.assertEqual((await pre_hook(dry_run_a, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(dry_run_a, None, None)

            apply_b = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-b --apply"},
            }, None, None)
            self.assertEqual(apply_b["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("does not match", apply_b["hookSpecificOutput"]["permissionDecisionReason"])

            apply_a = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer draft-a --apply"},
            }, None, None)
            self.assertEqual(apply_a["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_apply_patch_refreshes_chapter_dsl_when_allowed(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool patch chapter --apply",
                "novelgen tool refresh chapter-dsl",
                "novelgen tool check",
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            apply_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
            }, None, None)
            message = apply_context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("novelgen tool refresh chapter-dsl --id \"P1-V1-C1\"", message)
            self.assertIn("novelgen tool check all --target chapter --scope chapter --id \"P1-V1-C1\"", message)
            first_dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-a"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            await pre_hook(first_dry_run, None, None)
            await post_hook(first_dry_run, None, None)
            first_apply = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-a --apply"},
            }
            await pre_hook(first_apply, None, None)
            await post_hook(first_apply, None, None)
            first_followup_check = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12'},
                "tool_response": json.dumps({"blocking": True, "summary": {"total": 1}, "issues": [{"priority": "critical"}]}),
            }
            await pre_hook(first_followup_check, None, None)
            await post_hook(first_followup_check, None, None)
            second_dry_run = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-b"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }
            await pre_hook(second_dry_run, None, None)
            await post_hook(second_dry_run, None, None)
            await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-b --apply"},
            }, None, None)
            second_apply_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-b --apply"},
            }, None, None)
            second_message = second_apply_context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("Do not start another patch-buffer or patch cycle", second_message)
            self.assertIn("Go will surface the remaining issues", second_message)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_apply_refresh_derived_does_not_request_extra_refresh(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool patch chapter --apply --refresh-derived",
                "novelgen tool refresh chapter-dsl",
                "novelgen tool check",
            ], None)
            post_hook = hooks["PostToolUse"][0].hooks[0]
            apply_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply --refresh-derived"},
            }, None, None)
            message = apply_context["hookSpecificOutput"]["additionalContext"]
            self.assertNotIn("novelgen tool refresh chapter-dsl", message)
            self.assertIn("novelgen tool check all --target chapter --scope chapter --id \"P1-V1-C1\"", message)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_dry_run_can_apply_when_refresh_derived_is_required(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool patch chapter --id "P1-V1-C1"',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"',
                'novelgen tool patch chapter --id "P1-V1-C1" --apply --refresh-derived',
                'novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived',
            ], None)
            post_hook = hooks["PostToolUse"][0].hooks[0]
            dry_run_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft"},
                "tool_response": json.dumps({"next_actions": [{"action": "apply_validated_patch"}]}),
            }, None, None)
            message = dry_run_context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("with --apply --refresh-derived", message)
            self.assertNotIn("does not allow --apply", message)
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_allow_tool_returned_next_actions(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks(["novelgen tool patch chapter --apply"], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            tool_response = json.dumps({
                "next_actions": [
                    {"action": "refresh_derived_dsl", "command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C1"'},
                    {"action": "post_refresh_check", "command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12'},
                ],
            })
            apply_context = await post_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply"},
                "tool_response": tool_response,
            }, None, None)
            message = apply_context["hookSpecificOutput"]["additionalContext"]
            self.assertIn("tool-returned next_actions", message)
            self.assertIn("novelgen tool refresh chapter-dsl --id \"P1-V1-C1\"", message)
            refresh = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool refresh chapter-dsl --id "P1-V1-C1"'},
            }, None, None)
            self.assertEqual(refresh["hookSpecificOutput"]["permissionDecision"], "allow")
            check = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12'},
            }, None, None)
            self.assertEqual(check["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_after_context_return_final_json(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name logic --view index',
                'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name logic --view brief',
                'novelgen tool query outline --type chapter --id "P1-V1-C1"',
                "novelgen tool query context --type craft-character",
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            index_query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name logic --view index'},
                "tool_response": json.dumps({
                    "next_actions": [
                        {"action": "focused_check_clean"},
                        {"action": "return_final_json"},
                    ],
                }),
            }
            self.assertEqual((await pre_hook(index_query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(index_query, None, None)
            self.assertIn("return_final_json", complete["hookSpecificOutput"]["additionalContext"])
            self.assertIn("Do not fetch brief/full context", complete["hookSpecificOutput"]["additionalContext"])

            brief = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name logic --view brief'},
            }, None, None)
            self.assertEqual(brief["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", brief["hookSpecificOutput"]["permissionDecisionReason"])

            outline_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief'},
            }, None, None)
            self.assertEqual(outline_detail["hookSpecificOutput"]["permissionDecision"], "deny")

            craft_detail = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type craft-character --name "Lin" --view brief'},
            }, None, None)
            self.assertEqual(craft_detail["hookSpecificOutput"]["permissionDecision"], "deny")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_recap_brief_after_context_return_final_json(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view index',
                'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            index_query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view index'},
                "tool_response": json.dumps({
                    "next_actions": [
                        {"action": "focused_check_clean"},
                        {"action": "return_final_json"},
                    ],
                }),
            }
            self.assertEqual((await pre_hook(index_query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(index_query, None, None)
            self.assertIn("return_final_json", complete["hookSpecificOutput"]["additionalContext"])

            brief = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief'},
            }, None, None)
            self.assertEqual(brief["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", brief["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_outline_global_brief_after_unpatchable_route(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query context --type outline-global-repair --name "mysteries" --view index',
                'novelgen tool query context --type outline-global-repair --name "mysteries" --view brief',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            index_query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-global-repair --name "mysteries" --view index'},
                "tool_response": json.dumps({
                    "next_actions": [
                        {"action": "classify_unpatchable_global_issue"},
                        {"action": "return_final_json"},
                    ],
                }),
            }
            self.assertEqual((await pre_hook(index_query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            complete = await post_hook(index_query, None, None)
            self.assertIn("return_final_json", complete["hookSpecificOutput"]["additionalContext"])

            brief = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-global-repair --name "mysteries" --view brief'},
            }, None, None)
            self.assertEqual(brief["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", brief["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_named_outline_global_brief_after_unnamed_unpatchable_route(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                'novelgen tool query context --type outline-global-repair --view index',
                'novelgen tool query context --type outline-global-repair --name "mysteries" --view brief',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            index_query = {
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-global-repair --view index'},
                "tool_response": json.dumps({
                    "next_actions": [
                        {"action": "classify_unpatchable_global_issue"},
                        {"action": "return_final_json"},
                    ],
                }),
            }
            self.assertEqual((await pre_hook(index_query, None, None))["hookSpecificOutput"]["permissionDecision"], "allow")
            await post_hook(index_query, None, None)

            named_brief = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": 'novelgen tool query context --type outline-global-repair --name "mysteries" --view brief'},
            }, None, None)
            self.assertEqual(named_brief["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("returned no issues", named_brief["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_stop_after_exact_required_queries(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V1 --view index",
                'novelgen tool query outline --type refs --entity-type storyline --name "主线" --view brief',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            setup_input = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type outline-volume --id P1-V1 --view index"},
            }
            volume_input = {
                "tool_name": "Bash",
                "tool_input": {"command": "D:/Code/nolvegen/bin/novelgen.exe tool query outline --type refs --entity-type storyline --name \"主线\" --view brief"},
            }
            first = await pre_hook(setup_input, None, None)
            await post_hook(setup_input, None, None)
            duplicate = await pre_hook(setup_input, None, None)
            second = await pre_hook(volume_input, None, None)
            complete = await post_hook(volume_input, None, None)
            extra_bash = await pre_hook({
                "tool_name": "Bash",
                "tool_input": {"command": "printf '%s' '{\"content\":\"done\"}'"},
            }, None, None)

            self.assertEqual(first["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(duplicate["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertEqual(second["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertIn("All required_queries", complete["hookSpecificOutput"]["additionalContext"])
            self.assertEqual(extra_bash["hookSpecificOutput"]["permissionDecision"], "deny")
            self.assertIn("Return only the final JSON", extra_bash["hookSpecificOutput"]["permissionDecisionReason"])
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_tool_hooks_do_not_stop_when_followup_checks_are_allowed(self):
        runner = load_runner()
        fake_sdk = types.ModuleType("claude_agent_sdk")
        fake_types = types.ModuleType("claude_agent_sdk.types")

        @dataclass
        class HookMatcher:
            matcher: str | None = None
            hooks: list | None = None
            timeout: float | None = None

        fake_types.HookMatcher = HookMatcher
        fake_sdk.types = fake_types
        previous = sys.modules.get("claude_agent_sdk")
        previous_types = sys.modules.get("claude_agent_sdk.types")
        sys.modules["claude_agent_sdk"] = fake_sdk
        sys.modules["claude_agent_sdk.types"] = fake_types
        try:
            hooks = runner.build_tool_hooks([
                "novelgen tool query context --type outline-volume --id P1-V1 --view index",
                'novelgen tool check all --target outline --scope volume --id "P1-V1"',
            ], None)
            pre_hook = hooks["PreToolUse"][0].hooks[0]
            post_hook = hooks["PostToolUse"][0].hooks[0]
            query_input = {
                "tool_name": "Bash",
                "tool_input": {"command": "novelgen tool query context --type outline-volume --id P1-V1 --view index"},
            }
            check_input = {
                "tool_name": "Bash",
                "tool_input": {
                    "command": 'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12',
                },
            }

            first = await pre_hook(query_input, None, None)
            complete = await post_hook(query_input, None, None)
            check = await pre_hook(check_input, None, None)

            self.assertEqual(first["hookSpecificOutput"]["permissionDecision"], "allow")
            self.assertEqual(complete, {})
            self.assertEqual(check["hookSpecificOutput"]["permissionDecision"], "allow")
        finally:
            if previous is None:
                sys.modules.pop("claude_agent_sdk", None)
            else:
                sys.modules["claude_agent_sdk"] = previous
            if previous_types is None:
                sys.modules.pop("claude_agent_sdk.types", None)
            else:
                sys.modules["claude_agent_sdk.types"] = previous_types

    async def test_require_sdk_disables_http_fallback(self):
        runner = load_runner()
        previous = runner.run_with_claude_agent_sdk

        async def missing_sdk(invocation):
            raise ImportError()

        runner.run_with_claude_agent_sdk = missing_sdk
        try:
            with self.assertRaisesRegex(RuntimeError, "claude_agent_sdk is required"):
                await runner.run({"require_sdk": True, "user_prompt": "hi"})
        finally:
            runner.run_with_claude_agent_sdk = previous

    def test_configure_text_stream_forces_utf8_strict(self):
        runner = load_runner()

        class FakeStream:
            def __init__(self):
                self.calls = []

            def reconfigure(self, **kwargs):
                self.calls.append(kwargs)

        stream = FakeStream()
        runner.configure_text_stream(stream)
        self.assertEqual(stream.calls, [{"encoding": "utf-8", "errors": "strict"}])

    def test_repair_text_encoding_fixes_gbk_mojibake(self):
        runner = load_runner()
        repaired = runner.repair_text_encoding("宸查獙璇佹墍鏈夋煡璇㈠拰dry-run缁撴灉鏈夋晥")
        escaped = ascii(repaired)
        self.assertIn("\\u5df2\\u9a8c\\u8bc1", escaped)
        self.assertIn("\\u67e5\\u8be2", escaped)
        self.assertIn("dry-run", repaired)

    def test_repair_text_encoding_keeps_normal_chinese(self):
        runner = load_runner()
        text = "林野完成校准，火种核心稳定下来"
        self.assertEqual(runner.repair_text_encoding(text), text)

    def test_repair_text_encoding_keeps_normal_chinese_with_marker_like_chars(self):
        runner = load_runner()
        text = "苏璇在璇玑阵前确认权限，林野没有改动日志。"
        self.assertEqual(runner.repair_text_encoding(text), text)

    def test_repair_text_encoding_keeps_normal_fire_galaxy_prompt(self):
        runner = load_runner()
        text = "".join([
            chr(0x79D1), chr(0x5E7B), ", ", chr(0x5E9F), chr(0x571F), ", ",
            chr(0x673A), chr(0x7532), ", ", chr(0x672B), chr(0x4E16), ". ",
            chr(0x6797), chr(0x91CE), chr(0x5728), chr(0x8352), chr(0x9AA8),
            chr(0x53F0), chr(0x82CF), chr(0x9192), chr(0xFF0C),
            chr(0x6FC0), chr(0x6D3B), chr(0x706B), chr(0x79CD), chr(0x6838),
            chr(0x5FC3), chr(0x4E0E), chr(0x7EAF), chr(0x673A), chr(0x68B0),
            "3D", chr(0x6253), chr(0x5370), chr(0x8BBE), chr(0x5907), chr(0xFF0C),
            chr(0x4FDD), chr(0x7559), chr(0x7ED3), chr(0x5C3E), chr(0x94A9),
            chr(0x5B50), chr(0x5E76), chr(0x538B), chr(0x7F29), chr(0x91CD),
            chr(0x590D), chr(0x89E3), chr(0x91CA), chr(0x3002),
        ])
        repaired = runner.repair_text_encoding(text)
        self.assertEqual(repaired, text)
        self.assertIn(chr(0x706B) + chr(0x79CD) + chr(0x6838) + chr(0x5FC3), repaired)


class EvidenceGuardTests(unittest.TestCase):
    def setUp(self):
        self.runner = load_runner()

    def test_evidence_minimums_parses_config(self):
        minimums = self.runner.evidence_minimums({
            "min_query_calls": 1,
            "min_context_query_calls": 2,
            "min_check_calls": 3,
            "min_patch_apply_calls": 4,
        })
        self.assertEqual(minimums["query"], 1)
        self.assertEqual(minimums["context"], 2)
        self.assertEqual(minimums["check"], 3)
        self.assertEqual(minimums["patch_apply"], 4)
        self.assertEqual(self.runner.evidence_minimums(None), {
            "query": 0,
            "context": 0,
            "check": 0,
            "patch_apply": 0,
        })

    def test_count_evidence_tool_call_counts_queries_and_checks(self):
        counts = {"query": 0, "context": 0, "check": 0, "patch_apply": 0}
        observed = set()
        self.runner.count_evidence_tool_call(
            "novelgen tool query outline --type chapter --id P1-V1-C1 --view brief",
            counts,
            observed,
        )
        self.runner.count_evidence_tool_call(
            "novelgen tool query context --type outline-volume --id P1-V1 --view index",
            counts,
            observed,
        )
        self.runner.count_evidence_tool_call(
            'novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12',
            counts,
            observed,
        )
        self.assertEqual(counts["query"], 2)
        self.assertEqual(counts["context"], 1)
        self.assertEqual(counts["check"], 1)
        self.assertEqual(counts["patch_apply"], 0)
        self.assertIn("novelgen tool check all --target outline --scope volume --id p1-v1 --min-priority medium --max-issues 12", observed)

    def test_count_evidence_tool_call_counts_patch_apply(self):
        counts = {"query": 0, "context": 0, "check": 0, "patch_apply": 0}
        self.runner.count_evidence_tool_call(
            "novelgen tool patch outline --target volume --id P1-V1 --apply",
            counts,
        )
        self.assertEqual(counts["patch_apply"], 1)
        self.runner.count_evidence_tool_call(
            "novelgen tool patch outline --target volume --id P1-V1",
            counts,
        )
        self.assertEqual(counts["patch_apply"], 1)

    def test_missing_evidence_instruction_empty_when_satisfied(self):
        minimums = {"query": 1, "context": 0, "check": 1, "patch_apply": 0}
        counts = {"query": 2, "context": 0, "check": 1, "patch_apply": 0}
        self.assertEqual(
            self.runner.missing_evidence_instruction(minimums, [], counts),
            "",
        )

    def test_missing_evidence_instruction_reports_check_and_required_command(self):
        minimums = {"query": 0, "context": 0, "check": 1, "patch_apply": 0}
        counts = {"query": 3, "context": 1, "check": 0, "patch_apply": 0}
        observed = {"novelgen tool query context --type outline-volume --id p1-v1 --view index"}
        instruction = self.runner.missing_evidence_instruction(
            minimums,
            ["novelgen tool check all --target outline --scope volume --id P1-V1"],
            counts,
            observed,
        )
        self.assertIn("novelgen tool check", instruction)
        self.assertIn("tool evidence is missing", instruction)
        self.assertIn("run exactly", instruction)

    def test_missing_evidence_instruction_reports_context_and_patch(self):
        minimums = {"query": 1, "context": 1, "check": 0, "patch_apply": 1}
        counts = {"query": 1, "context": 0, "check": 0, "patch_apply": 0}
        instruction = self.runner.missing_evidence_instruction(minimums, [], counts)
        self.assertIn("query context", instruction)
        self.assertIn("--apply", instruction)

    def test_denial_guard_instruction(self):
        self.assertEqual(self.runner.denial_guard_instruction(False, True, False), "")
        self.assertEqual(self.runner.denial_guard_instruction(True, False, False), "")
        self.assertEqual(self.runner.denial_guard_instruction(True, True, True), "")
        instruction = self.runner.denial_guard_instruction(True, True, False)
        self.assertIn("denied", instruction)
        self.assertIn("workflow allowlist", instruction)

    def test_combine_guard_instructions(self):
        self.assertEqual(self.runner.combine_guard_instructions("", ""), "")
        self.assertEqual(
            self.runner.combine_guard_instructions("one", "", "two"),
            "one two",
        )


class LiveLoggerTests(unittest.TestCase):
    def setUp(self):
        self.runner = load_runner()

    def test_write_then_close_persists_all_records(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "live.jsonl")
            logger = self.runner.LiveLogger(path)
            logger.write("start", {"a": 1})
            logger.write("final", {"b": 2})
            logger.close()
            with open(path, "r", encoding="utf-8") as f:
                records = [json.loads(line) for line in f]
            self.assertEqual([r["event"] for r in records], ["start", "final"])
            self.assertEqual(records[0]["a"], 1)
            self.assertEqual(records[1]["b"], 2)

    def test_unserializable_payload_falls_back_and_logger_survives(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "live.jsonl")
            logger = self.runner.LiveLogger(path)
            circular = {}
            circular["self"] = circular
            logger.write("message", {"payload": circular})
            logger.write("final", {"ok": True})
            logger.close()
            with open(path, "r", encoding="utf-8") as f:
                records = [json.loads(line) for line in f]
            self.assertEqual(len(records), 2)
            self.assertTrue(records[0].get("fallback"))
            self.assertIn("payload", records[0].get("payload_repr", ""))
            self.assertEqual(records[1]["event"], "final")

    def test_io_error_retries_without_disabling_logger(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "live.jsonl")
            logger = self.runner.LiveLogger(path)
            real_file = logger._file

            class FlakyFile:
                def __init__(self, inner):
                    self._inner = inner
                    self._fail = True

                def write(self, data):
                    if self._fail:
                        self._fail = False
                        raise OSError("simulated transient failure")
                    return self._inner.write(data)

                def flush(self):
                    return self._inner.flush()

                def close(self):
                    return self._inner.close()

            logger._file = FlakyFile(real_file)
            with unittest.mock.patch.object(self.runner, "open", wraps=open):
                logger.write("message", {"n": 1})
            # The failed write's retry replaced logger._file with a fresh
            # handle; release the orphaned original so the temp dir unlocks.
            real_file.close()
            logger.write("message", {"n": 2})
            logger.close()
            with open(path, "r", encoding="utf-8") as f:
                records = [json.loads(line) for line in f]
            self.assertEqual(len(records), 2)
            self.assertEqual(records[0]["n"], 1)
            self.assertEqual(records[1]["n"], 2)


class AgentDeadlineTests(unittest.TestCase):
    def setUp(self):
        self.runner = load_runner()
        self._old_env = os.environ.get("NOVELGEN_AGENT_TIMEOUT")
        os.environ.pop("NOVELGEN_AGENT_TIMEOUT", None)

    def tearDown(self):
        if self._old_env is None:
            os.environ.pop("NOVELGEN_AGENT_TIMEOUT", None)
        else:
            os.environ["NOVELGEN_AGENT_TIMEOUT"] = self._old_env

    def test_deadline_from_options_with_grace(self):
        deadline = self.runner.agent_runner_deadline({"options": {"timeout": 300}})
        self.assertEqual(deadline, 240.0)

    def test_deadline_minimum_grace(self):
        deadline = self.runner.agent_runner_deadline({"options": {"timeout": 30}})
        self.assertEqual(deadline, 30.0)

    def test_deadline_falls_back_to_env(self):
        os.environ["NOVELGEN_AGENT_TIMEOUT"] = "1500"
        deadline = self.runner.agent_runner_deadline({"options": {}})
        self.assertEqual(deadline, 1440.0)

    def test_deadline_none_when_unset(self):
        self.assertIsNone(self.runner.agent_runner_deadline({"options": {}}))


if __name__ == "__main__":
    unittest.main()
