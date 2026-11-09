# WorkBuddy 官方平台发布（CLI + Skill）

官方入口是 WorkBuddy 开放平台，不是 `Add Marketplace`：

- https://open.workbuddy.cn
- 入驻：https://open.workbuddy.cn/docs/onboarding
- Connector 规范：https://open.workbuddy.cn/docs/connector

## 提交包

```text
open-platform/novelgen-connector/
├── connector-meta.json
├── cli.json
├── icon.svg
└── skills/
    ├── wish-book/SKILL.md
    ├── wish-check/SKILL.md
    ├── wish-ideas/SKILL.md
    └── wish-gen/SKILL.md
```

这是 **CLI + Skill** Connector：用户安装 Connector 时，WorkBuddy 用托管 Node
运行时执行：

```text
npm install -g https://github.com/Malxi/novelgen/releases/latest/download/novelgen-cli.tgz
```

npm 启动器会把对应平台的 `novelgen` 二进制下载到 `~/.novelgen/bin/<version>/`；
`status`/`unAuth` 执行 `novelgen version`，无副作用并返回 0。

## 发布前必须先生成 Release

发布流程：

```bash
git tag v0.1.1
git push origin v0.1.1
```

`.github/workflows/release.yml` 会生成：

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

必须等 GitHub Release 发布完成后再提交 Connector，否则 `cli.json` 的安装地址
会 404。

## 官方提交流程

1. 登录 https://open.workbuddy.cn
2. 完成个人/企业/组织机构认证
3. 设置开发者信息
4. 创建 Connector，选择 `CLI + Skill`
5. 上传 `open-platform/novelgen-connector` 的 zip
6. 提交审核，审核通过后发布

个人认证支持中国大陆身份证；企业认证需要营业执照/腾讯云或微信认证。
运营联系：openworkbuddy@tencent.com

## 当前检查项

- `connector-meta.json`：`source=novelgen`，`type=cli`
- `cli.json`：`init`/`unAuth`/`status` 覆盖 darwin/linux/win32
- `status`：`novelgen version`，10 秒内返回，无副作用
- `unAuth`：`novelgen version`，幂等
- 无 auth（本工具不需要账号授权）
- `versionCheck.minVersion=0.1.0`
- `minWorkbuddyVersion=4.24.0`（使用 `versionCheck`）

## 平台测试注意

官方要求至少 macOS/Linux 测试；当前仓库的 release workflow 已交叉编译三平台，
但正式提交前仍应在真实 macOS/Linux 上跑一遍：

```bash
npm install -g https://github.com/Malxi/novelgen/releases/latest/download/novelgen-cli.tgz
novelgen version
novelgen check --profile quality --target outline
```
