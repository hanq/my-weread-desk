import { useEffect, useMemo, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { BookDetail, Chapter } from '../types';

type Tab = 'toc' | 'intro' | 'notes';

export function BookDetailView({ bookId }: { bookId: string }) {
  const app = useApp();
  const [detail, setDetail] = useState<BookDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [tab, setTab] = useState<Tab>('toc');
  const download = app.downloads[bookId];

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api
      .bookDetail(bookId)
      .then((d) => alive && setDetail(d))
      .catch((e: unknown) => alive && setError(String((e as Error).message || e)))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [bookId]);

  const inShelf = useMemo(() => app.books.some((b) => b.id === bookId), [app.books, bookId]);
  const progressChapter = detail?.progress?.chapterUid ?? detail?.chapters[0]?.uid ?? 0;

  if (loading) {
    return (
      <div className="empty">
        <div className="spinner" />
        <span>正在加载书籍…</span>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="empty">
        <p>{error || '无法加载这本书'}</p>
        <button className="btn" onClick={() => navigate({ name: 'shelf' })}>
          返回书架
        </button>
      </div>
    );
  }

  const book = detail.book;

  const read = (chapterUid: number, fromStart = false) => {
    navigate({ name: 'reader', bookId, chapterUid: chapterUid || progressChapter, fromStart });
  };

  const startDownload = () => {
    api
      .startDownload(bookId)
      .then((state) => store.set({ downloads: { ...store.get().downloads, [bookId]: state } }))
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  return (
    <>
      <div className="page-head">
        <button className="icon-btn" onClick={() => navigate({ name: 'shelf' })}>
          <Icon name="left" />
        </button>
        <h1 className="truncate">{book.title}</h1>
        {detail.offline ? <span className="chip">离线</span> : null}
        <div className="grow" />
        <button className="btn btn-primary" onClick={() => read(progressChapter)}>
          <Icon name="book" size={15} />
          {detail.progress && detail.progress.chapterUid ? '继续阅读' : '开始阅读'}
        </button>
        <button className="btn" onClick={() => read(detail.chapters[0]?.uid ?? 0, true)}>
          从头读
        </button>
      </div>

      <div className="detail">
        <div className="detail-side">
          <div className="detail-cover" style={{ position: 'relative' }}>
            {book.cover ? (
              <img src={book.cover} alt="" referrerPolicy="no-referrer" />
            ) : (
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  display: 'grid',
                  placeItems: 'center',
                  padding: 18,
                  textAlign: 'center',
                  background: 'linear-gradient(150deg,#243049,#171b24)',
                  color: '#c6cede',
                  fontSize: 15,
                  lineHeight: 1.5,
                }}
              >
                {book.title}
              </div>
            )}
          </div>
          <button className="btn" onClick={startDownload} disabled={download?.running}>
            <Icon name="download" size={15} />
            {download?.running ? '下载中…' : detail.cachedCount > 0 ? '补齐剩余章节' : '下载全书'}
          </button>
          {download && (download.running || download.done > 0) ? (
            <div className="faint" style={{ fontSize: 11.5, lineHeight: 1.6 }}>
              {download.done}/{download.total} 章 · 失败 {download.failed}
              <div
                style={{
                  height: 4,
                  borderRadius: 99,
                  background: 'rgba(255,255,255,0.1)',
                  marginTop: 6,
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    height: '100%',
                    width: `${download.total ? (download.done / download.total) * 100 : 0}%`,
                    background: 'var(--accent)',
                    transition: 'width 0.3s ease',
                  }}
                />
              </div>
              {download.current ? <div className="truncate">正在下载：{download.current}</div> : null}
            </div>
          ) : null}
          <button
            className="btn btn-ghost"
            onClick={() =>
              api
                .mutateShelf(bookId, inShelf ? 'remove' : 'add')
                .then(() => api.books())
                .then((list) => {
                  store.set({ books: list });
                  toast(inShelf ? '已移出书架' : '已加入书架', 'success');
                })
                .catch((e: unknown) => toast(String((e as Error).message || e), 'error'))
            }
          >
            {inShelf ? '移出书架' : '加入书架'}
          </button>
          <div className="faint" style={{ fontSize: 11.5, lineHeight: 1.7 }}>
            已缓存 {detail.cachedCount}/{detail.chapterCount} 章
            {detail.cachedCount > 0 ? (
              <button
                style={{ display: 'block', color: 'var(--danger)', fontSize: 11.5, padding: 0, marginTop: 4 }}
                onClick={() => {
                  void api.clearChapterCache(bookId).then(() => setDetail({ ...detail, cachedCount: 0 }));
                }}
              >
                清除章节缓存
              </button>
            ) : null}
          </div>
        </div>

        <div className="detail-main">
          <div>
            <div className="detail-meta" style={{ marginBottom: 8 }}>
              <span>{book.author || '佚名'}</span>
              {book.category ? <span>· {book.category}</span> : null}
              {book.wordCount > 0 ? <span>· {formatWords(book.wordCount)}</span> : null}
              {book.rating > 0 ? <span>· ★ {book.rating.toFixed(1)}</span> : null}
              {book.publisher ? <span>· {book.publisher}</span> : null}
            </div>
            {detail.progress && detail.progress.percent > 0 ? (
              <div className="chip accent">已读 {detail.progress.percent.toFixed(1)}%</div>
            ) : null}
          </div>

          <div className="seg" style={{ alignSelf: 'flex-start' }}>
            {(
              [
                ['toc', `目录 ${detail.chapters.length}`],
                ['intro', '简介'],
                ['notes', `划线 ${detail.notes.length}`],
              ] as [Tab, string][]
            ).map(([k, label]) => (
              <button key={k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>
                {label}
              </button>
            ))}
          </div>

          {tab === 'toc' ? (
            <div className="toc">
              {detail.chapters.map((c) => (
                <TocItem key={c.uid} chapter={c} current={c.uid === detail.progress?.chapterUid} onOpen={() => read(c.uid)} />
              ))}
            </div>
          ) : tab === 'intro' ? (
            <div className="detail-intro">{book.intro || '这本书没有简介。'}</div>
          ) : (
            <div className="toc">
              {detail.notes.length === 0 ? (
                <div className="empty" style={{ height: 160 }}>
                  <p>还没有划线或想法</p>
                </div>
              ) : (
                detail.notes.map((n) => (
                  <div key={n.id} style={{ padding: '10px 12px', borderBottom: '1px solid var(--chrome-line)' }}>
                    <div className="faint" style={{ fontSize: 11 }}>
                      {n.chapterName || `第 ${n.chapterUid} 章`}
                    </div>
                    <div style={{ fontSize: 13, lineHeight: 1.75, marginTop: 4 }}>{n.text || n.content}</div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function TocItem({ chapter, current, onOpen }: { chapter: Chapter; current: boolean; onOpen: () => void }) {
  return (
    <button className={`toc-item level-${Math.min(3, chapter.level || 1)}${current ? ' current' : ''}`} onClick={onOpen}>
      <span className="idx">{chapter.index}</span>
      <span className="truncate">{chapter.title || '未命名章节'}</span>
      {chapter.wordCount > 0 ? (
        <span className="faint" style={{ marginLeft: 'auto', fontSize: 11 }}>
          {formatWords(chapter.wordCount)}
        </span>
      ) : null}
    </button>
  );
}

function formatWords(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1)} 万字`;
  return `${n} 字`;
}
