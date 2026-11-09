---
name: wish-check
display_name: 小说检查
display_name_en: Novel Check
description: 运行 Novelgen 的 quality、schema、RPG 或自定义规则检查并解释 findings。
description_zh: 运行结构、schema、RPG 或自定义规则检查，返回并解释 findings。
description_en: Run Novelgen quality, schema, RPG or custom rule checks and explain findings.
version: 0.1.0
author: Malxi
---

# Wish Check

检查是确定性的，不调用模型。在项目根目录执行，或加 `--dir`。

```bash
novelgen check --profile all --target outline --min-severity high --max-findings 12
novelgen check --profile quality --target chapter --id P1-V1-C1
novelgen check --profile custom --target outline --dir my-first-novel
```

Profile：

```text
quality   结构/契约规则
schema    craft/recap 类型化 schema
rpg       内存 RPG 模拟
all       quality + rpg
custom    .novelgen/rules/*.json
```

输出中的 `findings` 是标准列表，`issues` 是兼容别名。每条 finding 包含
`rule_id`、`severity`、`target_id`、`issue`、`suggestion`，可修复项还包含
`navigation`。

报告时先列 blocking/high，再列 medium/low；不要为了通过检查直接改项目文件，
把修复交给 `wish-book` 的 `book write` dry-run。

自定义规则示例（`.novelgen/rules/no-deus.json`）：

```json
{"id":"no-deus-ex","target":"chapter","severity":"high","kind":"regex","pattern":"突然.*醒来","message":"疑似机械降神"}
```

`kind` 支持 `regex` / `contains`；规则可以是单个对象或数组。
