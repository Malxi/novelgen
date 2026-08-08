#!/usr/bin/env python3
"""
review_matrix.py — 多轮 focus review + 聚类抽样 + 合理化 prompt 生成

用法:
  python3 scripts/review_matrix.py --project ~/books/system-log --volume 1 \
      --models deepseek-v4-flash,minimax-m3,qwen3.6-plus \
      --focus all --out-dir /tmp/review_matrix_out

流程:
  1. 每个 (model, focus) 组合跑一轮 `novelgen compose review --agent-sdk`
     (focus 即 --prompt 参数, 来自 scripts/review-focus/*.txt)
  2. 合并所有 review 报告的 suggestions
  3. 按主题关键词聚类
  4. 分层抽样: 每主题保底 1 条 + 随机补足到 --sample N 条
  5. 生成合理化 prompt (供下一次 review 裁决), 同时输出抽样后的建议 JSON

输出:
  out-dir/reviews/           各轮原始 review 报告
  out-dir/all_suggestions.json   合并后的建议 (含主题标签)
  out-dir/sampled_<n>.json       抽样结果 (可直接给 --suggestions 用)
  out-dir/rationalize_prompt.txt 合理化裁决 prompt
  out-dir/matrix_report.txt      汇总报告 (分数/主题分布/同质化统计)
"""
import argparse
import json
import os
import random
import re
import subprocess
import sys
from collections import Counter
from concurrent.futures import ThreadPoolExecutor, as_completed

# ============ 主题聚类关键词 ============
THEMES = {
    '角色动机/弧光': ['动机', '弧光', '成长', '双标', '反思', '执念', '行为准则', '内心', '人物', '私心',
                  '降智', '工具人', '弧线', '私心', '恩怨', '人情', '情绪', '失控'],
    '中段循环/节拍器': ['每日', '循环', '干预', '停摆', '引擎', '中段', '静默', '消失', '新办法', '方案',
                    '变奏', 'mutation', '压力', '节拍', '对称', '闭环', '同构', '重复', '节奏', '单调'],
    '推演/能力边界': ['推演', '模拟器', '能力边界', '日志系统', '数据挖掘', '单向', '边界', '报错', '信息差'],
    '感情线': ['感情线', '情感', '温度', '升温', '试探', '互动', '名不副实', '微光', '关系线', 'CP'],
    '伏笔/铺垫': ['伏笔', '回收', '埋线', '断线', '暗示', '解密', '魔门', '内门', '末日', '线'],
    '逻辑/自洽': ['矛盾', '自洽', '证据', '时间', '时序', '苔藓', '语义', '因果', '坐标', '地图',
                '巧合', '恰好', '逻辑', '漏洞', '冲突'],
    '爽点/弃书': ['弃书', '爽点', '捡宝', '声望', '高光', '兑现', '追读', '金句',
               '排比', '模板', 'AI味', '设计感', '平均化', '安全', '机味'],
    '结构/商业': ['结构', '商业', '预期', '承诺', '卷结构', '高潮', '钩子', '付费', '连载', '读者'],
    '倒计时/时序': ['倒计时', '30天', '天数', '剩余', '时钟'],
    '具体性': ['具体', '细节', '感官', '数字', '抽象', '说明书', '旁白', '空泛', '物证'],
}

def classify(text):
    hits = []
    for theme, kws in THEMES.items():
        if any(kw in text for kw in kws):
            hits.append(theme)
    return hits or ['其他']

def norm(s):
    return ''.join(c for c in s.lower() if c.isalnum() or '\u4e00' <= c <= '\u9fff')

def jaccard(a, b):
    sa, sb = set(a), set(b)
    if not sa or not sb:
        return 0.0
    return len(sa & sb) / len(sa | sb)

# ============ 多轮 review ============
def run_review(binary, project, volume, model, focus_name, focus_text, out_path, log_path):
    """跑一轮 review。focus_text 为 None 时使用内置 --focus (novelgen >= 内置版)。"""
    cmd = [binary, 'compose', 'review', '--agent-sdk', '--volume', str(volume)]
    if model:
        cmd += ['--model', model]
    if focus_text is not None:
        # 外部 focus 文件: 通过 --prompt 传入, 追加命令规范
        prompt = focus_text + """
【重要命令规范】每次只能执行一条 novelgen 命令。严禁使用 &&、||、;、| 或 echo 把多条命令串联。一次查询只查一个目标。"""
        cmd += ['--prompt', prompt]
    else:
        # 内置 focus: 用 novelgen 的 --focus 参数
        cmd += ['--focus', focus_name]
    cmd += ['--out', out_path]
    with open(log_path, 'w') as f:
        result = subprocess.run(cmd, cwd=project, stdout=f, stderr=subprocess.STDOUT)
    return result.returncode

def collect_suggestions(reviews_dir):
    """收集所有 review JSON 的 suggestions"""
    all_sugs = []
    for fname in sorted(os.listdir(reviews_dir)):
        if not fname.endswith('.json'):
            continue
        path = os.path.join(reviews_dir, fname)
        try:
            d = json.load(open(path))
        except Exception:
            continue
        score = d.get('overall_score')
        for s in d.get('suggestions', []):
            s2 = dict(s)
            s2['_src'] = fname.replace('.json', '')
            s2['_review_score'] = score
            s2['_themes'] = classify(s.get('issue', '') + ' ' + s.get('suggestion', ''))
            all_sugs.append(s2)
    return all_sugs

# ============ 分层抽样 ============
def stratified_sample(sugs, n, seed=42):
    """按主主题分层: 每主题保底1条, 随机补足到 n 条"""
    random.seed(seed)
    by_theme = {}
    for idx, s in enumerate(sugs):
        primary = s['_themes'][0] if s['_themes'] else '其他'
        by_theme.setdefault(primary, []).append(idx)
    picked = []
    remaining = []
    for theme, idxs in by_theme.items():
        if idxs:
            picked.append(idxs[0])
            remaining.extend(idxs[1:])
    while len(picked) < n and remaining:
        idx = random.choice(remaining)
        remaining.remove(idx)
        picked.append(idx)
    picked.sort()
    return [sugs[i] for i in picked]

def pure_random_sample(sugs, n, seed=42):
    random.seed(seed)
    idxs = sorted(random.sample(range(len(sugs)), min(n, len(sugs))))
    return [sugs[i] for i in idxs]

# ============ 合理化 prompt ============
def build_rationalize_prompt(items):
    lines = [
        "以下是从多次独立审查中抽取的大纲修改建议。请对它们进行'合理化'裁决，输出整合后的终审建议：",
        "1) 合并内容重叠或指向同一问题的建议（保留实质，去掉重复措辞）；",
        "2) 对互相冲突的建议必须做出取舍裁决，并说明取舍理由，禁止和稀泥式融合；",
        "3) 拒绝明显低价值、不可操作或空泛的建议；",
        "4) 每条终审建议必须保留 target_id/目标位置、具体问题和可操作的修改方向；",
        "5) 终审建议总数不超过 8 条，按优先级排序。",
        "",
        "原始建议列表：",
    ]
    for i, it in enumerate(items, 1):
        tgt = it.get('target_name') or it.get('target_id') or '?'
        lines.append(f"{i}. [{it.get('priority','')}] {tgt}: {it.get('issue','')}")
        if it.get('suggestion'):
            lines.append(f"   建议: {it['suggestion']}")
    return "\n".join(lines)

def main():
    ap = argparse.ArgumentParser(description='多轮 focus review 矩阵')
    ap.add_argument('--project', required=True, help='novelgen 项目目录')
    ap.add_argument('--binary', default=os.path.expanduser('~/novelgen-shallow/bin/novelgen'), help='novelgen 二进制路径')
    ap.add_argument('--volume', type=int, default=0, help='审查卷号(1-based), 0=整本')
    ap.add_argument('--models', default='deepseek-v4-flash,minimax-m3,qwen3.6-plus', help='逗号分隔模型列表')
    ap.add_argument('--focus', default='all', help="focus 选择: all=全部 focus 文件, 或逗号分隔文件名 (如 06_deai,02_logic)")
    ap.add_argument('--focus-dir', default=os.path.expanduser('~/novelgen-shallow/scripts/review-focus'), help='focus 文件目录')
    ap.add_argument('--out-dir', default='/tmp/review_matrix_out', help='输出目录')
    ap.add_argument('--sample', type=int, default=10, help='分层抽样条数')
    ap.add_argument('--seed', type=int, default=42, help='随机种子')
    ap.add_argument('--parallel', type=int, default=5, help='并行 review 数')
    ap.add_argument('--sample-mode', choices=['stratified', 'random'], default='stratified',
                    help='抽样方式: stratified(推荐) 或 random')
    args = ap.parse_args()

    os.makedirs(args.out_dir, exist_ok=True)
    reviews_dir = os.path.join(args.out_dir, 'reviews')
    os.makedirs(reviews_dir, exist_ok=True)

    # 选择 focus: 优先用内置 --focus, 回退到 focus 文件目录
    models = [m.strip() for m in args.models.split(',') if m.strip()]
    if not models:
        print("错误: --models 不能为空")
        sys.exit(1)
    builtin_focuses = ['reader', 'logic', 'character', 'commercial', 'storyline', 'deai']
    focus_files = None
    if args.focus != 'all':
        # 检查是否全是内置 focus 名
        focus_names = [n.strip() for n in args.focus.split(',')]
        if all(n in builtin_focuses for n in focus_names):
            # 内置 focus: 直接把名字作为 focus, 不用文件
            focus_files = None
            focus_names_used = focus_names
        else:
            focus_files = sorted(os.listdir(args.focus_dir))
            focus_files = [f for f in focus_files if f in args.focus.split(',') or
                           os.path.splitext(f)[0] in args.focus.split(',')]
            focus_names_used = [os.path.splitext(f)[0] for f in focus_files]
    else:
        focus_names_used = builtin_focuses

    if focus_files is not None:
        if not focus_files:
            print(f"错误: 没有匹配的 focus 文件 (--focus {args.focus})")
            sys.exit(1)
    print(f"=== review 矩阵: {len(models)} 模型 × {len(focus_names_used)} focus = {len(models)*len(focus_names_used)} 轮 ===")

    tasks = []
    for m in models:
        for fname in focus_names_used:
            name = f"{m}_{fname}"
            out_path = os.path.join(reviews_dir, f"{name}.json")
            log_path = os.path.join(reviews_dir, f"{name}.log")
            if focus_files is not None:
                focus_text = open(os.path.join(args.focus_dir, fname + '.txt'), encoding='utf-8').read()
            else:
                # 内置 focus: 不传 --prompt, 用 novelgen 的 --focus 参数
                focus_text = None
            tasks.append((name, m, fname, focus_text, out_path, log_path))

    with ThreadPoolExecutor(max_workers=args.parallel) as ex:
        done = 0
        results = {}
        fs = {ex.submit(run_review, args.binary, args.project, args.volume, m,
                        fn, ft, op, lp): n for (n, m, fn, ft, op, lp) in tasks}
        for fut in as_completed(fs):
            n = fs[fut]
            rc = fut.result()
            results[n] = rc
            done += 1
            print(f"  [{done}/{len(tasks)}] {n}: exit={rc}")

    ok = sum(1 for rc in results.values() if rc == 0)
    print(f"\n成功 {ok}/{len(tasks)} 轮")

    # 收集建议
    sugs = collect_suggestions(reviews_dir)
    print(f"合并建议总数: {len(sugs)}")
    if not sugs:
        print("没有收集到任何建议, 退出")
        sys.exit(1)

    # 主题分布
    theme_counter = Counter()
    for s in sugs:
        for t in s['_themes']:
            theme_counter[t] += 1
    print("\n主题分布:")
    for t, c in theme_counter.most_common():
        print(f"  {t}: {c}")

    # 同质化统计: 字面重复
    norms = [norm(s.get('issue','') + s.get('suggestion','')) for s in sugs]
    dup = 0
    seen = set()
    for n in norms:
        if n in seen:
            dup += 1
        seen.add(n)
    print(f"\n字面重复建议: {dup}/{len(sugs)}")

    # 保存全部建议
    all_path = os.path.join(args.out_dir, 'all_suggestions.json')
    json.dump(sugs, open(all_path, 'w'), ensure_ascii=False, indent=1)
    print(f"已保存全部建议: {all_path}")

    # 抽样
    if args.sample_mode == 'stratified':
        sampled = stratified_sample(sugs, args.sample, args.seed)
        mode_label = '分层抽样'
    else:
        sampled = pure_random_sample(sugs, args.sample, args.seed)
        mode_label = '纯随机抽样'
    sample_path = os.path.join(args.out_dir, f'sampled_{args.sample}.json')
    json.dump(sampled, open(sample_path, 'w'), ensure_ascii=False, indent=1)
    print(f"\n{mode_label} {len(sampled)} 条 → {sample_path}")

    # 主题覆盖
    sample_themes = Counter(s['_themes'][0] for s in sampled)
    print(f"抽样主题覆盖: {len(sample_themes)} 个: {dict(sample_themes)}")

    # 合理化 prompt
    rp = build_rationalize_prompt(sampled)
    rp_path = os.path.join(args.out_dir, 'rationalize_prompt.txt')
    open(rp_path, 'w', encoding='utf-8').write(rp)
    print(f"合理化 prompt → {rp_path}")

    # 汇总报告
    report = os.path.join(args.out_dir, 'matrix_report.txt')
    with open(report, 'w', encoding='utf-8') as f:
        f.write(f"review 矩阵: {len(models)} 模型 × {len(focus_names_used)} focus = {len(tasks)} 轮\n")
        f.write(f"成功: {ok}/{len(tasks)}\n")
        f.write(f"合并建议: {len(sugs)}, 字面重复: {dup}\n")
        f.write(f"主题分布: {dict(theme_counter)}\n")
        f.write(f"抽样({mode_label}): {len(sampled)}, 主题覆盖 {len(sample_themes)}: {dict(sample_themes)}\n")
    print(f"汇总报告 → {report}")
    print("\n下一步建议:")
    print(f"  合理化: novelgen compose review --agent-sdk --volume {args.volume} --prompt \"$(cat {rp_path})\"")
    print(f"  improve: novelgen compose improve --agent-sdk --volume {args.volume} --suggestions {sample_path}")

if __name__ == '__main__':
    main()
