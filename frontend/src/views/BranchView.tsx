import { useEffect, useMemo, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { navigate, toast } from '../store';
import type { BranchDetail, BranchScene, Intent, WorldSheet } from '../types';

export function BranchView({ branchId }: { branchId: string }) {
  const [detail, setDetail] = useState<BranchDetail | null>(null);
  const [intent, setIntent] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [conflict, setConflict] = useState<Intent | null>(null);
  const [sheet, setSheet] = useState<WorldSheet | null>(null);

  const load = async () => {
    try {
      const next = await api.branchDetail(branchId);
      setDetail(next);
      // The sheet is book-level, so it is fetched once the branch reveals its book.
      setSheet(await api.worldSheet(next.branch.bookId).catch(() => null));
      setError('');
    } catch (e) {
      setError(String((e as Error).message || e));
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [branchId]);

  const rules = useMemo(() => parseRules(detail?.branch.memoryMd ?? ''), [detail?.branch.memoryMd]);

  const cont = async () => {
    if (busy) return;
    const raw = intent.trim();
    setBusy(true);
    setError('');
    setConflict(null);
    try {
      // Compile first so a rule conflict is a choice, not a failed generation.
      if (raw) {
        const compiled = await api.compileIntent(branchId, raw);
        if (compiled.feasibility === 'violates') {
          setConflict(compiled);
          return;
        }
      }
      await api.continueBranch(branchId, raw);
      setIntent('');
      await load();
    } catch (e) {
      setError(String((e as Error).message || e));
    } finally {
      setBusy(false);
    }
  };

  const rejoin = async () => {
    try {
      await api.rejoinBranch(branchId);
      await load();
      toast('已收束这条世界线', 'success');
    } catch (e) {
      setError(String((e as Error).message || e));
    }
  };

  const exportBranch = async () => {
    try {
      const payload = await api.exportBranch(branchId);
      const ok = await copyText(payload);
      toast(ok ? '世界线包已复制到剪贴板' : '复制失败，请重试', ok ? 'success' : 'error');
    } catch (e) {
      setError(String((e as Error).message || e));
    }
  };

  const remove = async () => {
    if (!window.confirm('删除这条世界线？生成的内容会一并删除。')) return;
    try {
      await api.deleteBranch(branchId);
      navigate({ name: 'traversal' });
    } catch (e) {
      setError(String((e as Error).message || e));
    }
  };

  if (!detail) {
    return (
      <>
        <div className="page-head">
          <h1>世界线</h1>
        </div>
        <div className="page-body">
          <div className="empty">
            {error ? <p>{error}</p> : <><div className="spinner" /><span>加载中…</span></>}
          </div>
        </div>
      </>
    );
  }

  const b = detail.branch;
  const active = b.status === 'active';
  const lastScene = detail.scenes[detail.scenes.length - 1];
  const choices = active && lastScene ? lastScene.choices ?? [] : [];

  return (
    <>
      <div className="page-head">
        <h1>{b.title}</h1>
        <span className="sub">{active ? `进行中 · 目标第 ${b.rejoinSeq + 1} 幕` : '已收束'}</span>
        <div className="grow" />
        <button className="btn btn-ghost" onClick={() => navigate({ name: 'traversal' })}>
          <Icon name="left" size={14} /> 时间线
        </button>
        {active ? (
          <button className="btn btn-ghost" onClick={() => void rejoin()}>
            回到原著
          </button>
        ) : null}
        <button className="btn btn-ghost" onClick={() => void exportBranch()}>
          <Icon name="external" size={14} /> 导出
        </button>
        <button className="btn btn-ghost" onClick={() => void remove()}>
          <Icon name="close" size={14} /> 删除
        </button>
      </div>

      <div className="page-body branch-view">
        <div className="branch-rules">
          <div className="branch-rules-title">世界线宪法</div>
          {rules.length === 0 ? <div className="faint">没有解析到规则</div> : null}
          {rules.map((group) => (
            <div key={group.title} className="branch-rule-group">
              <div className="branch-rule-head">{group.title}</div>
              <ul>
                {group.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </div>
          ))}
          {(b.state.assumptions ?? []).length > 0 ? (
            <div className="branch-rule-group">
              <div className="branch-rule-head">既有设定</div>
              <ul>
                {(b.state.assumptions ?? []).map((a) => (
                  <li key={a}>{a}</li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>

        {sheet && sheet.stats.length > 0 ? (
          <div className="branch-sheet">
            <div className="branch-sheet-title">状态</div>
            <div className="branch-sheet-stats">
              {sheet.stats.map((def) => {
                const value = b.state.stats?.[def.key] ?? def.initial;
                const span = def.max - def.min;
                const pct = span > 0 ? Math.round(((value - def.min) / span) * 100) : 0;
                return (
                  <div key={def.key} className="stat-row">
                    <span className="stat-label">{def.label}</span>
                    <span className="stat-bar">
                      <span style={{ width: `${Math.max(0, Math.min(100, pct))}%` }} />
                    </span>
                    <span className="stat-value">{Math.round(value)}</span>
                  </div>
                );
              })}
            </div>
            {Object.keys(b.state.inventory ?? {}).length > 0 ? (
              <div className="branch-sheet-inventory">
                {Object.entries(b.state.inventory ?? {}).map(([id, qty]) => {
                  const item = sheet.items.find((it) => it.id === id);
                  return (
                    <span key={id} className="inventory-chip">
                      {item?.name ?? id} ×{qty}
                    </span>
                  );
                })}
              </div>
            ) : null}
          </div>
        ) : null}

        {b.summary ? <div className="branch-summary">{b.summary}</div> : null}

        <div className="branch-scenes">
          {detail.scenes.length === 0 ? (
            <div className="empty">
              <Icon name="sparkle" size={28} />
              <p>还没有生成任何一幕。</p>
              <p className="faint">在下方输入你的想法，然后继续。</p>
            </div>
          ) : null}
          {detail.scenes.map((sc) => (
            <BranchSceneCard key={sc.seq} scene={sc} />
          ))}
        </div>

        {choices.length > 0 ? (
          <div className="branch-choices">
            <div className="branch-choices-title">接下来</div>
            {choices.map((c) => (
              <button
                key={c.id}
                className="choice-chip"
                onClick={() => setIntent(c.text)}
                title={c.hint || '选用这个选择'}
              >
                {c.text}
              </button>
            ))}
          </div>
        ) : null}

        {conflict ? (
          <div className="branch-conflict">
            <div className="branch-conflict-title">这个想法与设定冲突</div>
            <div className="branch-conflict-reason">{conflict.reason}</div>
            {(conflict.blockedBy ?? []).length > 0 ? (
              <div className="faint">触发规则：{(conflict.blockedBy ?? []).join('、')}</div>
            ) : null}
            <div className="branch-conflict-alts">
              {(conflict.alternatives ?? []).map((alt) => (
                <button
                  key={alt}
                  className="btn btn-ghost"
                  onClick={() => {
                    setIntent(alt);
                    setConflict(null);
                  }}
                >
                  {alt}
                </button>
              ))}
            </div>
            <div>
              <button className="btn btn-ghost" onClick={() => setConflict(null)}>
                取消
              </button>
            </div>
          </div>
        ) : null}

        {error ? <div className="assistant-error">{error}</div> : null}

        {active ? (
          <div className="branch-input">
            <input
              value={intent}
              placeholder="这一幕你想让发生什么？（可留空）"
              onChange={(e) => setIntent(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  void cont();
                }
              }}
            />
            <button className="btn btn-primary" disabled={busy} onClick={() => void cont()}>
              {busy ? '生成中…' : detail.scenes.length ? '继续一幕' : '写下第一幕'}
            </button>
          </div>
        ) : null}
      </div>
    </>
  );
}

function BranchSceneCard({ scene }: { scene: BranchScene }) {
  const hard = (scene.verdicts ?? []).filter((v) => v.severity === 'hard');
  return (
    <div className="branch-scene">
      <div className="branch-scene-head">
        <span>
          第 {scene.seq + 1} 幕 · 对应原著第 {scene.canonSeq + 1} 幕
        </span>
        {scene.model ? <span className="faint">{scene.model}</span> : null}
      </div>
      <div className="branch-scene-text">{scene.text}</div>
      {hard.length > 0 ? (
        <div className="branch-scene-warn">校验未通过：{hard.map((v) => v.rule).join('、')}</div>
      ) : null}
    </div>
  );
}

// parseRules pulls the user block out of memory.md. It is deliberately tolerant:
// a hand-edited file may have extra prose, and losing the rules silently would
// be worse than showing fewer of them.
function parseRules(md: string): { title: string; items: string[] }[] {
  const user = md.split('<!-- USER:BEGIN -->')[1]?.split('<!-- USER:END -->')[0] ?? '';
  const out: { title: string; items: string[] }[] = [];
  let current: { title: string; items: string[] } | null = null;
  for (const raw of user.split('\n')) {
    const line = raw.trim();
    if (line.startsWith('## ')) {
      current = { title: line.slice(3).trim(), items: [] };
      out.push(current);
    } else if (line.startsWith('- ') && current) {
      current.items.push(line.slice(2).trim());
    }
  }
  return out.filter((g) => g.items.length > 0);
}

// copyText falls back to execCommand because the WebView clipboard API is not
// available in every embedding context.
async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const el = document.createElement('textarea');
    el.value = text;
    el.style.position = 'fixed';
    el.style.opacity = '0';
    document.body.appendChild(el);
    el.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(el);
    return ok;
  }
}
