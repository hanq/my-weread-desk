import { useMemo, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { Book } from '../types';

type Filter = 'all' | 'reading' | 'finished' | 'mp';
type Layout = 'grid' | 'list';

export function ShelfView() {
  const app = useApp();
  const [filter, setFilter] = useState<Filter>('all');
  const [layout, setLayout] = useState<Layout>('grid');
  const [query, setQuery] = useState('');
  const [menu, setMenu] = useState<{ book: Book; x: number; y: number } | null>(null);

  const books = useMemo(() => {
    let list = app.books;
    if (filter === 'finished') list = list.filter((b) => b.finished);
    if (filter === 'reading') list = list.filter((b) => !b.finished && b.lastRead > 0);
    if (filter === 'mp') list = list.filter((b) => b.isMp);
    const q = query.trim().toLowerCase();
    if (q) list = list.filter((b) => b.title.toLowerCase().includes(q) || b.author.toLowerCase().includes(q));
    return list;
  }, [app.books, filter, query]);

  const open = (book: Book) => {
    if (book.isMp) {
      navigate({ name: 'mp' });
      return;
    }
    navigate({ name: 'detail', bookId: book.id });
  };

  const mutate = (book: Book, action: string) => {
    setMenu(null);
    api
      .mutateShelf(book.id, action)
      .then(() => api.books())
      .then((list) => {
        store.set({ books: list });
        toast(action === 'remove' ? '已移出书架' : '已更新', 'success');
      })
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  return (
    <>
      <div className="page-head">
        <h1>书架</h1>
        <span className="sub">{app.books.length} 本</span>
        <div className="grow" />
        <div className="seg">
          {(
            [
              ['all', '全部'],
              ['reading', '在读'],
              ['finished', '读完'],
              ['mp', '公众号'],
            ] as [Filter, string][]
          ).map(([k, label]) => (
            <button key={k} className={filter === k ? 'on' : ''} onClick={() => setFilter(k)}>
              {label}
            </button>
          ))}
        </div>
        <button className={`icon-btn${layout === 'grid' ? ' active' : ''}`} onClick={() => setLayout('grid')} title="网格">
          <Icon name="columns" size={16} />
        </button>
        <button className={`icon-btn${layout === 'list' ? ' active' : ''}`} onClick={() => setLayout('list')} title="列表">
          <Icon name="list" size={16} />
        </button>
      </div>

      <div className="shelf-toolbar">
        <div className="input" style={{ display: 'flex', alignItems: 'center', gap: 8, maxWidth: 320 }}>
          <Icon name="search" size={14} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="在书架中筛选"
            style={{ border: 0, background: 'transparent', outline: 'none', width: '100%' }}
          />
        </div>
        <div className="grow" />
        <button className="btn btn-ghost" onClick={() => navigate({ name: 'search' })}>
          <Icon name="search" size={14} /> 搜索书城
        </button>
      </div>

      <div className="page-body">
        {app.shelfState === 'loading' && app.books.length === 0 ? (
          <div className="empty">
            <div className="spinner" />
            <span>正在同步书架…</span>
          </div>
        ) : books.length === 0 ? (
          <div className="empty">
            <Icon name="book" size={34} />
            <p>{query ? '没有匹配的书' : '书架还是空的，先同步一次吧'}</p>
            {!query ? (
              <button
                className="btn btn-primary"
                onClick={() => {
                  store.set({ shelfState: 'loading' });
                  api
                    .syncShelf()
                    .then((list) => store.set({ books: list, shelfState: 'idle', online: true }))
                    .catch((e: unknown) => {
                      store.set({ shelfState: 'error', online: false });
                      toast(String((e as Error).message || e), 'error');
                    });
                }}
              >
                同步书架
              </button>
            ) : null}
          </div>
        ) : layout === 'grid' ? (
          <div className="shelf-grid">
            {books.map((b) => (
              <button
                key={b.id}
                className="book-card"
                onClick={() => open(b)}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setMenu({ book: b, x: e.clientX, y: e.clientY });
                }}
              >
                <div className="cover">
                  {b.cover ? (
                    <img src={b.cover} alt="" loading="lazy" referrerPolicy="no-referrer" />
                  ) : (
                    <div className="fallback">{b.title}</div>
                  )}
                  {b.finished ? <span className="cached">已读完</span> : null}
                </div>
                <div className="book-title">{b.title}</div>
                <div className="book-author">{b.author || (b.isMp ? '公众号' : '')}</div>
              </button>
            ))}
          </div>
        ) : (
          <div className="shelf-list">
            {books.map((b) => (
              <div
                key={b.id}
                className="book-row"
                onClick={() => open(b)}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setMenu({ book: b, x: e.clientX, y: e.clientY });
                }}
              >
                <div className="cover">
                  {b.cover ? <img src={b.cover} alt="" loading="lazy" referrerPolicy="no-referrer" /> : null}
                </div>
                <div className="grow">
                  <div className="truncate" style={{ fontSize: 13.5 }}>
                    {b.title}
                  </div>
                  <div className="faint" style={{ fontSize: 11.5 }}>
                    {b.author}
                    {b.category ? ` · ${b.category}` : ''}
                  </div>
                </div>
                {b.finished ? <span className="chip">已读完</span> : null}
                {b.secret ? <span className="chip">私密</span> : null}
                {b.isMp ? <span className="chip accent">公众号</span> : null}
              </div>
            ))}
          </div>
        )}
      </div>

      {menu ? (
        <>
          <div style={{ position: 'fixed', inset: 0, zIndex: 40 }} onClick={() => setMenu(null)} onContextMenu={(e) => e.preventDefault()} />
          <div
            style={{
              position: 'fixed',
              left: Math.min(menu.x, window.innerWidth - 190),
              top: Math.min(menu.y, window.innerHeight - 200),
              zIndex: 41,
              width: 178,
              padding: 5,
              borderRadius: 9,
              background: '#232838',
              border: '1px solid rgba(255,255,255,0.1)',
              boxShadow: 'var(--shadow-2)',
            }}
          >
            <MenuItem icon="book" label="打开详情" onClick={() => { setMenu(null); navigate({ name: 'detail', bookId: menu.book.id }); }} />
            <MenuItem
              icon="check"
              label={menu.book.finished ? '标记未读完' : '标记已读完'}
              onClick={() => mutate(menu.book, menu.book.finished ? 'unfinish' : 'finish')}
            />
            <MenuItem
              icon="lock"
              label={menu.book.secret ? '取消私密' : '设为私密'}
              onClick={() => mutate(menu.book, menu.book.secret ? 'public' : 'secret')}
            />
            <MenuItem icon="close" label="移出书架" danger onClick={() => mutate(menu.book, 'remove')} />
          </div>
        </>
      ) : null}
    </>
  );
}

function MenuItem({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: 'book' | 'check' | 'lock' | 'close';
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 9,
        width: '100%',
        padding: '7px 9px',
        borderRadius: 6,
        fontSize: 12.5,
        color: danger ? 'var(--danger)' : 'var(--ink)',
        textAlign: 'left',
      }}
      onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(255,255,255,0.06)')}
      onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
    >
      <Icon name={icon} size={14} /> {label}
    </button>
  );
}
