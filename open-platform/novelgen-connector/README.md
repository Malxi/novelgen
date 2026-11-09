# Novelgen Connector (CLI + Skill)

Official WorkBuddy Open Platform package for 许愿机 Novelgen.

```text
novelgen-connector/
├── connector-meta.json
├── cli.json
├── icon.svg
└── skills/
    ├── wish-book/SKILL.md
    ├── wish-check/SKILL.md
    ├── wish-ideas/SKILL.md
    └── wish-gen/SKILL.md
```

## How installation works

`cli.json` declares a managed Node runtime and installs the npm launcher from a
stable GitHub release asset:

```text
https://github.com/Malxi/novelgen/releases/latest/download/novelgen-cli.tgz
```

The npm package's `postinstall` downloads the matching platform binary from the
same release into `~/.novelgen/bin/<version>/`. `status` and `unAuth` run
`novelgen version`; they are read-only and return 0 when the CLI is installed.

## Release prerequisite

The release workflow at `.github/workflows/release.yml` builds:

```text
novelgen-windows-amd64.exe
novelgen-windows-arm64.exe
novelgen-darwin-amd64
novelgen-darwin-arm64
novelgen-linux-amd64
novelgen-linux-arm64
novelgen-cli-<version>.tgz
novelgen-cli.tgz
```

Trigger it by pushing a tag:

```bash
git tag v0.1.1
git push origin v0.1.1
```

Wait for the GitHub Release to publish before submitting the connector.

## Local verification

```bash
go build -o bin/novelgen.exe .
bin/novelgen.exe version
bin/novelgen.exe book status --json
bin/novelgen.exe check --profile quality --target outline
```

## Submit

1. Sign in at https://open.workbuddy.cn
2. Complete personal or enterprise verification.
3. Set the developer profile.
4. Create a Connector (`CLI + Skill`) and upload this directory as a zip.
5. Submit for review.

Submission checklist from the official docs:

- `connector-meta.json` is complete and `source` is globally unique kebab-case.
- `type` is `"cli"`.
- `cli.json` has `init`, `unAuth`, `status` for `darwin`, `linux`, `win32`.
- `status` returns 0 within 10 seconds and has no side effects.
- `unAuth` is idempotent.
- Skills contain the real command usage, not MCP tool names.
- No real tokens or secrets are present.
- `minWorkbuddyVersion` is `4.24.0` because `versionCheck` is used.

Official docs:

- https://open.workbuddy.cn/docs/onboarding
- https://open.workbuddy.cn/docs/connector
- Contact: openworkbuddy@tencent.com
