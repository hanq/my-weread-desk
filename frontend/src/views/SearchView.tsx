import { useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { navigate, toast } from '../store';
import type { Book, SearchResult } from '../types';

const SCOPES: [string, string][] = [
  ['ebook', '电子书'],
  ['all', '全部'],
  ['fulltext', '全文'],
  ['author', '作者'],
  ['mp', '公众号'],
];

export function SearchView() {
  const [keyword, setKeyword] = useState('');
  const [scope, setScope] = useState('ebook');
  const [result, setResult] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(0);

  const run = (nextPage = 0) => {
    const q = keyword.trim();
    if (!q) return;
    setLoading(true);
    api
      .search(q, scope, nextPage)
      .then((res) => {
        setPage(nextPage);
        setResult((prev) => (nextPage === 0 || !prev ? res : { ...res, books: dedupe([...prev.books, ...res.books]) }));
      })
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'))
      .finally(() => setLoading(false));
  };

  return (
    <>
      <div className="page-head">
        <h1>搜索</h1>
        <span className="sub">微信读书书城</span>
      </div>

      <div className="shelf-toolbar">
        <div
          className="input"
          style={{ display: 'flex', alignItems: 'center', gap: 8, maxWidth: 460 }}
        >
          <Icon name="search" size={15} />
          <input
            autoFocus
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') run(0);
            }}
            placeholder="书名、作者或全文关键字"
            style={{ border: 0, background: 'transparent', outline: 'none', width: '100%' }}
          />
          {keyword ? (
            <button className="icon-btn" style={{ width: 22, height: 22 }} onClick={() => setKeyword('')}>
              <Icon name="close" size={12} />
            </button>
          ) : null}
        </div>
        <div className="seg">
          {SCOPES.map(([k, label]) => (
            <button key={k} className={scope === k ? 'on' : ''} onClick={() => setScope(k)}>
              {label}
            </button>
          ))}
        </div>
        <button className="btn btn-primary" onClick={() => run(0)} disabled={loading || !keyword.trim()}>
          {loading ? '搜索中' : '搜索'}
        </button>
      </div>

      <div className="page-body">
        {!result ? (
          <div className="empty">
            <Icon name="search" size={32} />
            <p>输入关键字开始搜索</p>
          </div>
        ) : result.books.length === 0 ? (
          <div className="empty">
            <p>没有找到相关结果</p>
          </div>
        ) : (
          <>
            <div className="shelf-grid">
              {result.books.map((b) => (
                <BookResult key={b.id} book={b} />
              ))}
            </div>
            {result.hasMore ? (
              <div style={{ display: 'flex', justifyContent: 'center', padding: 22 }}>
                <button className="btn" onClick={() => run(page + 1)} disabled={loading}>
                  {loading ? '加载中…' : '加载更多'}
                </button>
              </div>
            ) : null}
          </>
        )}
      </div>
    </>
  );
}

function BookResult({ book }: { book: Book }) {
  return (
    <button className="book-card" onClick={() => navigate({ name: 'detail', bookId: book.id })}>
      <div className="cover">
        {book.cover ? (
          <img src={book.cover} alt="" loading="lazy" referrerPolicy="no-referrer" />
        ) : (
          <div className="fallback">{book.title}</div>
        )}
        {book.rating > 0 ? <span className="cached">{book.rating.toFixed(1)} 分</span> : null}
      </div>
      <div className="book-title">{book.title}</div>
      <div className="book-author">{book.author}</div>
    </button>
  );
}

function dedupe(list: Book[]): Book[] {
  const seen = new Set<string>();
  return list.filter((b) => (seen.has(b.id) ? false : (seen.add(b.id), true)));
}
