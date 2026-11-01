import { useEffect, useMemo, useRef, useState } from 'react';

import { api, events } from '../api';
import { Icon } from '../icons';
import { canTraverse } from '../bookKind';
import { navigate, toast, useApp } from '../store';
import type { CanonAnchor, StoryGraphStatus, StoryScene } from '../types';

export function TraversalView({ initialBookId }: { initialBookId?: string } = {}) {
  const app = useApp();
  const books = useMemo(
    () => app.books.filter((b) => !b.isAudio && !b.isMp && canTraverse(b.kind)),
    [app.books],
  );

  const [bookId, setBookId] = useState('');
  const [status, setStatus] = useState<StoryGraphStatus | null>(null);
  const [scenes, setScenes] = useState<StoryScene[]>([]);
  const [anchors, setAnchors] = useState<CanonAnchor[]>([]);
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importPayload, setImportPayload] = useState('');

  const bookRef = useRef('');
  bookRef.current = bookId;
  const appliedInitial = useRef('');

  // Opening from the shelf or detail page should land on that book once, while
  // still allowing the reader to switch to another book inside the view.
  useEffect(() => {
    if (books.length === 0) return;
    if (initialBookId && books.some((b) => b.id === initialBookId)) {
      if (appliedInitial.current !== initialBookId) {
        appliedInitial.current = initialBookId;
        setBookId(initialBookId);
      }
      return;
    }
    if (bookId) return;
    const last = app.settings?.lastBookId;
    const pick = books.find((b) => b.id === last) ?? books[0];
    setBookId(pick.id);
  }, [initialBookId, app.settings?.lastBookId, bookId, books]);

  const reload = async (id: string) => {
    const [nextStatus, nextScenes, nextAnchors] = await Promise.all([
      api.storyGraphStatus(id).catch(() => null),
      api.storyTimeline(id).catch(() => [] as StoryScene[]),
      api.storyAnchors(id).catch(() => [] as CanonAnchor[]),
    ]);
    setStatus(nextStatus);
    setScenes(nextScenes);
    setAnchors(nextAnchors);
  };

  useEffect(() => {
    if (!bookId) return;
    setScenes([]);
    setAnchors([]);
    setStatus(null);
    void reload(bookId);
  }, [bookId]);

  // Progress comes from the Go side; once the job stops, pull the timeline.
  useEffect(() => {
    return events.on('story:progress', (ev: StoryGraphStatus) => {
      if (!ev || ev.bookId !== bookRef.current) return;
      setStatus(ev);
      if (ev.status !== 'running') {
        api
          .storyTimeline(ev.bookId)
          .then(setScenes)
          .catch(() => {});
      }
    });
  }, []);

  const build = async () => {
    if (!bookId || busy) return;
    setBusy(true);
    try {
      setStatus(await api.buildStoryGraph(bookId));
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    } finally {
      setBusy(false);
    }
  };

  const doImport = async () => {
    if (!bookId) return;
    try {
      const branch = await api.importBranch(bookId, importPayload);
      setImporting(false);
      setImportPayload('');
      navigate({ name: 'branch', branchId: branch.id });
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  const fork = async (seq: number) => {
    if (!bookId) return;
    try {
      const branch = await api.startBranch(bookId, seq, '');
      navigate({ name: 'branch', branchId: branch.id });
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  const running = status?.status === 'running';
  const indexed = (status?.sceneCount ?? 0) > 0;
  const percent = status && status.total > 0 ? Math.round((status.done / status.total) * 100) : 0;

  const anchorByScene = useMemo(() => {
    const map = new Map<number, CanonAnchor[]>();
    for (const a of anchors) {
      const list = map.get(a.sceneSeq) ?? [];
      list.push(a);
      map.set(a.sceneSeq, list);
    }
    return map;
  }, [anchors]);

  // Candidates are never adopted automatically: an anchor decides what the book
  // is about, so the reader confirms before it enters the canon.
  const suggest = async () => {
    if (!bookId) return;
    try {
      const candidates = await api.suggestAnchors(bookId);
      if (candidates.length === 0) {
        toast('没有可用的候选锚点', 'info');
        return;
      }
      if (!window.confirm(`模型给出 ${candidates.length} 个候选锚点，全部采用？`)) return;
      let next = anchors;
      for (const c of candidates) {
        next = await api.saveAnchor(c);
      }
      setAnchors(next);
      toast(`已采用 ${candidates.length} 个锚点`, 'success');
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  // The sheet decides what the reader can and cannot do, so candidates are
  // shown for confirmation instead of being written straight into the book.
  const deriveWorld = async () => {
    if (!bookId) return;
    try {
      const sheet = await api.deriveWorldSheet(bookId);
      const summary = [
        `${sheet.stats.length} 项属性：${sheet.stats.map((s) => s.label).join('、')}`,
        `${sheet.items.length} 件道具`,
      ].join('\n');
      if (!window.confirm(`${summary}\n\n采用这张世界表？`)) return;
      await api.saveWorldSheet(bookId, sheet);
      toast('世界表已保存', 'success');
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  const removeAnchor = async (id: string) => {
    if (!bookId) return;
    try {
      setAnchors(await api.deleteAnchor(bookId, id));
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  if (books.length === 0) {
    return (
      <>
        <div className="page-head">
          <h1>故事穿越</h1>
          <span className="sub">时间线 · 平行世界</span>
        </div>
        <div className="page-body">
          <div className="empty">
            <Icon name="clock" size={30} />
            <p>没有可穿越的小说。</p>
            <p className="faint">
              故事穿越只支持虚构叙事。如果这本书是小说，请在书籍详情里把 AI 类型改为「小说」。
            </p>
            <button className="btn" onClick={() => navigate({ name: 'shelf' })}>
              回到书架
            </button>
          </div>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="page-head">
        <h1>故事穿越</h1>
        <span className="sub">
          {scenes.length > 0 ? `${scenes.length} 幕 · 时间线与平行世界` : '时间线 · 平行世界'}
        </span>
        <div className="grow" />
        <select className="assistant-select" value={bookId} onChange={(e) => setBookId(e.target.value)}>
          {books.map((b) => (
            <option key={b.id} value={b.id}>
              {b.title}
            </option>
          ))}
        </select>
        <button
          className="btn btn-ghost"
          title="查找原文事实、定位情节请使用读书助手"
          onClick={() => navigate({ name: 'assistant', bookId: bookId || undefined })}
        >
          <Icon name="sparkle" size={14} /> 读书助手
        </button>
        {running ? (
          <button className="btn btn-ghost" onClick={() => void api.cancelStoryGraph(bookId)}>
            停止
          </button>
        ) : (
          <button className="btn btn-primary" disabled={!bookId || busy} onClick={() => void build()}>
            <Icon name="refresh" size={14} /> {indexed ? '重建时间线' : '建立时间线'}
          </button>
        )}
        {indexed && !running ? (
          <button className="btn btn-ghost" onClick={() => void suggest()}>
            <Icon name="star" size={14} /> 锚点{anchors.length ? ` ${anchors.length}` : ''}
          </button>
        ) : null}
        {indexed && !running ? (
          <button className="btn btn-ghost" onClick={() => void deriveWorld()}>
            <Icon name="folder" size={14} /> 世界表
          </button>
        ) : null}
        <button className="btn btn-ghost" onClick={() => setImporting((v) => !v)}>
          <Icon name="external" size={14} /> 导入
        </button>
      </div>

      <div className="page-body traversal">
        <div className="traversal-status">
          <Icon name="clock" size={14} />
          {running ? (
            <span>
              正在抽取 {status?.done}/{status?.total} 章（{percent}%）
            </span>
          ) : indexed ? (
            <span>
              {status?.sceneCount} 幕 · {status?.failed ? `${status.failed} 章未标注` : '全部完成'}
            </span>
          ) : (
            <span>{status?.error ? `失败：${status.error}` : '尚未建立时间线'}</span>
          )}
          <div className="grow" />
          {indexed ? (
            <button
              className="btn btn-ghost"
              onClick={() => void api.dropStoryGraph(bookId).then(() => reload(bookId))}
            >
              删除
            </button>
          ) : null}
        </div>

        {running && status ? (
          <div className="traversal-progress">
            <span style={{ width: `${percent}%` }} />
          </div>
        ) : null}

        {importing ? (
          <div className="import-panel">
            <textarea
              value={importPayload}
              placeholder="粘贴世界线包（JSON）"
              onChange={(e) => setImportPayload(e.target.value)}
            />
            <div className="import-actions">
              <button
                className="btn btn-primary"
                disabled={!importPayload.trim()}
                onClick={() => void doImport()}
              >
                导入
              </button>
              <button
                className="btn btn-ghost"
                onClick={() => {
                  setImporting(false);
                  setImportPayload('');
                }}
              >
                取消
              </button>
            </div>
          </div>
        ) : null}

        {scenes.length === 0 && !running ? (
          <div className="empty">
            <Icon name="clock" size={30} />
            <p>时间线还没有建立。</p>
            <p className="faint">需要先下载本书的章节。未配置 AI 时，每一章直接作为一个场景。</p>
            <p className="faint">只想查原文事实，请回到「读书助手」。</p>
          </div>
        ) : (
          <div className="timeline">
            {scenes.map((scene) => {
              const sceneAnchors = anchorByScene.get(scene.seq) ?? [];
              return (
                <div
                  key={scene.seq}
                  role="button"
                  tabIndex={0}
                  className={`timeline-item${sceneAnchors.length ? ' has-anchor' : ''}`}
                  onClick={() =>
                    navigate({ name: 'reader', bookId: scene.bookId, chapterUid: scene.chapterUid })
                  }
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      navigate({ name: 'reader', bookId: scene.bookId, chapterUid: scene.chapterUid });
                    }
                  }}
                  title="打开这一幕对应的章节"
                >
                  <span className="timeline-seq">{scene.seq + 1}</span>
                  <span className="timeline-body">
                    <span className="timeline-title">{scene.chapterTitle || `第 ${scene.chapterIdx} 章`}</span>
                    <span className="timeline-summary">{scene.summary || '（本章没有摘要）'}</span>
                    <span className="timeline-meta">
                      {scene.location ? <span className="timeline-badge">{scene.location}</span> : null}
                      {scene.timeHint ? <span className="timeline-badge">{scene.timeHint}</span> : null}
                      {scene.pov ? <span className="timeline-badge">视角 {scene.pov}</span> : null}
                      {scene.mood ? <span className="timeline-badge accent">{scene.mood}</span> : null}
                      {(scene.participants ?? []).slice(0, 4).map((p) => (
                        <span key={p} className="timeline-badge">
                          {p}
                        </span>
                      ))}
                      {scene.source === 'chapter' ? <span className="timeline-badge faint">未标注</span> : null}
                      <button
                        className="timeline-badge fork"
                        title="从这一幕分叉出平行世界"
                        onClick={(e) => {
                          e.stopPropagation();
                          void fork(scene.seq);
                        }}
                      >
                        分叉
                      </button>
                    </span>
                    {sceneAnchors.map((a) => (
                      <span key={a.id} className="timeline-anchor">
                        <span className="timeline-anchor-kind">{kindLabel(a.kind)}</span>
                        <span className="grow">{a.summary}</span>
                        <button
                          className="anchor-del"
                          title="删除锚点"
                          onClick={(e) => {
                            e.stopPropagation();
                            void removeAnchor(a.id);
                          }}
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </span>
                  <Icon name="right" size={13} />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </>
  );
}

const KIND_LABEL: Record<string, string> = {
  identity: '身份',
  relationship: '关系',
  reveal: '揭示',
  climax: '高潮',
  ending: '结局',
  beat: '节点',
};

function kindLabel(kind: string): string {
  return KIND_LABEL[kind] ?? '节点';
}
