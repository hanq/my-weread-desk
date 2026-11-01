import { useEffect, useMemo, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { BOOK_KIND_OPTIONS, bookKindLabel, canTraverse, firstReadableChapter } from '../bookKind';
import { navigate, store, toast, useApp } from '../store';
import type { BookDetail, BookKind, Chapter } from '../types';
import { coverUrl, handleCoverError } from '../bookCover';

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
    setDetail(null);
    setError('');
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
  const progressChapter = detail?.progress?.chapterUid || firstReadableChapter(detail?.chapters ?? [])?.uid || 0;

  if (loading) {
    return (
      <>
        <div className="page-head">
          <button className="icon-btn" onClick={() => navigate({ name: 'shelf' })} title="返回书架">
            <Icon name="left" />
          </button>
          <h1 className="truncate">书籍详情</h1>
        </div>
        <div className="empty">
          <div className="spinner" />
          <span>正在加载书籍…</span>
        </div>
      </>
    );
  }

  if (!detail) {
    return (
      <>
        <div className="page-head">
          <button className="icon-btn" onClick={() => navigate({ name: 'shelf' })} title="返回书架">
            <Icon name="left" />
          </button>
          <h1 className="truncate">书籍详情</h1>
        </div>
        <div className="empty">
          <p>{error || '无法加载这本书'}</p>
          <button className="btn" onClick={() => navigate({ name: 'shelf' })}>
            返回书架
          </button>
        </div>
      </>
    );
  }

  const book = detail.book;
  const hasProgress = Boolean(detail.progress && detail.progress.chapterUid);
  const readLabel = hasProgress ? '继续阅读' : '开始阅读';

  const read = (chapterUid: number, fromStart = false) => {
    navigate({ name: 'reader', bookId, chapterUid: chapterUid || progressChapter, fromStart });
  };

  const startDownload = () => {
    api
      .startDownload(bookId)
      .then((state) => store.set({ downloads: { ...store.get().downloads, [bookId]: state } }))
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  const toggleShelf = () => {
    api
      .mutateShelf(bookId, inShelf ? 'remove' : 'add')
      .then(() => api.books())
      .then((list) => {
        store.set({ books: list });
        toast(inShelf ? '已移出书架' : '已加入书架', 'success');
      })
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  const clearCache = () => {
    void api
      .clearChapterCache(bookId)
      .then(() => setDetail((prev) => (prev ? { ...prev, cachedCount: 0 } : prev)))
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  const updateKind = (kind: BookKind) => {
    void api
      .setBookKind(bookId, kind)
      .then((next) => {
        setDetail((prev) =>
          prev ? { ...prev, book: { ...prev.book, kind: next, kindSource: 'user' } } : prev,
        );
        api.books().then((list) => store.set({ books: list })).catch(() => {});
        toast('已更新 AI 类型', 'success');
      })
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  return (
    <>
      <div className="page-head">
        <button className="icon-btn" onClick={() => navigate({ name: 'shelf' })} title="返回书架">
          <Icon name="left" />
        </button>
        <h1 className="truncate">{book.title}</h1>
        {detail.offline ? <span className="chip">离线</span> : null}
      </div>

      <div className="detail">
        <aside className="detail-side">
          <div className="detail-cover">
            {coverUrl(book) ? (
              <img
                src={coverUrl(book)}
                alt=""
                referrerPolicy="no-referrer"
                onError={(e) => handleCoverError(book, e)}
              />
            ) : (
              <div className="detail-cover-fallback">{book.title}</div>
            )}
          </div>
        </aside>

        <div className="detail-main">
          <div className="detail-heading">
            <div className="detail-meta">
              <span>{book.author || '佚名'}</span>
              {book.category ? <span>· {book.category}</span> : null}
              {book.wordCount > 0 ? <span>· {formatWords(book.wordCount)}</span> : null}
              {book.rating > 0 ? <span>· ★ {book.rating.toFixed(1)}</span> : null}
              {book.publisher ? <span>· {book.publisher}</span> : null}
              {book.kind ? <span className="chip">AI 类型 · {bookKindLabel(book.kind)}</span> : null}
            </div>
            {detail.progress && detail.progress.percent > 0 ? (
              <div className="chip accent">已读 {detail.progress.percent.toFixed(1)}%</div>
            ) : null}
          </div>

          <div className="detail-actions">
            <button className="btn btn-primary" onClick={() => read(progressChapter)}>
              <Icon name="book" size={15} />
              {readLabel}
            </button>
            <button className="btn" onClick={() => read(firstReadableChapter(detail.chapters)?.uid ?? 0, true)}>
              从头读
            </button>
            <button className="btn" onClick={startDownload} disabled={download?.running}>
              <Icon name="download" size={15} />
              {download?.running ? '下载中…' : detail.cachedCount > 0 ? '补齐剩余章节' : '下载全书'}
            </button>
            {!book.isAudio && !book.isMp ? (
              <>
                <button
                  className="btn btn-ghost"
                  title="基于本书原文检索并定位情节"
                  onClick={() => navigate({ name: 'assistant', bookId })}
                >
                  <Icon name="sparkle" size={15} /> 问助手
                </button>
                {canTraverse(book.kind) ? (
                  <button
                    className="btn btn-ghost"
                    title="把本书拆成时间线，进入平行世界"
                    onClick={() => navigate({ name: 'traversal', bookId })}
                  >
                    <Icon name="clock" size={15} /> 穿越此书
                  </button>
                ) : null}
              </>
            ) : null}
            {!book.isAudio && !book.isMp ? (
              <label className="detail-kind">
                <span>AI 类型</span>
                <select
                  value={book.kind ?? 'other'}
                  onChange={(e) => void updateKind(e.target.value as BookKind)}
                >
                  {BOOK_KIND_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {!book.isAudio && !book.isMp && !canTraverse(book.kind) ? (
              <span className="detail-kind-note">
                故事穿越只对小说开放；如果识别有误，请把 AI 类型改为「小说」。
              </span>
            ) : null}
            <button className="btn btn-ghost" onClick={toggleShelf}>
              {inShelf ? '移出书架' : '加入书架'}
            </button>
            {detail.cachedCount > 0 ? (
              <button className="btn btn-ghost detail-clear" onClick={clearCache}>
                清除章节缓存
              </button>
            ) : null}
            <span className="detail-cache-note">
              已缓存 {detail.cachedCount}/{detail.chapterCount} 章
            </span>
          </div>

          {download && (download.running || download.done > 0) ? (
            <div className="detail-download">
              <div className="detail-download-row">
                <span>
                  {download.done}/{download.total} 章 · 失败 {download.failed}
                </span>
                {download.current ? <span className="truncate">正在下载：{download.current}</span> : null}
              </div>
              <div className="detail-download-bar">
                <span
                  style={{
                    width: `${download.total ? (download.done / download.total) * 100 : 0}%`,
                  }}
                />
              </div>
            </div>
          ) : null}

          <div className="detail-tabs">
            <div className="seg">
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
            <div className="detail-notes">
              {detail.notes.length === 0 ? (
                <div className="empty" style={{ height: 160 }}>
                  <p>还没有划线或想法</p>
                </div>
              ) : (
                detail.notes.map((n) => (
                  <div key={n.id} className="detail-note">
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
