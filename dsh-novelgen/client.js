window.__ModuleLoader__.load({
  id: '@local/dsh-novelgen',
  factory(require) {
    const React = require('react');
    const h = React.createElement;
    const API = '/plugins/novelgen';

    const LABELS = {
      zh: {
        title: 'Novelgen 创作台',
        open: '打开创作台',
        close: '收起',
        refresh: '刷新',
        flow: '创作流',
        resources: '资源',
        graph: '关系',
        next: '下一步',
        noNext: '暂无下一步',
        prepare: '准备委托',
        nextDirect: '运行管线',
        project: '项目',
        chooseProject: '选择项目',
        noProject: '未找到 Novelgen 项目',
        noProjectHint: '把会话打开到 novel.json 所在目录，或在上方选择项目。',
        loading: '加载中…',
        error: '出错了',
        retry: '重试',
        search: '搜索资源',
        allKinds: '全部类型',
        onlyOpen: '仅未完成',
        selectFirst: '先选择至少一个资源',
        selectLimit: '最多选择 20 个资源',
        insert: '插入输入框',
        insertHint: '选择资源后会把一条 select 提示插入主会话输入框。',
        insertConflict: '输入框状态已变化，请重试或复制下面提示。',
        noInput: '当前输入框不可插入，已生成提示供复制。',
        selection: '已选',
        intent: '意图',
        note: '补充要求（可选）',
        clear: '清空',
        remove: '移除',
        addSelect: '加入选择',
        detail: '详情',
        preview: '预览',
        tabLabel: '小说',
        viewSubtitle: '创作流 · 资源 · 关系',
        stagePreview: '阶段预览',
        stagePreviewHint: '点击查看该阶段预览',
        prepareStage: '准备此阶段',
        previewLoading: '加载预览…',
        previewFailed: '预览加载失败',
        previewRetry: '重试',
        previewEmpty: '暂无可预览内容',
        previewEmptyHint: '该资源还没有生成可预览的内容，可以先准备对应的创作任务。',
        previewPartialEvents: '只显示前 {n} 条事件，展开原始数据查看全部。',
        previewTruncated: '预览已截断，展开原始数据查看完整内容。',
        rawData: '原始数据',
        summary: '摘要',
        genres: '类型',
        premise: '核心设定',
        theme: '主题',
        tone: '基调',
        pov: '视角',
        audience: '目标读者',
        coreCast: '核心角色',
        storylines: '故事线',
        rules: '规则',
        payoff: '卷承诺',
        events: '事件',
        conflict: '冲突',
        pacing: '节奏',
        chapterPayoff: '章节兑现',
        chapterText: '正文',
        role: '定位',
        age: '年龄',
        gender: '性别',
        powerLevel: '等级',
        affiliations: '归属',
        background: '背景',
        motivation: '动机',
        personality: '性格',
        skills: '技能',
        abilities: '能力',
        voice: '语气',
        description: '描述',
        function: '功能',
        category: '类别',
        rarity: '稀有度',
        owner: '拥有者',
        leader: '领袖',
        goals: '目标',
        atmosphere: '氛围',
        history: '历史',
        powers: '能力/效果',
        limitations: '限制',
        significance: '意义',
        notes: '备注',
        plotBeats: '情节节拍',
        lastLine: '最后一句',
        nextOpening: '下一章开场',
        type: '类型',
        importance: '重要度',
        location: '地点',
        desire: '欲望',
        pressure: '压力',
        cleverMove: '巧招',
        payoffMoment: '兑现时刻',
        reward: '奖励',
        hook: '钩子',
        socialProof: '社会证明',
        path: '路径',
        words: '字',
        chapters: '章',
        volumes: '卷',
        characters: '角色',
        graphEmpty: '选择一个资源查看关系',
        graphHint: '点击节点切换中心；连线表示包含、出场、地点、物品、推进或 recap 关系。',
        nextHint: '主对话负责推进；具体创作交给 subagent。',
        projectSwitch: '切换项目',
        openProject: '打开项目',
        copy: '复制',
        copied: '已复制',
        stage_setup: '设定',
        stage_outline: '大纲',
        stage_craft: '设定集',
        stage_write: '正文',
        stage_recap: '连续性',
        stage_review: '审校',
        kind_setup: '设定',
        kind_part: '部',
        kind_volume: '卷',
        kind_chapter: '章节',
        kind_character: '角色',
        kind_location: '地点',
        kind_item: '物品',
        kind_organization: '组织',
        kind_storyline: '故事线',
        kind_recap: 'Recap',
        status_done: '完成',
        status_active: '进行中',
        status_pending: '待开始',
        status_missing: '缺失',
        status_mention: '提及',
        status_written: '已写',
        status_unwritten: '未写',
        intent_auto: '自动判断',
        intent_write: '续写正文',
        intent_improve: '改进内容',
        intent_review: '审校',
        intent_plan: '规划结构',
        intent_recap: '更新 Recap',
        intent_ask: '只讨论',
        noCandidates: '没有可用的项目路径',
        nextReason: '原因',
        prepareResource: '准备此资源',
        runNext: '运行下一步',
        runStage: '运行此阶段',
        runResource: '直接运行',
        runSelection: '运行所选',
        insertOnly: '只插入提示',
        runBatch: '连续运行',
        batchSteps: '连续步数',
        stepsCount: '{n} 步',
        runSubmitted: '已提交，主对话会继续执行。',
        runUnavailable: '当前输入框不支持自动提交，请用「只插入提示」。',
        refreshing: '刷新中…',
        updatedAt: '更新于 {time}',
        recentChanges: '最近变更',
        filterVolume: '按卷筛选',
        allVolumes: '全部卷',
        filterStatus: '按状态筛选',
        allStatus: '全部状态',
        statusUnfinished: '未完成',
        statusWrittenFilter: '已写正文',
        statusUnwrittenFilter: '未写正文',
        resourceCount: '{n} 个资源',
        loadMore: '加载更多（还有 {n} 个）',
        promptFallback: '提示已生成，可复制到输入框：',
        closePanel: '关闭',
        writtenProgress: '正文 {a}/{b}',
        craftProgress: '设定集 {a}/{b}',
        emptyList: '没有匹配的资源',
        noFocus: '从资源或关系页选择一个资源',
        totalResources: '{n} 个资源',
        graphCenter: '中心',
        prev: '上一个',
        nextPage: '下一个',
        stageAction: '为这个阶段准备下一步',
        manualHint: '也可以直接在对话里说：继续 novelgen 的下一步。'
      },
      en: {
        title: 'Novelgen Studio',
        open: 'Open studio',
        close: 'Collapse',
        refresh: 'Refresh',
        flow: 'Flow',
        resources: 'Resources',
        graph: 'Graph',
        next: 'Next',
        noNext: 'No next step',
        prepare: 'Prepare',
        nextDirect: 'Run pipeline',
        project: 'Project',
        chooseProject: 'Choose project',
        noProject: 'No Novelgen project found',
        noProjectHint: 'Open the session in the directory that contains novel.json, or pick a project above.',
        loading: 'Loading…',
        error: 'Something went wrong',
        retry: 'Retry',
        search: 'Search resources',
        allKinds: 'All kinds',
        onlyOpen: 'Only unfinished',
        selectFirst: 'Select at least one resource first',
        selectLimit: 'Select at most 20 resources',
        insert: 'Insert into input',
        insertHint: 'Selected resources become a select prompt in the main conversation input.',
        insertConflict: 'The composer changed. Retry or copy the generated prompt below.',
        noInput: 'The composer is not writable here; the prompt was generated for copying.',
        selection: 'Selected',
        intent: 'Intent',
        note: 'Extra instruction (optional)',
        clear: 'Clear',
        remove: 'Remove',
        addSelect: 'Add to selection',
        detail: 'Detail',
        preview: 'Preview',
        tabLabel: 'Novel',
        viewSubtitle: 'Flow · resources · relationships',
        stagePreview: 'Stage preview',
        stagePreviewHint: 'Click to preview this stage',
        prepareStage: 'Prepare stage',
        previewLoading: 'Loading preview…',
        previewFailed: 'Preview failed',
        previewRetry: 'Retry',
        previewEmpty: 'Nothing to preview yet',
        previewEmptyHint: 'This resource has no generated content yet; prepare the corresponding creation task first.',
        previewPartialEvents: 'Showing the first {n} events; open the raw data for the rest.',
        previewTruncated: 'Preview truncated; open the raw data for the full content.',
        rawData: 'Raw data',
        summary: 'Summary',
        genres: 'Genres',
        premise: 'Premise',
        theme: 'Theme',
        tone: 'Tone',
        pov: 'POV',
        audience: 'Audience',
        coreCast: 'Core cast',
        storylines: 'Storylines',
        rules: 'Rules',
        payoff: 'Payoff contract',
        events: 'Events',
        conflict: 'Conflict',
        pacing: 'Pacing',
        chapterPayoff: 'Chapter payoff',
        chapterText: 'Chapter text',
        role: 'Role',
        age: 'Age',
        gender: 'Gender',
        powerLevel: 'Level',
        affiliations: 'Affiliations',
        background: 'Background',
        motivation: 'Motivation',
        personality: 'Personality',
        skills: 'Skills',
        abilities: 'Abilities',
        voice: 'Voice',
        description: 'Description',
        function: 'Function',
        category: 'Category',
        rarity: 'Rarity',
        owner: 'Owner',
        leader: 'Leader',
        goals: 'Goals',
        atmosphere: 'Atmosphere',
        history: 'History',
        powers: 'Powers',
        limitations: 'Limitations',
        significance: 'Significance',
        notes: 'Notes',
        plotBeats: 'Plot beats',
        lastLine: 'Last line',
        nextOpening: 'Next opening',
        type: 'Type',
        importance: 'Importance',
        location: 'Location',
        desire: 'Desire',
        pressure: 'Pressure',
        cleverMove: 'Clever move',
        payoffMoment: 'Payoff moment',
        reward: 'Reward',
        hook: 'Hook',
        socialProof: 'Social proof',
        path: 'Path',
        words: 'chars',
        chapters: 'chapters',
        volumes: 'volumes',
        characters: 'characters',
        graphEmpty: 'Select a resource to see its relationships',
        graphHint: 'Click a node to re-center. Edges mean contains, features, location, item, advance, or recap.',
        nextHint: 'The main conversation drives the loop; concrete creation goes to a subagent.',
        projectSwitch: 'Switch project',
        openProject: 'Open project',
        copy: 'Copy',
        copied: 'Copied',
        stage_setup: 'Setup',
        stage_outline: 'Outline',
        stage_craft: 'Craft',
        stage_write: 'Manuscript',
        stage_recap: 'Continuity',
        stage_review: 'Review',
        kind_setup: 'Setup',
        kind_part: 'Part',
        kind_volume: 'Volume',
        kind_chapter: 'Chapter',
        kind_character: 'Character',
        kind_location: 'Location',
        kind_item: 'Item',
        kind_organization: 'Organization',
        kind_storyline: 'Storyline',
        kind_recap: 'Recap',
        status_done: 'Done',
        status_active: 'In progress',
        status_pending: 'Pending',
        status_missing: 'Missing',
        status_mention: 'Mentioned',
        status_written: 'Written',
        status_unwritten: 'Unwritten',
        intent_auto: 'Auto',
        intent_write: 'Write',
        intent_improve: 'Improve',
        intent_review: 'Review',
        intent_plan: 'Plan',
        intent_recap: 'Update recap',
        intent_ask: 'Discuss only',
        noCandidates: 'No project paths found',
        nextReason: 'Reason',
        prepareResource: 'Prepare this resource',
        runNext: 'Run next step',
        runStage: 'Run this stage',
        runResource: 'Run this resource',
        runSelection: 'Run selection',
        insertOnly: 'Insert prompt only',
        runBatch: 'Run batch',
        batchSteps: 'Batch steps',
        stepsCount: '{n} steps',
        runSubmitted: 'Submitted; the main conversation will continue.',
        runUnavailable: 'This composer cannot auto-submit; use “Insert prompt only”.',
        refreshing: 'Refreshing…',
        updatedAt: 'Updated {time}',
        recentChanges: 'Recent changes',
        filterVolume: 'Filter by volume',
        allVolumes: 'All volumes',
        filterStatus: 'Filter by status',
        allStatus: 'All statuses',
        statusUnfinished: 'Unfinished',
        statusWrittenFilter: 'Written',
        statusUnwrittenFilter: 'Unwritten',
        resourceCount: '{n} resources',
        loadMore: 'Load more ({n} remaining)',
        promptFallback: 'Prompt generated; copy it into the input:',
        closePanel: 'Close',
        writtenProgress: 'Manuscript {a}/{b}',
        craftProgress: 'Craft {a}/{b}',
        emptyList: 'No matching resources',
        noFocus: 'Pick a resource on the Resources or Graph tab',
        totalResources: '{n} resources',
        graphCenter: 'Center',
        prev: 'Previous',
        nextPage: 'Next',
        stageAction: 'Prepare the next step for this stage',
        manualHint: 'You can also type: continue the next novelgen step.'
      }
    };

    function interpolate(text, params) {
      if (!params) return text;
      let value = text;
      for (const [key, replacement] of Object.entries(params)) {
        value = value.split(`{${key}}`).join(String(replacement));
      }
      return value;
    }

    function makeT(ctx) {
      const zh = LABELS.zh;
      const en = LABELS.en;
      let bound = null;
      if (ctx.locale) {
        try {
          ctx.effect(() => {
            const offs = [];
            for (const [locale, dict] of [
              ['zh', zh],
              ['en', en]
            ]) {
              try {
                const off = ctx.locale.register('novelgen', locale, dict);
                if (typeof off === 'function') offs.push(off);
              } catch {
                // Older locale services may require the batch form; fallback below.
              }
            }
            return () => {
              for (const off of offs) {
                try {
                  off();
                } catch {
                  // already disposed
                }
              }
            };
          });
          bound = ctx.locale.bind('novelgen');
        } catch {
          bound = null;
        }
      }
      const isZh = typeof navigator !== 'undefined' && /^zh/i.test(navigator.language || '');
      return (key, params) => {
        if (bound) {
          try {
            const value = bound(key);
            if (typeof value === 'string' && value && value !== key) return interpolate(value, params);
          } catch {
            // fall through to the local dictionary
          }
        }
        return interpolate(isZh ? zh[key] : en[key], params) || key;
      };
    }

    function kindLabel(t, kind) {
      const key = `kind_${kind}`;
      return t(key);
    }

    function statusLabel(t, status) {
      if (status === 'done') return t('status_done');
      if (status === 'active') return t('status_active');
      if (status === 'missing') return t('status_missing');
      if (status === 'mention') return t('status_mention');
      return t('status_pending');
    }

    function stageLabel(t, stage) {
      return t(`stage_${stage}`) || stage || '';
    }

    function formatNumber(value) {
      const number = Number(value);
      if (!Number.isFinite(number)) return '0';
      return number.toLocaleString();
    }

    function short(text, limit) {
      const value = String(text || '');
      return value.length > limit ? `${value.slice(0, Math.max(0, limit - 1))}…` : value;
    }

    function formatTime(value) {
      try {
        const date = value instanceof Date ? value : new Date(value);
        if (Number.isNaN(date.getTime())) return '';
        return date.toLocaleString(undefined, {
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit'
        });
      } catch {
        return '';
      }
    }

    const API_TIMEOUT_MS = 30000;

    async function api(pathname, options = {}) {
      const controller = typeof AbortController === 'function' ? new AbortController() : null;
      const timer = controller ? setTimeout(() => controller.abort(), API_TIMEOUT_MS) : null;
      try {
        const response = await fetch(`${API}${pathname}`, {
          credentials: 'same-origin',
          headers: options.body ? { 'content-type': 'application/json' } : undefined,
          ...options,
          signal: controller ? controller.signal : options.signal
        });
        let json;
        try {
          json = await response.json();
        } catch {
          throw new Error(`HTTP ${response.status}`);
        }
        if (response.status === 401) {
          throw new Error(json && json.error ? json.error : 'authentication required');
        }
        if (!response.ok && (!json || json.ok !== false)) {
          throw new Error(json && json.error ? json.error : `HTTP ${response.status}`);
        }
        return json;
      } catch (error) {
        if (error && error.name === 'AbortError') throw new Error('request timed out');
        throw error;
      } finally {
        if (timer) clearTimeout(timer);
      }
    }

    const CSS = `
      .ng-root { position: relative; font-size: 12px; line-height: 1.45; color: var(--dsw-alias-label-primary); }
      .ng-bar { display: flex; align-items: center; gap: 8px; min-height: 34px; padding: 4px 8px;
        border: 1px solid var(--dsw-alias-border-l1); border-radius: 10px;
        background: var(--dsw-alias-bg-layer-1); }
      .ng-mark { width: 22px; height: 22px; border-radius: 6px; background: var(--dsw-alias-brand-primary);
        color: #fff; display: inline-flex; align-items: center; justify-content: center;
        font-weight: 700; font-size: 11px; letter-spacing: .02em; }
      .ng-name { font-weight: 600; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-spacer { flex: 1 1 auto; }
      .ng-muted { color: var(--dsw-alias-label-secondary); }
      .ng-btn { appearance: none; border: 1px solid var(--dsw-alias-border-l1);
        background: var(--dsw-alias-bg-layer-2); color: var(--dsw-alias-label-primary);
        border-radius: 7px; padding: 4px 9px; font-size: 12px; line-height: 18px; cursor: pointer; }
      .ng-btn:hover { border-color: var(--dsw-alias-border-l2); }
      .ng-btn:disabled { opacity: .5; cursor: default; }
      .ng-btn-primary { background: var(--dsw-alias-brand-primary); border-color: var(--dsw-alias-brand-primary); color: #fff; }
      .ng-panel { margin-top: 6px; border: 1px solid var(--dsw-alias-border-l1); border-radius: 12px;
        background: var(--dsw-alias-bg-layer-1); overflow: hidden; }
      .ng-head { display: flex; align-items: center; gap: 8px; padding: 8px 10px;
        border-bottom: 1px solid var(--dsw-alias-border-l1); flex-wrap: wrap; }
      .ng-tabs { display: flex; gap: 2px; padding: 6px 8px 0; border-bottom: 1px solid var(--dsw-alias-border-l1); }
      .ng-tab { border: 0; background: transparent; color: var(--dsw-alias-label-secondary);
        padding: 5px 10px; border-radius: 7px 7px 0 0; cursor: pointer; font-size: 12px; }
      .ng-tab.active { color: var(--dsw-alias-label-primary); background: var(--dsw-alias-bg-layer-2); }
      .ng-body { padding: 10px; max-height: 46vh; overflow: auto; }
      .ng-stages { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 8px; }
      .ng-stage { border: 1px solid var(--dsw-alias-border-l1); border-radius: 9px; padding: 8px;
        background: var(--dsw-alias-bg-layer-2); cursor: pointer; min-width: 0; }
      .ng-stage:hover { border-color: var(--dsw-alias-border-l2); }
      .ng-stage.done { border-color: var(--dsw-alias-state-success-primary); }
      .ng-stage.active { border-color: var(--dsw-alias-state-warn-primary); }
      .ng-stage-top { display: flex; align-items: center; gap: 6px; }
      .ng-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--dsw-alias-state-idle-primary); flex: none; }
      .ng-dot.done { background: var(--dsw-alias-state-success-primary); }
      .ng-dot.active { background: var(--dsw-alias-state-warn-primary); }
      .ng-stage-name { font-weight: 600; }
      .ng-stage-count { margin-left: auto; font-variant-numeric: tabular-nums; color: var(--dsw-alias-label-secondary); }
      .ng-stage-detail { margin-top: 3px; color: var(--dsw-alias-label-secondary); font-size: 11px;
        overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-next { margin-top: 10px; border: 1px solid var(--dsw-alias-brand-primary); border-radius: 10px;
        padding: 9px 10px; background: var(--dsw-alias-bg-layer-2); }
      .ng-next-title { font-weight: 600; display: flex; align-items: center; gap: 6px; }
      .ng-next-body { margin-top: 4px; color: var(--dsw-alias-label-secondary); }
      .ng-actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
      .ng-toolbar { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; margin-bottom: 8px; }
      .ng-toolbar .ng-select { max-width: 190px; }
      .ng-recent .ng-list { max-height: 220px; }
      .ng-input, .ng-select { border: 1px solid var(--dsw-alias-border-l1);
        background: var(--dsw-alias-bg-layer-1); color: var(--dsw-alias-label-primary);
        border-radius: 7px; padding: 5px 8px; font-size: 12px; line-height: 18px; outline: none; max-width: 100%; }
      .ng-input:focus, .ng-select:focus { border-color: var(--dsw-alias-brand-primary); }
      .ng-split { display: grid; grid-template-columns: minmax(0, 1.05fr) minmax(0, .95fr);
        gap: 0; border: 1px solid var(--dsw-alias-border-l1); border-radius: 10px;
        overflow: hidden; background: var(--dsw-alias-bg-base); }
      .ng-split > .ng-list { border: 0; border-radius: 0; }
      .ng-split > .ng-detail { border: 0; border-radius: 0; background: var(--dsw-alias-bg-layer-2); }
      @media (max-width: 760px) {
        .ng-split { grid-template-columns: 1fr; }
        .ng-split > .ng-detail { border-top: 1px solid var(--dsw-alias-border-l1); }
      }
      .ng-row { display: flex; gap: 8px; align-items: flex-start; padding: 6px 8px;
        border-bottom: 1px solid var(--dsw-alias-border-l1); cursor: pointer; min-width: 0; }
      .ng-row:last-child { border-bottom: 0; }
      .ng-row:hover { background: var(--dsw-alias-bg-layer-2); }
      .ng-row.active { background: var(--dsw-alias-bg-layer-2);
        box-shadow: inset 2px 0 0 var(--dsw-alias-brand-primary); }
      .ng-check { margin-top: 3px; flex: none; accent-color: var(--dsw-alias-brand-primary); }
      .ng-row-main { min-width: 0; flex: 1 1 auto; }
      .ng-row-title { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-row-sub { color: var(--dsw-alias-label-secondary); font-size: 11px;
        overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-badge { display: inline-flex; align-items: center; border-radius: 999px; padding: 0 6px;
        font-size: 10px; line-height: 16px; background: var(--dsw-alias-bg-base);
        border: 1px solid var(--dsw-alias-border-l1); color: var(--dsw-alias-label-secondary); white-space: nowrap; }
      .ng-badge.done { color: var(--dsw-alias-state-success-primary); }
      .ng-badge.missing { color: var(--dsw-alias-state-warn-primary); }
      .ng-badge.active { color: var(--dsw-alias-state-warn-primary); }
      .ng-detail { border: 1px solid var(--dsw-alias-border-l1); border-radius: 12px;
        background: var(--dsw-alias-bg-base); min-width: 0; overflow: hidden; }
      .ng-detail-head { padding: 12px 14px; background: var(--dsw-alias-bg-layer-2);
        border-bottom: 1px solid var(--dsw-alias-border-l1); }
      .ng-detail-kicker { display: flex; align-items: center; gap: 6px; color: var(--dsw-alias-label-secondary);
        font-size: 10px; letter-spacing: .06em; text-transform: uppercase; }
      .ng-kind-dot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
      .ng-detail-title { margin-top: 5px; font-size: 17px; font-weight: 650; line-height: 1.3; }
      .ng-detail-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px;
        margin-top: 5px; color: var(--dsw-alias-label-secondary); font-size: 11px; }
      .ng-path { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 10px;
        background: var(--dsw-alias-bg-base); border: 1px solid var(--dsw-alias-border-l1);
        border-radius: 5px; padding: 1px 6px; }
      .ng-detail-actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 9px; }
      .ng-detail-body { padding: 12px 14px; }
      .ng-preview-hero { padding: 12px 14px; border: 1px solid var(--dsw-alias-border-l1);
        border-radius: 10px; background: var(--dsw-alias-bg-layer-2); }
      .ng-lead { font-size: 13px; line-height: 1.7; white-space: pre-wrap; word-break: break-word; }
      .ng-chip-row { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
      .ng-tag { display: inline-flex; align-items: center; padding: 2px 8px; border-radius: 999px;
        border: 1px solid var(--dsw-alias-border-l1); background: var(--dsw-alias-bg-base);
        color: var(--dsw-alias-label-secondary); font-size: 11px; line-height: 17px; }
      button.ng-tag { color: var(--dsw-alias-label-primary); cursor: pointer; font: inherit; }
      button.ng-tag:hover { border-color: var(--dsw-alias-brand-primary); color: var(--dsw-alias-brand-primary); }
      .ng-stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(84px, 1fr));
        gap: 6px; margin-top: 10px; }
      .ng-stat { border: 1px solid var(--dsw-alias-border-l1); border-radius: 8px; padding: 7px 9px;
        background: var(--dsw-alias-bg-base); min-width: 0; }
      .ng-stat-value { font-size: 15px; font-weight: 650; line-height: 1.3; font-variant-numeric: tabular-nums; }
      .ng-stat-label { margin-top: 1px; color: var(--dsw-alias-label-secondary); font-size: 10px;
        overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 8px; margin-top: 10px; }
      .ng-card { border: 1px solid var(--dsw-alias-border-l1); border-radius: 8px; padding: 8px 10px;
        background: var(--dsw-alias-bg-base); min-width: 0; }
      .ng-card-label { color: var(--dsw-alias-label-secondary); font-size: 10px; letter-spacing: .03em; }
      .ng-card-value { margin-top: 3px; font-size: 12px; line-height: 1.6; white-space: pre-wrap;
        word-break: break-word; }
      .ng-preview-section { margin-top: 14px; }
      .ng-section-title { margin-bottom: 6px; color: var(--dsw-alias-label-secondary);
        font-size: 11px; font-weight: 650; letter-spacing: .04em; }
      .ng-list { border: 1px solid var(--dsw-alias-border-l1); border-radius: 10px;
        max-height: 320px; overflow: auto; background: var(--dsw-alias-bg-base); }
      .ng-list-row { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; gap: 8px;
        align-items: center; width: 100%; padding: 8px 10px; border: 0;
        border-bottom: 1px solid var(--dsw-alias-border-l1); background: transparent; color: inherit;
        text-align: left; font: inherit; }
      .ng-list-row:last-child { border-bottom: 0; }
      .ng-list-row:not(.ng-list-row-static) { cursor: pointer; }
      .ng-list-row:not(.ng-list-row-static):hover { background: var(--dsw-alias-bg-layer-2); }
      .ng-list-index { color: var(--dsw-alias-label-secondary); font-size: 10px; font-variant-numeric: tabular-nums; }
      .ng-list-main { min-width: 0; }
      .ng-list-title { font-size: 12px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-list-sub { margin-top: 1px; color: var(--dsw-alias-label-secondary); font-size: 11px;
        overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-entity-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 6px; }
      .ng-entity { border: 1px solid var(--dsw-alias-border-l1); border-radius: 8px; padding: 7px 9px;
        background: var(--dsw-alias-bg-base); color: inherit; text-align: left; font: inherit; min-width: 0; }
      .ng-entity-name { font-size: 12px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ng-entity-role { margin-top: 2px; color: var(--dsw-alias-label-secondary); font-size: 11px;
        overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      button.ng-entity { cursor: pointer; }
      button.ng-entity:hover { border-color: var(--dsw-alias-brand-primary); }
      .ng-timeline { position: relative; margin-left: 4px; padding-left: 14px;
        border-left: 1px solid var(--dsw-alias-border-l2); }
      .ng-timeline-item { position: relative; padding-bottom: 12px; }
      .ng-timeline-item:last-child { padding-bottom: 0; }
      .ng-timeline-dot { position: absolute; left: -18px; top: 4px; width: 7px; height: 7px;
        border-radius: 50%; background: var(--dsw-alias-brand-primary);
        box-shadow: 0 0 0 3px var(--dsw-alias-bg-base); }
      .ng-timeline-title { font-size: 12px; font-weight: 600; }
      .ng-timeline-meta { margin-top: 1px; color: var(--dsw-alias-label-secondary); font-size: 10px; }
      .ng-timeline-text { margin-top: 3px; color: var(--dsw-alias-label-secondary); font-size: 11px; line-height: 1.55; }
      .ng-quote { border-left: 3px solid var(--dsw-alias-brand-primary); padding: 7px 11px;
        background: var(--dsw-alias-bg-layer-2); border-radius: 0 8px 8px 0;
        font-size: 12px; line-height: 1.65; white-space: pre-wrap; word-break: break-word; }
      .ng-bullet-list { margin: 0; padding-left: 18px; font-size: 12px; line-height: 1.65; }
      .ng-skeleton { display: grid; gap: 8px; }
      .ng-skeleton-bar { height: 13px; border-radius: 6px; background: var(--dsw-alias-bg-layer-2); }
      .ng-preview-error { display: flex; flex-direction: column; gap: 8px; align-items: flex-start; }
      .ng-empty-preview { text-align: center; padding: 22px 12px; color: var(--dsw-alias-label-secondary); }
      .ng-raw { margin-top: 14px; padding-top: 8px; border-top: 1px solid var(--dsw-alias-border-l1); }
      .ng-raw summary { cursor: pointer; color: var(--dsw-alias-label-secondary); font-size: 11px; }
      .ng-raw-pre { margin: 8px 0 0; white-space: pre-wrap; word-break: break-word;
        font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 11px;
        line-height: 1.5; max-height: 320px; overflow: auto; background: var(--dsw-alias-bg-layer-1);
        border: 1px solid var(--dsw-alias-border-l1); border-radius: 7px; padding: 8px; }
      .ng-prose-preview { margin: 0; white-space: pre-wrap; word-break: break-word;
        font-size: 12.5px; line-height: 1.8; max-height: 520px; overflow: auto;
        background: var(--dsw-alias-bg-base); border: 1px solid var(--dsw-alias-border-l1);
        border-radius: 9px; padding: 12px; }
      .ng-graph { width: 100%; height: auto; display: block; border: 1px solid var(--dsw-alias-border-l1);
        border-radius: 9px; background: var(--dsw-alias-bg-base); }
      .ng-graph text { font-size: 9px; fill: var(--dsw-alias-label-primary); pointer-events: none; }
      .ng-legend { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 6px; color: var(--dsw-alias-label-secondary); font-size: 11px; }
      .ng-legend-item { display: inline-flex; align-items: center; gap: 4px; }
      .ng-legend-dot { width: 8px; height: 8px; border-radius: 50%; }
      .ng-tray { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; padding: 8px 10px;
        border-top: 1px solid var(--dsw-alias-border-l1); background: var(--dsw-alias-bg-base); }
      .ng-chip { display: inline-flex; align-items: center; gap: 4px; border: 1px solid var(--dsw-alias-border-l1);
        background: var(--dsw-alias-bg-layer-2); border-radius: 999px; padding: 2px 8px; font-size: 11px; }
      .ng-chip button { border: 0; background: transparent; color: var(--dsw-alias-label-secondary);
        cursor: pointer; padding: 0 2px; font-size: 12px; }
      .ng-error { color: var(--dsw-alias-state-error-primary); font-size: 11px; padding: 4px 0; }
      .ng-empty { padding: 18px; text-align: center; color: var(--dsw-alias-label-secondary); }
      .ng-empty-title { font-weight: 600; color: var(--dsw-alias-label-primary); margin-bottom: 4px; }
      .ng-fallback { width: 100%; min-height: 74px; resize: vertical; margin-top: 4px; }
      .ng-hint { color: var(--dsw-alias-label-secondary); font-size: 11px; }
      .ng-kv { display: flex; flex-wrap: wrap; gap: 6px 12px; margin-top: 4px; color: var(--dsw-alias-label-secondary); font-size: 11px; }
      .ng-kv b { color: var(--dsw-alias-label-primary); font-weight: 600; }
      .ng-clickable { cursor: pointer; }
      .ng-bar-title { min-width: 0; display: flex; flex-direction: column; gap: 1px; }
      .ng-view { height: 100%; min-height: 560px; display: flex; flex-direction: column; gap: 8px; }
      .ng-view .ng-panel { flex: 1; display: flex; flex-direction: column; min-height: 0; margin-top: 0; }
      .ng-view .ng-body { flex: 1; max-height: none; }
      .ng-view-bar { flex: none; }
      .ng-flow { display: grid; grid-template-columns: minmax(0, 1.08fr) minmax(0, .92fr); gap: 12px; align-items: start; }
      .ng-flow-main { min-width: 0; }
      .ng-flow-side { min-width: 0; padding: 10px; border: 1px solid var(--dsw-alias-border-l1);
        border-radius: 12px; background: var(--dsw-alias-bg-layer-1); }
      .ng-flow-side-head { display: flex; align-items: flex-start; gap: 8px; margin-bottom: 8px; }
      .ng-flow-side-head > div { flex: 1 1 auto; min-width: 0; }
      .ng-flow-side .ng-detail { border: 0; border-radius: 0; background: transparent; }
      .ng-flow-side .ng-detail-head { padding: 0 0 10px; background: transparent; }
      .ng-flow-side .ng-detail-body { padding: 0; }
      @media (max-width: 900px) {
        .ng-flow { grid-template-columns: 1fr; }
        .ng-flow-side { border-top: 1px solid var(--dsw-alias-border-l1); }
      }
      .ng-stage.selected { border-color: var(--dsw-alias-brand-primary); box-shadow: inset 0 0 0 1px var(--dsw-alias-brand-primary); }
      .ng-recent { margin-top: 12px; }
      .ng-updated { white-space: nowrap; }
      .ng-notice { color: var(--dsw-alias-state-success-primary); font-size: 11px; white-space: nowrap; }
      .ng-select-sm { padding: 3px 6px; font-size: 11px; line-height: 16px; }
    `;

    function EmptyProject(props) {
      const { t, projects, onPick, error } = props;
      return h(
        'div',
        { className: 'ng-empty' },
        h('div', { className: 'ng-empty-title' }, t('noProject')),
        h('div', null, t('noProjectHint')),
        error ? h('div', { className: 'ng-error' }, error) : null,
        projects && projects.length
          ? h(
              'div',
              { style: { marginTop: '10px', display: 'flex', gap: '6px', justifyContent: 'center', flexWrap: 'wrap' } },
              h(
                'select',
                {
                  className: 'ng-select',
                  defaultValue: '',
                  onChange: (event) => {
                    if (event.target.value) onPick(event.target.value);
                  }
                },
                h('option', { value: '' }, t('chooseProject')),
                projects.map((project) =>
                  h('option', { key: project.root, value: project.root }, `${project.name} — ${project.root}`)
                )
              )
            )
          : h('div', { className: 'ng-hint', style: { marginTop: '6px' } }, t('noCandidates'))
      );
    }

    function NovelgenDock(props) {
      const { sessionId, inputActions, t } = props;
      const isView = props.mode === 'view';
      const [openState, setOpenState] = React.useState(Boolean(props.initialOpen));
      const open = isView || openState;
      const [loading, setLoading] = React.useState(false);
      const [data, setData] = React.useState(props.initialData || null);
      const [projects, setProjects] = React.useState(
        Array.isArray(props.initialProjects) ? props.initialProjects : []
      );
      const [error, setError] = React.useState('');
      const [tab, setTab] = React.useState(props.initialTab || 'flow');
      const [stageKey, setStageKey] = React.useState(props.initialStage || 'setup');
      const [selected, setSelected] = React.useState([]);
      const [focusId, setFocusId] = React.useState(
        props.initialFocusId || (props.initialData && props.initialData.next ? props.initialData.next.targetId : null)
      );
      const [preview, setPreview] = React.useState(props.initialPreview || null);
      const [previewLoading, setPreviewLoading] = React.useState(false);
      const [previewError, setPreviewError] = React.useState('');
      const [previewNonce, setPreviewNonce] = React.useState(0);
      const [query, setQuery] = React.useState('');
      const [kind, setKind] = React.useState('all');
      const [statusFilter, setStatusFilter] = React.useState('all');
      const [volumeFilter, setVolumeFilter] = React.useState('all');
      const [visibleLimit, setVisibleLimit] = React.useState(60);
      const [intent, setIntent] = React.useState('auto');
      const [note, setNote] = React.useState('');
      const [fallback, setFallback] = React.useState('');
      const [busy, setBusy] = React.useState(false);
      const [lastUpdated, setLastUpdated] = React.useState(null);
      const [notice, setNotice] = React.useState('');
      const [batchSteps, setBatchSteps] = React.useState(5);

      const focusRef = React.useRef(focusId);
      const projectRef = React.useRef('');
      const previewFocusRef = React.useRef('');
      const loadSeqRef = React.useRef(0);
      const refreshTimerRef = React.useRef(null);
      React.useEffect(() => {
        focusRef.current = focusId;
      }, [focusId]);

      React.useEffect(
        () => () => {
          if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
        },
        []
      );

      React.useEffect(() => {
        if (!notice) return undefined;
        const timer = setTimeout(() => setNotice(''), 6000);
        return () => clearTimeout(timer);
      }, [notice]);

      const load = React.useCallback(
        async (projectDir) => {
          if (!sessionId) return;
          const seq = ++loadSeqRef.current;
          setLoading(true);
          try {
            const params = new URLSearchParams({ sessionId });
            if (projectDir) params.set('project', projectDir);
            else if (projectRef.current) params.set('project', projectRef.current);
            const json = await api(`/state?${params.toString()}`);
            if (seq !== loadSeqRef.current) return;
            if (!json.ok) {
              setData(null);
              setProjects(Array.isArray(json.projects) ? json.projects : []);
              setError(json.error === 'no-project' ? '' : json.error || json.message || t('error'));
              return;
            }
            setData(json.state);
            projectRef.current = json.state.project.root;
            if (Array.isArray(json.projects)) setProjects(json.projects);
            setError('');
            setLastUpdated(new Date());
            setPreviewNonce((value) => value + 1);
            if (!focusRef.current) {
              const next = json.state.next;
              const firstChapter = (json.state.resources || []).find((resource) => resource.kind === 'chapter');
              const defaultStage = !(json.state.counts && json.state.counts.setup)
                ? 'setup'
                : next && next.stage
                  ? next.stage
                  : 'setup';
              setStageKey(defaultStage);
              setFocusId(
                defaultStage === 'setup'
                  ? 'setup'
                  : next && next.targetId
                    ? next.targetId
                    : firstChapter
                      ? firstChapter.id
                      : null
              );
            }
          } catch (failure) {
            if (seq !== loadSeqRef.current) return;
            setError(failure instanceof Error ? failure.message : String(failure));
          } finally {
            if (seq === loadSeqRef.current) setLoading(false);
          }
        },
        [sessionId, t]
      );

      React.useEffect(() => {
        load();
      }, [load]);

      React.useEffect(() => {
        if (!open) return undefined;
        const timer = setInterval(() => {
          if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
          load(projectRef.current || undefined);
        }, 15000);
        const onFocus = () => load(projectRef.current || undefined);
        if (typeof window !== 'undefined') window.addEventListener('focus', onFocus);
        return () => {
          clearInterval(timer);
          if (typeof window !== 'undefined') window.removeEventListener('focus', onFocus);
        };
      }, [open, load]);

      React.useEffect(() => {
        setVisibleLimit(60);
      }, [query, kind, statusFilter, volumeFilter]);

      React.useEffect(() => {
        if (open) load();
      }, [open, load]);

      React.useEffect(() => {
        let cancelled = false;
        (async () => {
          if (!sessionId) return;
          try {
            const json = await api(`/projects?sessionId=${encodeURIComponent(sessionId)}`);
            if (!cancelled && json.ok) setProjects(Array.isArray(json.projects) ? json.projects : []);
          } catch {
            // the project picker is optional
          }
        })();
        return () => {
          cancelled = true;
        };
      }, [sessionId]);

      React.useEffect(() => {
        if (!open || !focusId) {
          setPreview(null);
          setPreviewError('');
          setPreviewLoading(false);
          previewFocusRef.current = '';
          return undefined;
        }
        let cancelled = false;
        const sameFocus = previewFocusRef.current === focusId;
        if (!sameFocus) setPreviewLoading(true);
        setPreviewError('');
        (async () => {
          try {
            const params = new URLSearchParams({ sessionId, id: focusId });
            if (data && data.project && data.project.root) params.set('project', data.project.root);
            const json = await api(`/preview?${params.toString()}`);
            if (cancelled) return;
            if (!json.ok) {
              setPreview(null);
              setPreviewError(json.error || t('previewFailed'));
              return;
            }
            setPreview(json.preview || null);
            previewFocusRef.current = focusId;
          } catch (failure) {
            if (!cancelled) {
              setPreview(null);
              setPreviewError(failure instanceof Error ? failure.message : String(failure));
            }
          } finally {
            if (!cancelled) setPreviewLoading(false);
          }
        })();
        return () => {
          cancelled = true;
        };
      }, [open, focusId, sessionId, data && data.project ? data.project.root : '', previewNonce, t]);

      const resources = (data && data.resources) || [];
      const byId = React.useMemo(() => {
        const map = new Map();
        for (const resource of resources) map.set(resource.id, resource);
        return map;
      }, [resources]);

      const focus = focusId ? byId.get(focusId) : null;
      const project = data && data.project ? data.project : null;
      const next = data && data.next ? data.next : null;
      const counts = (data && data.counts) || {};

      const stageResourceId = (stage) => {
        if (!data || !stage) return null;
        const items = data.resources || [];
        if (stage.key === 'setup') return 'setup';
        if (stage.key === 'outline') {
          const volume =
            items.find((item) => item.kind === 'volume' && item.status !== 'done') ||
            items.find((item) => item.kind === 'volume');
          return volume ? volume.id : items.find((item) => item.kind === 'part')?.id || null;
        }
        if (stage.key === 'craft') {
          const crafted = items.find((item) => item.kind === 'character' && item.status === 'done');
          const missing = items.find((item) => item.kind === 'character' && item.status === 'missing');
          return crafted ? crafted.id : missing ? missing.id : null;
        }
        if (stage.key === 'write') {
          if (next && next.targetId && next.targetId.startsWith('chapter:')) return next.targetId;
          const unwritten =
            items.find((item) => item.kind === 'chapter' && item.meta && item.meta.written === false) ||
            items.find((item) => item.kind === 'chapter');
          return unwritten ? unwritten.id : null;
        }
        if (stage.key === 'recap') {
          const recap = items.find((item) => item.kind === 'recap');
          if (recap) return recap.id;
          const written = items.find((item) => item.kind === 'chapter' && item.meta && item.meta.written);
          return written ? written.id : null;
        }
        return next && next.targetId ? next.targetId : null;
      };

      const focusStage = (stage) => {
        setStageKey(stage.key);
        const id = stageResourceId(stage);
        if (id) {
          focusRef.current = id;
          setFocusId(id);
        }
        setTab('flow');
      };

      const activeStage =
        (data && data.stages.find((stage) => stage.key === stageKey)) ||
        (data && data.stages[0]) ||
        null;

      const resourceVolumeId = (resource) => {
        if (!resource) return '';
        if (resource.kind === 'chapter') return resource.meta?.volumeId || '';
        if (resource.kind === 'volume') return resource.id.slice('volume:'.length);
        if (resource.kind === 'recap') {
          const chapterId = resource.meta?.chapterId || resource.id.slice('recap:'.length);
          const chapter = byId.get(`chapter:${chapterId}`);
          if (chapter?.meta?.volumeId) return chapter.meta.volumeId;
          const match = /^(.*-V\d+)/.exec(chapterId);
          return match ? match[1] : '';
        }
        return '';
      };

      const volumeOptions = React.useMemo(
        () =>
          resources
            .filter((resource) => resource.kind === 'volume')
            .map((resource) => ({ id: resource.id.slice('volume:'.length), title: resource.title }))
            .sort((a, b) => a.id.localeCompare(b.id)),
        [resources]
      );

      const recentResources = React.useMemo(() => {
        const seen = new Set();
        return resources
          .filter(
            (resource) =>
              resource.path &&
              resource.meta &&
              resource.meta.updatedAt &&
              (resource.kind !== 'chapter' || resource.meta.written)
          )
          .map((resource) => ({ resource, time: Date.parse(resource.meta.updatedAt) }))
          .filter((entry) => Number.isFinite(entry.time))
          .sort((a, b) => b.time - a.time)
          .filter((entry) => {
            const resource = entry.resource;
            const key =
              resource.kind === 'chapter' || resource.kind === 'recap'
                ? resource.id
                : `${resource.kind}:${resource.path || resource.id}`;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
          })
          .slice(0, 6)
          .map((entry) => entry.resource);
      }, [resources]);

      const filtered = React.useMemo(() => {
        const needle = query.trim().toLowerCase();
        return resources
          .filter((resource) => (kind === 'all' ? true : resource.kind === kind))
          .filter((resource) => {
            if (statusFilter === 'unfinished') return resource.status !== 'done';
            if (statusFilter === 'written') {
              return resource.kind !== 'chapter' || Boolean(resource.meta && resource.meta.written);
            }
            if (statusFilter === 'unwritten') {
              return resource.kind === 'chapter' && !(resource.meta && resource.meta.written);
            }
            return true;
          })
          .filter((resource) => {
            if (volumeFilter === 'all') return true;
            return resourceVolumeId(resource) === volumeFilter;
          })
          .filter((resource) => {
            if (!needle) return true;
            return `${resource.id}\n${resource.title}\n${resource.summary}\n${resource.path}`
              .toLowerCase()
              .includes(needle);
          });
      }, [resources, kind, statusFilter, volumeFilter, query]);

      const visibleResources = filtered.slice(0, visibleLimit);

      const toggle = (id) => {
        setSelected((previous) => {
          if (previous.includes(id)) return previous.filter((item) => item !== id);
          if (previous.length >= 20) {
            setError(t('selectLimit'));
            return previous;
          }
          return [...previous, id];
        });
      };

      const insertPrompt = async (text) => {
        if (!text) return;
        setFallback(text);
        try {
          if (!inputActions || typeof inputActions.captureInsertion !== 'function' || typeof inputActions.insertText !== 'function') {
            setError(t('noInput'));
            return;
          }
          const span = inputActions.captureInsertion();
          const ok = inputActions.insertText(text, span);
          if (!ok) {
            setError(t('insertConflict'));
            return;
          }
          setError('');
          setFallback('');
          if (!isView) setOpenState(false);
        } catch (failure) {
          setError(failure instanceof Error ? failure.message : String(failure));
        }
      };

      const submitPrompt = (text) => {
        if (!text) return;
        setFallback(text);
        try {
          if (
            !inputActions ||
            typeof inputActions.captureInsertion !== 'function' ||
            typeof inputActions.insertText !== 'function' ||
            typeof inputActions.submit !== 'function'
          ) {
            setError(t('runUnavailable'));
            return;
          }
          const span = inputActions.captureInsertion();
          const ok = inputActions.insertText(`\n\n${text}`, span);
          if (!ok) {
            setError(t('insertConflict'));
            return;
          }
          // The composer's submit() currently returns void and reports a
          // refusal through its own state; only insertText has a reliable
          // boolean result.
          inputActions.submit();
          setFallback('');
          setError('');
          setNotice(t('runSubmitted'));
          if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
          refreshTimerRef.current = setTimeout(() => load(projectRef.current || undefined), 1800);
        } catch (failure) {
          setError(failure instanceof Error ? failure.message : String(failure));
        }
      };

      const doSelection = async (run = false) => {
        const ids = selected.length ? selected : focusId ? [focusId] : [];
        if (!ids.length) {
          setError(t('selectFirst'));
          return;
        }
        setBusy(true);
        try {
          const json = await api('/selection', {
            method: 'POST',
            body: JSON.stringify({
              sessionId,
              ids,
              intent,
              note,
              project: project ? project.root : ''
            })
          });
          if (!json.ok) {
            setError(json.error || t('error'));
            return;
          }
          if (run) submitPrompt(json.prompt);
          else await insertPrompt(json.prompt);
        } catch (failure) {
          setError(failure instanceof Error ? failure.message : String(failure));
        } finally {
          setBusy(false);
        }
      };

      const nextPrompt = () => {
        if (!project) return '';
        const args = {
          ...(next && next.prepareArguments ? next.prepareArguments : { stage: 'next' }),
          projectDir: project.root
        };
        const reviewOnly = next && next.stage === 'review';
        return [
          `继续 Novelgen 项目《${project.name}》的长程创作（projectDir: \`${project.root}\`）。`,
          next
            ? `当前下一步：${stageLabel(t, next.stage)}${next.targetId ? ` · ${next.targetId}` : ''}（${next.reason || ''}）。`
            : '当前没有明确的下一步，请先调用 novelgen_status 判断。',
          `请先调用 novelgen_status，再调用 novelgen_prepare(${JSON.stringify(args)})；把返回的 delegate.prompt 原样交给 subagent。`,
          reviewOnly
            ? '这是只读审校：收集审校 JSON，不要调用 novelgen_apply；需要修改时先向我确认。'
            : '收到 JSON 后用 novelgen_apply 落盘。',
          '如果 novelgen_prepare 返回 direct 模式，把 command 作为后台任务运行，完成后重新 status。',
          '完成后继续 status → prepare → subagent → apply，直到需要我决策或本次目标完成。'
        ].join('\n');
      };

      const doNext = () => insertPrompt(nextPrompt());
      const runNext = () => submitPrompt(nextPrompt());

      const resourcePrompt = () => {
        if (!focus || !project) return '';
        if (focus.kind === 'storyline') {
          return [
            `请分析 Novelgen 项目《${project.name}》中的故事线：${focus.title}（\`${focus.id}\`）。`,
            '结合大纲中的章节推进和相关角色，指出它目前的主线位置、断点和下一步可以强化的地方。',
            '如果只是讨论，不要修改项目；需要改结构时再调用 novelgen_prepare。'
          ].join('\n');
        }
        const stage =
          focus.kind === 'chapter'
            ? 'write'
            : focus.kind === 'volume' || focus.kind === 'part'
              ? 'outline'
              : focus.kind === 'setup'
                ? 'setup'
                : focus.kind === 'recap'
                  ? 'recap'
                  : ['character', 'location', 'item', 'organization'].includes(focus.kind)
                    ? 'craft'
                    : 'next';
        const args = { projectDir: project.root, stage, targetId: focus.id, intent: intent || 'auto' };
        return [
          `请处理 Novelgen 项目《${project.name}》中的资源：${focus.title}（\`${focus.id}\`）。`,
          `先调用 novelgen_prepare(${JSON.stringify(args)}) 生成任务包；把返回的 delegate.prompt 原样交给 subagent。`,
          intent === 'ask'
            ? '这是讨论请求：只分析和建议，不要修改项目，也不要调用 novelgen_apply。'
            : intent === 'review'
              ? '这是审校请求：只收集审校 JSON，不要调用 novelgen_apply；需要修改时先向我确认。'
              : '收到 JSON 后用 novelgen_apply 落盘。',
          '如果要先讨论而不是修改，请只给出分析和建议，不要调用 novelgen_apply。'
        ].join('\n');
      };

      const prepareResource = () => insertPrompt(resourcePrompt());
      const runResource = () => submitPrompt(resourcePrompt());

      const stagePrompt = (stage) => {
        if (!project) return '';
        const args = { projectDir: project.root, stage: stage.key, intent: 'auto' };
        if (next && next.stage === stage.key && next.prepareArguments) {
          Object.assign(args, next.prepareArguments);
        }
        return [
          `请为 Novelgen 项目《${project.name}》的「${stageLabel(t, stage.key)}」阶段准备下一步创作。`,
          `先调用 novelgen_status，再调用 novelgen_prepare(${JSON.stringify(args)})；把 delegate.prompt 原样交给 subagent；收到 JSON 后用 novelgen_apply 落盘。`,
          '如果返回 direct 模式，把 command 作为后台任务运行，完成后重新 status。'
        ].join('\n');
      };

      const stageAction = (stage) => insertPrompt(stagePrompt(stage));
      const runStage = (stage) => submitPrompt(stagePrompt(stage));

      const batchPrompt = (steps) => {
        if (!project) return '';
        const count = Math.max(1, Math.min(20, Number(steps) || 1));
        return [
          `请为 Novelgen 项目《${project.name}》连续推进 ${count} 个创作步骤（以成功调用 novelgen_apply 落盘计）。`,
          `先调用 create_goal，objective 写清楚“连续推进 ${count} 个 Novelgen 创作步骤”，max_goal_rounds 至少 ${count}；如果已有活动 goal，就 update_goal 合并这个目标。`,
          '然后循环执行：novelgen_status → novelgen_prepare → subagent → novelgen_apply。每完成一步更新 goal 进度，不要为每个小步骤停下来等我确认。',
          '遇到 blocking、需要用户决策、或 subagent 连续失败时暂停，并明确汇报已完成步数、失败原因和剩余工作。',
          '最后汇报：完成数、每一步落盘的文件/资源、检查结果和下一步建议。'
        ].join('\n');
      };

      const runBatch = () => submitPrompt(batchPrompt(batchSteps));

      const switchProject = async (root) => {
        if (!root) return;
        setBusy(true);
        try {
          const json = await api('/project', {
            method: 'POST',
            body: JSON.stringify({ sessionId, projectDir: root })
          });
          if (!json.ok) {
            setError(json.error || t('error'));
            return;
          }
          setSelected([]);
          setFocusId(null);
          setPreview(null);
          setKind('all');
          setStatusFilter('all');
          setVolumeFilter('all');
          setQuery('');
          projectRef.current = json.project.root;
          await load(json.project.root);
        } catch (failure) {
          setError(failure instanceof Error ? failure.message : String(failure));
        } finally {
          setBusy(false);
        }
      };

      const copyFallback = async () => {
        try {
          await navigator.clipboard.writeText(fallback);
          setError('');
        } catch {
          setError(t('copy') + ': ' + t('error'));
        }
      };

      const renderBar = () => {
        const written = counts.written || 0;
        const chapters = counts.chapters || 0;
        return h(
          'div',
          { className: `ng-bar ${isView ? 'ng-view-bar' : ''}` },
          h('div', { className: 'ng-mark', 'aria-hidden': true }, 'N'),
          h(
            'div',
            { className: 'ng-bar-title' },
            h('div', { className: 'ng-name', title: project ? project.root : '' }, project ? project.name : t('title')),
            isView ? h('div', { className: 'ng-hint' }, t('viewSubtitle')) : null
          ),
          project
            ? h(
                'span',
                { className: 'ng-muted' },
                t('writtenProgress', { a: formatNumber(written), b: formatNumber(chapters) })
              )
            : null,
          next && project
            ? h(
                'span',
                { className: 'ng-badge active', title: next.reason || '' },
                `${stageLabel(t, next.stage)} · ${next.task || ''}`
              )
            : null,
          h('div', { className: 'ng-spacer' }),
          lastUpdated
            ? h(
                'span',
                { className: 'ng-muted ng-updated' },
                loading ? t('refreshing') : t('updatedAt', { time: formatTime(lastUpdated) })
              )
            : null,
          notice ? h('span', { className: 'ng-notice' }, notice) : null,
          isView
            ? null
            : h(
                'button',
                {
                  className: 'ng-btn',
                  type: 'button',
                  onClick: () => setOpenState((value) => !value),
                  'aria-expanded': open
                },
                open ? t('close') : t('open')
              ),
          h(
            'button',
            {
              className: 'ng-btn',
              type: 'button',
              title: t('refresh'),
              'aria-label': t('refresh'),
              onClick: () => load(project ? project.root : undefined)
            },
            '↻'
          )
        );
      };

      const renderProjectPicker = () => {
        if (!projects.length) return null;
        const currentRoot = project ? project.root : '';
        const options = projects.slice();
        if (currentRoot && !options.some((item) => item.root === currentRoot)) {
          options.unshift({ root: currentRoot, name: project.name });
        }
        return h(
          'select',
          {
            className: 'ng-select',
            value: currentRoot,
            title: t('projectSwitch'),
            onChange: (event) => switchProject(event.target.value)
          },
          h('option', { value: '' }, t('chooseProject')),
          options.map((item) =>
            h('option', { key: item.root, value: item.root }, `${item.name} — ${item.root}`)
          )
        );
      };

      const renderFlow = () => {
        if (!data) return null;
        return h(
          'div',
          { className: 'ng-flow' },
          h(
            'div',
            { className: 'ng-flow-main' },
            h(
              'div',
              { className: 'ng-stages' },
              data.stages.map((stage) =>
                h(
                  'div',
                  {
                    key: stage.key,
                    className: `ng-stage ${stage.status} ${stageKey === stage.key ? 'selected' : ''}`,
                    onClick: () => focusStage(stage),
                    title: t('stagePreviewHint')
                  },
                  h(
                    'div',
                    { className: 'ng-stage-top' },
                    h('span', { className: `ng-dot ${stage.status}` }),
                    h('span', { className: 'ng-stage-name' }, stageLabel(t, stage.key)),
                    h(
                      'span',
                      { className: 'ng-stage-count' },
                      `${formatNumber(stage.current)}/${formatNumber(stage.total)}`
                    )
                  ),
                  h('div', { className: 'ng-stage-detail' }, stage.detail || '')
                )
              )
            ),
            next
              ? h(
                  'div',
                  { className: 'ng-next' },
                  h(
                    'div',
                    { className: 'ng-next-title' },
                    h('span', { className: 'ng-badge active' }, t('next')),
                    h('span', null, `${stageLabel(t, next.stage)}${next.targetId ? ` · ${next.targetId}` : ''}`)
                  ),
                  h('div', { className: 'ng-next-body' }, next.reason || ''),
                  h(
                    'div',
                    { className: 'ng-actions' },
                    h(
                      'button',
                      { className: 'ng-btn ng-btn-primary', type: 'button', onClick: runNext },
                      next.direct ? t('nextDirect') : t('runNext')
                    ),
                    h(
                      'button',
                      { className: 'ng-btn', type: 'button', onClick: doNext },
                      t('insertOnly')
                    ),
                    h(
                      'select',
                      {
                        className: 'ng-select ng-select-sm',
                        value: batchSteps,
                        title: t('batchSteps'),
                        onChange: (event) => setBatchSteps(Number(event.target.value))
                      },
                      [1, 3, 5, 10].map((value) =>
                        h('option', { key: value, value }, t('stepsCount', { n: value }))
                      )
                    ),
                    h(
                      'button',
                      { className: 'ng-btn', type: 'button', onClick: runBatch },
                      t('runBatch')
                    ),
                    h('span', { className: 'ng-hint' }, t('nextHint'))
                  )
                )
              : h(
                  'div',
                  { className: 'ng-next' },
                  h('div', { className: 'ng-next-title' }, t('noNext')),
                  h('div', { className: 'ng-next-body' }, t('manualHint'))
                ),
            h(
              'div',
              { className: 'ng-kv' },
              h('span', null, `${t('volumes')}: `, h('b', null, formatNumber(counts.volumes || 0))),
              h('span', null, `${t('chapters')}: `, h('b', null, formatNumber(counts.chapters || 0))),
              h('span', null, `${t('characters')}: `, h('b', null, formatNumber(counts.characters || 0))),
              h('span', null, `${t('resources')}: `, h('b', null, formatNumber(resources.length))),
              h('span', null, `${t('words')}: `, h('b', null, formatNumber(counts.words || 0)))
            ),
            recentResources.length
              ? h(
                  'div',
                  { className: 'ng-recent' },
                  h('div', { className: 'ng-section-title' }, t('recentChanges')),
                  h(
                    'div',
                    { className: 'ng-list' },
                    recentResources.map((resource) =>
                      h(
                        'button',
                        {
                          key: resource.id,
                          type: 'button',
                          className: 'ng-list-row',
                          onClick: () => setFocusId(resource.id)
                        },
                        h('span', { className: 'ng-list-index' }, kindLabel(t, resource.kind).slice(0, 1)),
                        h(
                          'div',
                          { className: 'ng-list-main' },
                          h('div', { className: 'ng-list-title' }, resource.title),
                          h(
                            'div',
                            { className: 'ng-list-sub' },
                            `${kindLabel(t, resource.kind)} · ${formatTime(resource.meta.updatedAt)}`
                          )
                        ),
                        h('span', { className: 'ng-badge' }, kindLabel(t, resource.kind))
                      )
                    )
                  )
                )
              : null
          ),
          h(
            'div',
            { className: 'ng-flow-side' },
            h(
              'div',
              { className: 'ng-flow-side-head' },
              h(
                'div',
                { style: { minWidth: 0 } },
                h(
                  'div',
                  { className: 'ng-detail-title' },
                  `${stageLabel(t, activeStage ? activeStage.key : 'setup')} · ${t('stagePreview')}`
                ),
                h('div', { className: 'ng-hint' }, activeStage && activeStage.detail ? activeStage.detail : '')
              ),
              activeStage
                ? h(
                    'div',
                    { className: 'ng-actions', style: { marginTop: 0 } },
                    h(
                      'button',
                      {
                        className: 'ng-btn ng-btn-primary',
                        type: 'button',
                        onClick: () => runStage(activeStage)
                      },
                      t('runStage')
                    ),
                    h(
                      'button',
                      {
                        className: 'ng-btn',
                        type: 'button',
                        onClick: () => stageAction(activeStage)
                      },
                      t('insertOnly')
                    )
                  )
                : null
            ),
            renderPreview()
          )
        );
      };

      const renderStat = (label, value) =>
        h(
          'div',
          { className: 'ng-stat', key: label },
          h('div', { className: 'ng-stat-value' }, value === undefined || value === null || value === '' ? '—' : String(value)),
          h('div', { className: 'ng-stat-label' }, label)
        );

      const renderCard = (label, value, key) => {
        if (value === undefined || value === null || value === '') return null;
        let content;
        if (Array.isArray(value)) {
          content = value
            .map((item) => (item && typeof item === 'object' ? JSON.stringify(item) : String(item)))
            .filter(Boolean)
            .join('、');
        } else if (typeof value === 'object') {
          content = JSON.stringify(value, null, 2);
        } else {
          content = String(value);
        }
        if (!content) return null;
        return h(
          'div',
          { className: 'ng-card', key: key || label },
          h('div', { className: 'ng-card-label' }, label),
          h('div', { className: 'ng-card-value' }, content)
        );
      };

      const renderChips = (items, onPick, keyPrefix) => {
        const values = (items || [])
          .map((item) => (typeof item === 'string' ? item : item && item.name ? item.name : ''))
          .filter(Boolean);
        if (!values.length) return null;
        return h(
          'div',
          { className: 'ng-chip-row' },
          values.map((value, index) => {
            const interactive = typeof onPick === 'function' && onPick(value);
            return h(
              interactive ? 'button' : 'span',
              {
                key: `${keyPrefix || 'chip'}-${value}-${index}`,
                className: 'ng-tag',
                type: interactive ? 'button' : undefined,
                onClick: interactive ? () => setFocusId(onPick(value)) : undefined
              },
              value
            );
          })
        );
      };

      const renderSection = (title, body) => {
        if (!body) return null;
        return h(
          'div',
          { className: 'ng-preview-section' },
          h('div', { className: 'ng-section-title' }, title),
          body
        );
      };

      const renderRaw = (current) => {
        if (!current.text) return null;
        return h(
          'details',
          { className: 'ng-raw' },
          h('summary', null, t('rawData')),
          h('pre', { className: 'ng-raw-pre' }, current.text),
          current.truncated ? h('div', { className: 'ng-hint' }, t('previewTruncated')) : null
        );
      };

      const renderEmptyPreview = () =>
        h(
          'div',
          { className: 'ng-empty-preview' },
          h('div', { className: 'ng-empty-title' }, t('previewEmpty')),
          h('div', { className: 'ng-hint' }, t('previewEmptyHint'))
        );

      const renderChapterRows = (chapters, onPick) =>
        h(
          'div',
          { className: 'ng-list' },
          (chapters || []).map((chapter, index) =>
            h(
              'button',
              {
                key: chapter.id,
                type: 'button',
                className: 'ng-list-row',
                onClick: () => onPick(chapter)
              },
              h('span', { className: 'ng-list-index' }, String(index + 1).padStart(2, '0')),
              h(
                'div',
                { className: 'ng-list-main' },
                h('div', { className: 'ng-list-title' }, `${chapter.id} · ${chapter.title || ''}`),
                chapter.summary ? h('div', { className: 'ng-list-sub' }, short(chapter.summary, 150)) : null
              ),
              h(
                'span',
                { className: `ng-badge ${chapter.written ? 'done' : 'missing'}` },
                chapter.written ? t('status_written') : t('status_unwritten')
              )
            )
          )
        );

      const renderVolumeRows = (volumes) =>
        h(
          'div',
          { className: 'ng-list' },
          (volumes || []).map((volumeItem, index) =>
            h(
              'button',
              {
                key: volumeItem.id,
                type: 'button',
                className: 'ng-list-row',
                onClick: () => setFocusId(`volume:${volumeItem.id}`)
              },
              h('span', { className: 'ng-list-index' }, String(index + 1).padStart(2, '0')),
              h(
                'div',
                { className: 'ng-list-main' },
                h('div', { className: 'ng-list-title' }, `${volumeItem.id} · ${volumeItem.title || ''}`),
                volumeItem.summary ? h('div', { className: 'ng-list-sub' }, short(volumeItem.summary, 150)) : null
              ),
              h('span', { className: 'ng-tag' }, `${volumeItem.chapterCount} ${t('chapters')}`)
            )
          )
        );

      const renderTimeline = (events) =>
        h(
          'div',
          { className: 'ng-timeline' },
          (events || []).map((event, index) =>
            h(
              'div',
              { key: `${event.subject || event.type || 'event'}-${index}`, className: 'ng-timeline-item' },
              h('div', { className: 'ng-timeline-dot' }),
              h(
                'div',
                { className: 'ng-timeline-body' },
                h('div', { className: 'ng-timeline-title' }, event.subject || event.type || `${t('events')} ${index + 1}`),
                [event.actor, event.action, event.target].filter(Boolean).length
                  ? h(
                      'div',
                      { className: 'ng-timeline-meta' },
                      [event.actor, event.action, event.target].filter(Boolean).join(' → ')
                    )
                  : null,
                event.details ? h('div', { className: 'ng-timeline-text' }, short(event.details, 420)) : null
              )
            )
          )
        );

      const renderPayoff = (payoff) => {
        if (!payoff) return null;
        if (typeof payoff !== 'object' || Array.isArray(payoff)) return renderCard(t('chapterPayoff'), payoff);
        const labels = {
          desire: t('desire'),
          pressure: t('pressure'),
          clever_move: t('cleverMove'),
          payoff_moment: t('payoffMoment'),
          reward: t('reward'),
          hook: t('hook'),
          social_proof: t('socialProof')
        };
        const cards = Object.entries(payoff)
          .filter(([, value]) => value !== undefined && value !== null && value !== '')
          .map(([key, value]) => renderCard(labels[key] || key, value, `payoff-${key}`));
        return cards.filter(Boolean).length ? h('div', { className: 'ng-card-grid' }, cards) : null;
      };

      const renderPreviewBody = (current) => {
        const dataValue = current.data;
        const raw = renderRaw(current);

        if (!dataValue) {
          return h('div', null, renderEmptyPreview(), raw);
        }

        if (current.kind === 'setup') {
          const cast = Array.isArray(dataValue.core_cast) ? dataValue.core_cast : [];
          const storylines = Array.isArray(dataValue.storylines) ? dataValue.storylines : [];
          const rules = Array.isArray(dataValue.rules) ? dataValue.rules : [];
          return h(
            'div',
            null,
            h(
              'div',
              { className: 'ng-preview-hero' },
              h('div', { className: 'ng-lead' }, dataValue.premise || dataValue.theme || ''),
              renderChips(dataValue.genres, null, 'genre')
            ),
            h(
              'div',
              { className: 'ng-stat-grid' },
              renderStat(t('coreCast'), cast.length),
              renderStat(t('storylines'), storylines.length),
              renderStat(t('rules'), rules.length),
              renderStat(t('volumes'), counts.volumes || 0)
            ),
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('theme'), dataValue.theme),
              renderCard(t('tone'), dataValue.tone),
              renderCard(t('pov'), dataValue.pov_style),
              renderCard(t('audience'), dataValue.target_audience)
            ),
            renderSection(
              t('coreCast'),
              cast.length
                ? h(
                    'div',
                    { className: 'ng-entity-grid' },
                    cast.map((member) => {
                      const target = `character:${member.name}`;
                      const clickable = byId.has(target);
                      return h(
                        clickable ? 'button' : 'div',
                        {
                          key: member.name,
                          className: 'ng-entity',
                          type: clickable ? 'button' : undefined,
                          onClick: clickable ? () => setFocusId(target) : undefined
                        },
                        h('div', { className: 'ng-entity-name' }, member.name),
                        h('div', { className: 'ng-entity-role' }, member.role || member.story_function || '')
                      );
                    })
                  )
                : null
            ),
            renderSection(
              t('storylines'),
              storylines.length
                ? h(
                    'div',
                    { className: 'ng-list' },
                    storylines.map((storyline, index) =>
                      h(
                        'div',
                        { key: storyline.name, className: 'ng-list-row ng-list-row-static' },
                        h('span', { className: 'ng-list-index' }, String(index + 1).padStart(2, '0')),
                        h(
                          'div',
                          { className: 'ng-list-main' },
                          h('div', { className: 'ng-list-title' }, storyline.name),
                          storyline.description ? h('div', { className: 'ng-list-sub' }, short(storyline.description, 160)) : null
                        ),
                        storyline.type ? h('span', { className: 'ng-tag' }, storyline.type) : null
                      )
                    )
                  )
                : null
            ),
            renderSection(
              t('rules'),
              rules.length ? h('ul', { className: 'ng-bullet-list' }, rules.map((rule, index) => h('li', { key: index }, rule))) : null
            ),
            raw
          );
        }

        if (current.kind === 'part') {
          const volumes = Array.isArray(dataValue.volumes) ? dataValue.volumes : [];
          const chapterCount = volumes.reduce((sum, volumeItem) => sum + (volumeItem.chapterCount || 0), 0);
          return h(
            'div',
            null,
            h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-lead' }, dataValue.summary || '')),
            h(
              'div',
              { className: 'ng-stat-grid' },
              renderStat(t('volumes'), volumes.length),
              renderStat(t('chapters'), chapterCount)
            ),
            renderSection(t('volumes'), renderVolumeRows(volumes)),
            raw
          );
        }

        if (current.kind === 'volume') {
          const chapters = Array.isArray(dataValue.chapters) ? dataValue.chapters : [];
          const written = chapters.filter((chapter) => chapter.written).length;
          const eventCount = chapters.reduce((sum, chapter) => sum + (chapter.eventCount || 0), 0);
          return h(
            'div',
            null,
            h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-lead' }, dataValue.summary || '')),
            h(
              'div',
              { className: 'ng-stat-grid' },
              renderStat(t('chapters'), chapters.length),
              renderStat(t('status_written'), written),
              renderStat(t('events'), eventCount)
            ),
            renderSection(t('chapters'), renderChapterRows(chapters, (chapter) => setFocusId(`chapter:${chapter.id}`))),
            renderSection(t('payoff'), renderPayoff(dataValue.payoff_contract)),
            raw
          );
        }

        if (current.kind === 'chapter' && current.format === 'markdown') {
          return h(
            'div',
            null,
            dataValue.summary ? h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-lead' }, dataValue.summary)) : null,
            h('div', { className: 'ng-stat-grid' }, renderStat(t('words'), formatNumber(dataValue.words || 0))),
            renderSection(
              t('chapterText'),
              h('pre', { className: 'ng-prose-preview' }, current.text)
            ),
            current.truncated ? h('div', { className: 'ng-hint' }, t('previewTruncated')) : null
          );
        }

        if (current.kind === 'chapter') {
          const events = Array.isArray(dataValue.events) ? dataValue.events : [];
          const characters = Array.isArray(dataValue.characters) ? dataValue.characters : [];
          const eventCount = dataValue.eventCount || events.length;
          return h(
            'div',
            null,
            dataValue.summary ? h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-lead' }, dataValue.summary)) : null,
            h(
              'div',
              { className: 'ng-stat-grid' },
              renderStat(t('events'), eventCount),
              renderStat(t('characters'), characters.length),
              renderStat(t('words'), formatNumber(current.meta && current.meta.words ? current.meta.words : 0))
            ),
            renderChips(
              characters,
              (name) => (byId.has(`character:${name}`) ? `character:${name}` : null),
              'chapter-character'
            ),
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('location'), dataValue.location),
              renderCard(t('conflict'), dataValue.conflict),
              renderCard(t('pacing'), dataValue.pacing)
            ),
            renderSection(t('events'), renderTimeline(events)),
            eventCount > events.length
              ? h('div', { className: 'ng-hint' }, t('previewPartialEvents', { n: events.length }))
              : null,
            renderSection(t('chapterPayoff'), renderPayoff(dataValue.chapter_payoff)),
            raw
          );
        }

        if (current.kind === 'character') {
          return h(
            'div',
            null,
            h(
              'div',
              { className: 'ng-preview-hero' },
              h('div', { className: 'ng-lead' }, dataValue.background || dataValue.appearance || ''),
              renderChips(dataValue.affiliations, null, 'affiliation')
            ),
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('role'), dataValue.role_in_story || dataValue.rpg_role),
              renderCard(t('age'), dataValue.age),
              renderCard(t('gender'), dataValue.gender),
              renderCard(t('powerLevel'), dataValue.power_level)
            ),
            renderSection(
              t('motivation'),
              dataValue.motivation ? h('div', { className: 'ng-quote' }, dataValue.motivation) : null
            ),
            renderSection(t('personality'), renderChips(dataValue.personality, null, 'personality')),
            renderSection(t('skills'), renderChips(dataValue.skills, null, 'skill')),
            renderSection(t('abilities'), renderChips(dataValue.abilities, null, 'ability')),
            renderSection(t('voice'), dataValue.voice ? h('div', { className: 'ng-quote' }, dataValue.voice) : null),
            raw
          );
        }

        if (current.kind === 'location' || current.kind === 'item' || current.kind === 'organization') {
          const kind = current.kind;
          return h(
            'div',
            null,
            h(
              'div',
              { className: 'ng-preview-hero' },
              h('div', { className: 'ng-lead' }, dataValue.description || dataValue.background || '')
            ),
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('function'), dataValue.function || dataValue.purpose),
              renderCard(t('category'), dataValue.category || dataValue.type),
              renderCard(t('rarity'), dataValue.rarity),
              renderCard(t('owner'), dataValue.owner),
              kind === 'organization' ? renderCard(t('leader'), dataValue.leadership) : null,
              kind === 'organization' ? renderCard(t('goals'), dataValue.goals) : null
            ),
            renderSection(t('atmosphere'), dataValue.atmosphere ? h('div', { className: 'ng-quote' }, dataValue.atmosphere) : null),
            renderSection(t('history'), dataValue.history ? h('div', { className: 'ng-quote' }, dataValue.history) : null),
            renderSection(t('powers'), renderChips(dataValue.powers, null, 'power')),
            renderSection(t('limitations'), renderChips(dataValue.limitations, null, 'limitation')),
            renderSection(t('significance'), dataValue.significance ? h('div', { className: 'ng-quote' }, dataValue.significance) : null),
            renderSection(t('notes'), dataValue.notes ? h('div', { className: 'ng-quote' }, dataValue.notes) : null),
            raw
          );
        }

        if (current.kind === 'recap') {
          const beats = Array.isArray(dataValue.plot_beats) ? dataValue.plot_beats : [];
          return h(
            'div',
            null,
            dataValue.last_line
              ? h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-quote' }, dataValue.last_line))
              : null,
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('location'), dataValue.location),
              renderCard(t('nextOpening'), dataValue.next_opening_hint)
            ),
            renderChips(dataValue.present, null, 'present'),
            renderSection(
              t('plotBeats'),
              beats.length ? h('ul', { className: 'ng-bullet-list' }, beats.map((beat, index) => h('li', { key: index }, beat))) : null
            ),
            raw
          );
        }

        if (current.kind === 'storyline') {
          return h(
            'div',
            null,
            h('div', { className: 'ng-preview-hero' }, h('div', { className: 'ng-lead' }, dataValue.description || '')),
            h(
              'div',
              { className: 'ng-card-grid' },
              renderCard(t('type'), dataValue.type),
              renderCard(t('importance'), dataValue.importance)
            ),
            raw
          );
        }

        return h('div', null, renderEmptyPreview(), raw);
      };

      const renderPreview = () => {
        if (!focus) return h('div', { className: 'ng-hint' }, t('noFocus'));
        const ready = preview && preview.id === focus.id;
        return h(
          'div',
          { className: 'ng-detail' },
          h(
            'div',
            { className: 'ng-detail-head' },
            h(
              'div',
              { className: 'ng-detail-kicker' },
              h('span', { className: 'ng-kind-dot', style: { background: nodeColor(focus.kind) } }),
              h('span', null, kindLabel(t, focus.kind)),
              h('span', { className: `ng-badge ${focus.status}` }, statusLabel(t, focus.status))
            ),
            h('div', { className: 'ng-detail-title' }, focus.title),
            h(
              'div',
              { className: 'ng-detail-meta' },
              focus.path ? h('code', { className: 'ng-path' }, focus.path) : null,
              focus.words ? h('span', null, `${formatNumber(focus.words)} ${t('words')}`) : null
            ),
            h(
              'div',
              { className: 'ng-detail-actions' },
              h('button', { className: 'ng-btn', type: 'button', onClick: () => toggle(focus.id) }, t('addSelect')),
              h('button', { className: 'ng-btn', type: 'button', onClick: prepareResource }, t('insertOnly')),
              h('button', { className: 'ng-btn ng-btn-primary', type: 'button', onClick: runResource }, t('runResource'))
            )
          ),
          h(
            'div',
            { className: 'ng-detail-body' },
            previewLoading
              ? h(
                  'div',
                  { className: 'ng-skeleton' },
                  h('div', { className: 'ng-skeleton-bar', style: { width: '72%' } }),
                  h('div', { className: 'ng-skeleton-bar', style: { width: '94%' } }),
                  h('div', { className: 'ng-skeleton-bar', style: { width: '86%' } })
                )
              : previewError
                ? h(
                    'div',
                    { className: 'ng-preview-error' },
                    h('div', { className: 'ng-error' }, `${t('previewFailed')}: ${previewError}`),
                    h(
                      'button',
                      {
                        className: 'ng-btn',
                        type: 'button',
                        onClick: () => setPreviewNonce((value) => value + 1)
                      },
                      t('previewRetry')
                    )
                  )
                : ready
                  ? renderPreviewBody(preview)
                  : renderEmptyPreview()
          )
        );
      };

      const renderResources = () => {
        return h(
          'div',
          null,
          h(
            'div',
            { className: 'ng-toolbar' },
            h('input', {
              className: 'ng-input',
              style: { flex: '1 1 180px' },
              placeholder: t('search'),
              value: query,
              onChange: (event) => setQuery(event.target.value)
            }),
            h(
              'select',
              { className: 'ng-select', value: kind, onChange: (event) => setKind(event.target.value) },
              h('option', { value: 'all' }, t('allKinds')),
              ['setup', 'part', 'volume', 'chapter', 'character', 'location', 'item', 'organization', 'storyline', 'recap'].map(
                (value) => h('option', { key: value, value }, kindLabel(t, value))
              )
            ),
            h(
              'select',
              {
                className: 'ng-select',
                value: volumeFilter,
                title: t('filterVolume'),
                onChange: (event) => setVolumeFilter(event.target.value)
              },
              h('option', { value: 'all' }, t('allVolumes')),
              volumeOptions.map((volume) =>
                h('option', { key: volume.id, value: volume.id }, `${volume.id} · ${volume.title}`)
              )
            ),
            h(
              'select',
              {
                className: 'ng-select',
                value: statusFilter,
                title: t('filterStatus'),
                onChange: (event) => setStatusFilter(event.target.value)
              },
              h('option', { value: 'all' }, t('allStatus')),
              h('option', { value: 'unfinished' }, t('statusUnfinished')),
              h('option', { value: 'written' }, t('statusWrittenFilter')),
              h('option', { value: 'unwritten' }, t('statusUnwrittenFilter'))
            ),
            h('span', { className: 'ng-hint' }, t('resourceCount', { n: filtered.length }))
          ),
          h(
            'div',
            { className: 'ng-split' },
            h(
              'div',
              { className: 'ng-list', role: 'list' },
              visibleResources.length
                ? visibleResources.map((resource) =>
                    h(
                      'div',
                      {
                        key: resource.id,
                        role: 'listitem',
                        className: `ng-row ${focusId === resource.id ? 'active' : ''}`,
                        onClick: () => setFocusId(resource.id)
                      },
                      h('input', {
                        className: 'ng-check',
                        type: 'checkbox',
                        checked: selected.includes(resource.id),
                        onClick: (event) => event.stopPropagation(),
                        onChange: () => toggle(resource.id),
                        'aria-label': resource.title
                      }),
                      h(
                        'div',
                        { className: 'ng-row-main' },
                        h('div', { className: 'ng-row-title' }, resource.title),
                        h(
                          'div',
                          { className: 'ng-row-sub' },
                          `${kindLabel(t, resource.kind)} · ${statusLabel(t, resource.status)}${
                            resource.summary ? ` · ${resource.summary}` : ''
                          }`
                        )
                      ),
                      h('span', { className: `ng-badge ${resource.status}` }, statusLabel(t, resource.status))
                    )
                  )
                : h('div', { className: 'ng-empty' }, t('emptyList'))
            ),
            renderPreview()
          ),
          visibleResources.length < filtered.length
            ? h(
                'div',
                { className: 'ng-actions' },
                h(
                  'button',
                  {
                    className: 'ng-btn',
                    type: 'button',
                    onClick: () => setVisibleLimit((value) => value + 120)
                  },
                  t('loadMore', { n: filtered.length - visibleResources.length })
                )
              )
            : null
        );
      };

      const nodeColor = (kindName) => {
        if (kindName === 'chapter') return 'var(--dsw-alias-brand-primary)';
        if (kindName === 'character') return 'var(--dsw-alias-state-success-primary)';
        if (kindName === 'volume' || kindName === 'part') return 'var(--dsw-alias-label-secondary)';
        if (kindName === 'location') return 'var(--dsw-alias-state-warn-primary)';
        if (kindName === 'storyline') return 'var(--dsw-alias-state-success-primary)';
        if (kindName === 'recap') return 'var(--dsw-alias-state-idle-primary)';
        return 'var(--dsw-alias-label-secondary)';
      };

      const renderGraph = () => {
        if (!focus) return h('div', { className: 'ng-empty' }, t('graphEmpty'));
        const width = 720;
        const height = 380;
        const centerX = width / 2;
        const centerY = height / 2;
        const neighborIds = [];
        for (const edge of (data && data.edges) || []) {
          if (edge[0] === focus.id && edge[1] !== focus.id) neighborIds.push(edge[1]);
          else if (edge[1] === focus.id && edge[0] !== focus.id) neighborIds.push(edge[0]);
        }
        const unique = [...new Set(neighborIds)].slice(0, 24);
        const nodes = [{ id: focus.id, resource: focus, x: centerX, y: centerY, r: 24 }];
        unique.forEach((id, index) => {
          const resource = byId.get(id);
          if (!resource) return;
          const angle = (Math.PI * 2 * index) / Math.max(1, unique.length) - Math.PI / 2;
          const ring = unique.length > 14 && index % 2 === 0 ? 260 : 200;
          nodes.push({
            id,
            resource,
            x: centerX + Math.cos(angle) * ring,
            y: centerY + Math.sin(angle) * (ring * 0.55),
            r: 15
          });
        });
        const positions = new Map(nodes.map((node) => [node.id, node]));
        const lines = [];
        for (const edge of (data && data.edges) || []) {
          const from = positions.get(edge[0]);
          const to = positions.get(edge[1]);
          if (!from || !to) continue;
          lines.push(
            h('line', {
              key: `${edge[0]}-${edge[1]}-${edge[2]}`,
              x1: from.x,
              y1: from.y,
              x2: to.x,
              y2: to.y,
              stroke: 'var(--dsw-alias-border-l2)',
              strokeWidth: 1
            })
          );
        }
        return h(
          'div',
          null,
          h(
            'svg',
            { className: 'ng-graph', viewBox: `0 0 ${width} ${height}`, role: 'img', 'aria-label': t('graph') },
            lines,
            nodes.map((node) =>
              h(
                'g',
                {
                  key: node.id,
                  className: 'ng-clickable',
                  onClick: () => setFocusId(node.id)
                },
                h('title', null, `${kindLabel(t, node.resource.kind)}: ${node.resource.title}`),
                h('circle', {
                  cx: node.x,
                  cy: node.y,
                  r: node.r,
                  style: { fill: nodeColor(node.resource.kind) },
                  stroke: 'var(--dsw-alias-bg-base)',
                  strokeWidth: 3
                }),
                h(
                  'text',
                  {
                    x: node.x,
                    y: node.y + node.r + 11,
                    textAnchor: 'middle'
                  },
                  short(node.resource.title, 13)
                )
              )
            )
          ),
          h(
            'div',
            { className: 'ng-legend' },
            ['chapter', 'character', 'location', 'volume', 'storyline', 'recap'].map((value) =>
              h(
                'span',
                { key: value, className: 'ng-legend-item' },
                h('span', { className: 'ng-legend-dot', style: { background: nodeColor(value) } }),
                kindLabel(t, value)
              )
            )
          ),
          h('div', { className: 'ng-hint', style: { marginTop: '4px' } }, t('graphHint')),
          h(
            'div',
            { className: 'ng-actions' },
            h('button', { className: 'ng-btn', type: 'button', onClick: () => toggle(focus.id) }, t('addSelect')),
            h('button', { className: 'ng-btn', type: 'button', onClick: () => setTab('resources') }, t('detail')),
            h('button', { className: 'ng-btn', type: 'button', onClick: prepareResource }, t('insertOnly')),
            h('button', { className: 'ng-btn ng-btn-primary', type: 'button', onClick: runResource }, t('runResource'))
          )
        );
      };

      const renderTray = () => {
        const items = selected
          .map((id) => byId.get(id))
          .filter(Boolean)
          .map((resource) =>
            h(
              'span',
              { key: resource.id, className: 'ng-chip', title: resource.id },
              h('span', null, short(resource.title, 22)),
              h(
                'button',
                { type: 'button', title: t('remove'), 'aria-label': t('remove'), onClick: () => toggle(resource.id) },
                '×'
              )
            )
          );
        return h(
          'div',
          null,
          h(
            'div',
            { className: 'ng-tray' },
            h('span', { className: 'ng-hint' }, `${t('selection')}: ${selected.length}`),
            items,
            h('div', { className: 'ng-spacer' }),
            h(
              'select',
              { className: 'ng-select', value: intent, onChange: (event) => setIntent(event.target.value), title: t('intent') },
              ['auto', 'write', 'improve', 'review', 'plan', 'recap', 'ask'].map((value) =>
                h('option', { key: value, value }, t(`intent_${value}`))
              )
            ),
            h('input', {
              className: 'ng-input',
              style: { flex: '1 1 180px' },
              placeholder: t('note'),
              value: note,
              onChange: (event) => setNote(event.target.value)
            }),
            selected.length
              ? h('button', { className: 'ng-btn', type: 'button', onClick: () => setSelected([]) }, t('clear'))
              : null,
            h(
              'button',
              { className: 'ng-btn', type: 'button', disabled: busy, onClick: () => doSelection(false) },
              t('insert')
            ),
            h(
              'button',
              { className: 'ng-btn ng-btn-primary', type: 'button', disabled: busy, onClick: () => doSelection(true) },
              busy ? t('loading') : t('runSelection')
            )
          ),
          h('div', { className: 'ng-hint', style: { padding: '0 10px 8px' } }, t('insertHint')),
          fallback
            ? h(
                'div',
                { style: { padding: '0 10px 10px' } },
                h('div', { className: 'ng-hint' }, t('promptFallback')),
                h('textarea', { className: 'ng-input ng-fallback', readOnly: true, value: fallback }),
                h(
                  'div',
                  { className: 'ng-actions' },
                  h('button', { className: 'ng-btn', type: 'button', onClick: copyFallback }, t('copy'))
                )
              )
            : null
        );
      };

      const renderPanel = () => {
        if (!data) {
          return h(
            'div',
            { className: `ng-panel ${isView ? 'ng-view-panel' : ''}` },
            h(EmptyProject, { t, projects, onPick: switchProject, error })
          );
        }
        return h(
          'div',
          { className: `ng-panel ${isView ? 'ng-view-panel' : ''}` },
          h(
            'div',
            { className: 'ng-head' },
            renderProjectPicker(),
            error ? h('div', { className: 'ng-error', style: { flex: '1 1 100%' } }, error) : null
          ),
          h(
            'div',
            { className: 'ng-tabs' },
            ['flow', 'resources', 'graph'].map((value) =>
              h(
                'button',
                {
                  key: value,
                  type: 'button',
                  className: `ng-tab ${tab === value ? 'active' : ''}`,
                  onClick: () => setTab(value)
                },
                t(value)
              )
            )
          ),
          h(
            'div',
            { className: 'ng-body' },
            tab === 'flow' ? renderFlow() : tab === 'resources' ? renderResources() : renderGraph()
          ),
          renderTray()
        );
      };

      if (!sessionId) return null;
      return h(
        'div',
        { className: `ng-root ${isView ? 'ng-view' : 'ng-dock'}` },
        h('style', null, CSS),
        renderBar(),
        open ? renderPanel() : null
      );
    }

    return {
      inject: ['slots', 'locale'],
      apply(ctx) {
        const t = makeT(ctx);
        ctx.slots.inject('conversation.view', () =>
          ctx.slots.register(
            {
              name: 'conversation.view',
              id: 'novelgen',
              order: 20,
              label: () => t('tabLabel')
            },
            (props) => h(NovelgenDock, { ...props, t, mode: 'view' })
          )
        );
      }
    };
  }
});
