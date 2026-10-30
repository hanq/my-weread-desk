import { useEffect, useMemo, useRef, useState } from 'react';

import { api, events } from '../api';
import { Icon } from '../icons';
import { navigate, toast, useApp } from '../store';
import type { StoryGraphStatus, StoryScene } from '../types';

export function TraversalView() {
  const app = useApp();
  const books = useMemo(() => app.books.filter((b) => !b.isAudio && !b.isMp), [app.books]);

  const [bookId, setBookId] = useState('');
  const [status, setStatus] = useState<StoryGraphStatus | null>(null);
  const [scenes, setScenes] = useState<StoryScene[]>([]);
  const [busy, setBusy] = useState(false);

  const bookRef = useRef('');
  bookRef.current = bookId;

  useEffect(() => {
    if (bookId || books.length === 0) return;
    const last = app.settings?.lastBookId;
    const pick = books.find((b) => b.id === last) ?? books[0];
    setBookId(pick.id);
  }, [app.settings?.lastBookId, bookId, books]);

  const reload = async (id: string) => {
    const [nextStatus, nextScenes] = await Promise.all([
      api.storyGraphStatus(id).catch(() => null),
      api.storyTimeline(id).catch(() => [] as StoryScene[]),
    ]);
    setStatus(nextStatus);
    setScenes(nextScenes);
  };

  useEffect(() => {
    if (!bookId) return;
    setScenes([]);
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

  const running = status?.status === 'running';
  const indexed = (status?.sceneCount ?? 0) > 0;
  const percent = status && status.total > 0 ? Math.round((status.done / status.total) * 100) : 0;

  return (
    <>
      <div className="page-head">
        <h1>穿越</h1>
        <span className="sub">{scenes.length > 0 ? `${scenes.length} 幕` : '把书拆成时间线'}</span>
        <div className="grow" />
        <select className="assistant-select" value={bookId} onChange={(e) => setBookId(e.target.value)}>
          {books.map((b) => (
            <option key={b.id} value={b.id}>
              {b.title}
            </option>
          ))}
        </select>
        {running ? (
          <button className="btn btn-ghost" onClick={() => void api.cancelStoryGraph(bookId)}>
            停止
          </button>
        ) : (
          <button className="btn btn-primary" disabled={!bookId || busy} onClick={() => void build()}>
            <Icon name="refresh" size={14} /> {indexed ? '重建时间线' : '建立时间线'}
          </button>
        )}
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

        {scenes.length === 0 && !running ? (
          <div className="empty">
            <Icon name="clock" size={30} />
            <p>时间线还没有建立。</p>
            <p className="faint">需要先下载本书的章节。未配置 AI 时，每一章直接作为一个场景。</p>
          </div>
        ) : (
          <div className="timeline">
            {scenes.map((scene) => (
              <button
                key={scene.seq}
                className="timeline-item"
                onClick={() =>
                  navigate({ name: 'reader', bookId: scene.bookId, chapterUid: scene.chapterUid })
                }
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
                  </span>
                </span>
                <Icon name="right" size={13} />
              </button>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
