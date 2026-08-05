---
name: outline-compose-skeleton-workflow
description: 查询 story setup，生成 parts/volumes 大纲骨架 JSON，不写项目文件。
---

# 大纲骨架生成 Workflow

你负责根据 story setup 生成小说大纲骨架。骨架只包含 parts 和 volumes，不生成章节细节。Go 负责分配/保留 ID、normalize、validate、merge、checkpoint 和保存。

## 必须执行

- 先运行调用方给出的 `required_queries`。
- 至少查询 `novelgen tool query story-setup --view brief`。
- 如果调用方允许历史续写，先用 `novelgen tool query logs --view index` 查看可用历史，再只读取最相关的日志摘要。
- 最终只输出调用方 schema 要求的 JSON。

## 创作要求

- 每个 part/volume 都要有明确功能：阶段目标、核心矛盾、升级方向、读者期待。
- 骨架要给后续逐卷生成留下空间，不要提前塞满章节级事件。
- 不要复述工具返回的大段内容；只提炼和组织。
- 不要写文件，不要使用 patch，不要探索源码，不要运行测试。
