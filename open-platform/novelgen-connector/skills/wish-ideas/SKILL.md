---
name: wish-ideas
display_name: 灵感抽卡
display_name_en: Idea Draw
description: 使用确定性 seed 抽取角色名卡、场景、设定体系、世界包等创作参考。
description_zh: 使用确定性 seed 抽取角色名卡、场景、设定体系、世界包等创作参考。
description_en: Draw deterministic name cards, scenarios, premise engines and world packs.
version: 0.1.0
author: Malxi
---

# Wish Ideas

`ideas` 是参考库，不是项目状态，不需要打开项目。

```bash
novelgen ideas draw --type namecard --genre 科幻 --count 12 --seed 42 --view brief
novelgen ideas draw --type premise --genre 科幻 --count 3 --seed 7
novelgen ideas draw --type scenario --genre 科幻 --count 5 --seed 7
novelgen ideas types
```

可用类型：

```text
provocation axiom event law scenario engine worldpack premise
storyline cast namecard resource itemcard reference name
```

- `count` 最大 20；保留 `seed` 方便复现。
- `seed=0` 由工具选择并报告新 seed。
- 角色名卡抽到的名字和目标原型可以直接用于 setup/outline；剧情卡要显式选择后
  再进入 outline，而不是自动塞进项目。
- 不要把参考卡直接写入 `novel.json`。
