import { useMemo, useRef, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { Book } from '../types';

type Filter = 'all' | 'reading' | 'finished' | 'mp';
type Layout = 'grid' | 'list';
type Sort = 'recent' | 'added' | 'title';

export function ShelfView() {
  const app = useApp();
  const [filter, setFilter] = useState<Filter>('all');
  const [layout, setLayout] = useState<Layout>('grid');
  const [sort, setSort] = useState<Sort>('recent');
  const [query, setQuery] = useState('');
  const [menu, setMenu] = useState<{ book: Book; x: number; y: number } | null>(null);
  const [menuPickingGroup, setMenuPickingGroup] = useState(false);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const openingRef = useRef<string | null>(null);
  // 'all' | 'ungrouped' | a folder id.
  const [groupFilter, setGroupFilter] = useState('all');
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState('');
  const [editingGroup, setEditingGroup] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState('');
  // Enter and blur both fire on an inline editor; the guard keeps one save.
  const submittingRef = useRef(false);

  const books = useMemo(() => {
    let list = app.books;
    if (filter === 'finished') list = list.filter((b) => b.finished);
    if (filter === 'reading') list = list.filter((b) => !b.finished && b.lastRead > 0);
    if (filter === 'mp') list = list.filter((b) => b.isMp);
    if (groupFilter === 'ungrouped') list = list.filter((b) => !b.groupId);
    else if (groupFilter !== 'all') list = list.filter((b) => b.groupId === groupFilter);
    const q = query.trim().toLowerCase();
    if (q) list = list.filter((b) => b.title.toLowerCase().includes(q) || b.author.toLowerCase().includes(q));

    // The server returns the shelf in its own order, so sort it locally.
    // `lastRead` is the last reading time, `updateAt` the shelf entry time.
    const sorted = [...list];
    if (sort === 'recent') {
      sorted.sort(
        (a, b) => b.lastRead - a.lastRead || b.updateAt - a.updateAt || a.title.localeCompare(b.title, 'zh-CN'),
      );
    } else if (sort === 'added') {
      sorted.sort((a, b) => b.updateAt - a.updateAt || a.title.localeCompare(b.title, 'zh-CN'));
    } else {
      sorted.sort((a, b) => a.title.localeCompare(b.title, 'zh-CN'));
    }
    return sorted;
  }, [app.books, filter, groupFilter, query, sort]);

  const open = async (book: Book) => {
    if (book.isAudio) {
      // Audiobook albums have tracks, not chapters. Opening the text reader
      // would fail with a confusing "no chapters" error.
      toast('有声书暂不支持在应用内播放', 'info');
      return;
    }
    if (book.isMp) {
      navigate({ name: 'mp' });
      return;
    }
    if (openingRef.current) return;
    openingRef.current = book.id;
    setOpeningId(book.id);
    try {
      const detail = await api.bookDetail(book.id);
      const chapterUid = detail.progress?.chapterUid || detail.chapters[0]?.uid || 0;
      if (chapterUid > 0) {
        navigate({ name: 'reader', bookId: book.id, chapterUid });
      } else {
        navigate({ name: 'detail', bookId: book.id });
      }
    } catch {
      // Offline or a failed detail lookup should still leave a way into the
      // book; the detail page owns the retry and download controls.
      navigate({ name: 'detail', bookId: book.id });
    } finally {
      openingRef.current = null;
      setOpeningId(null);
    }
  };

  const mutate = (book: Book, action: string) => {
    setMenu(null);
    setMenuPickingGroup(false);
    api
      .mutateShelf(book.id, action)
      .then(() => api.books())
      .then((list) => {
        store.set({ books: list });
        toast(action === 'remove' ? '已移出书架' : '已更新', 'success');
      })
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
  };

  const refreshLibrary = async () => {
    const [books, groups] = await Promise.all([api.books(), api.groups()]);
    store.set({ books, groups });
  };

  const reportError = (e: unknown) => toast(String((e as Error).message || e), 'error');

  const submitGroup = async () => {
    if (submittingRef.current) return;
    const name = draft.trim();
    if (!name) {
      setCreating(false);
      return;
    }
    submittingRef.current = true;
    try {
      await api.createGroup(name);
      setCreating(false);
      setDraft('');
      await refreshLibrary();
      toast('已创建分组', 'success');
    } catch (e) {
      reportError(e);
    } finally {
      submittingRef.current = false;
    }
  };

  const commitRename = async (groupId: string) => {
    if (submittingRef.current) return;
    const name = editDraft.trim();
    const target = app.groups.find((g) => g.id === groupId);
    if (!name || !target || name === target.name) {
      setEditingGroup(null);
      return;
    }
    submittingRef.current = true;
    try {
      await api.renameGroup(groupId, name);
      setEditingGroup(null);
      await refreshLibrary();
    } catch (e) {
      reportError(e);
    } finally {
      submittingRef.current = false;
    }
  };

  const removeGroup = async (groupId: string) => {
    if (!window.confirm('删除这个分组？书籍不会被移出书架。')) return;
    try {
      await api.deleteGroup(groupId);
      if (groupFilter === groupId) setGroupFilter('all');
      await refreshLibrary();
      toast('已删除分组', 'success');
    } catch (e) {
      reportError(e);
    }
  };

  const moveToGroup = async (book: Book, groupId: string) => {
    setMenu(null);
    setMenuPickingGroup(false);
    try {
      await api.setBookGroup([book.id], groupId);
      await refreshLibrary();
      toast(groupId ? '已移动到分组' : '已取消分组', 'success');
    } catch (e) {
      reportError(e);
    }
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

      <div className="shelf-groups">
        <button className={`group-chip${groupFilter === 'all' ? ' on' : ''}`} onClick={() => setGroupFilter('all')}>
          <Icon name="folder" size={14} /> 全部
        </button>
        {app.groups.map((g) =>
          editingGroup === g.id ? (
            <input
              key={g.id}
              className="group-input"
              autoFocus
              value={editDraft}
              onChange={(e) => setEditDraft(e.target.value)}
              onBlur={() => void commitRename(g.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void commitRename(g.id);
                if (e.key === 'Escape') setEditingGroup(null);
              }}
            />
          ) : (
            <button
              key={g.id}
              className={`group-chip${groupFilter === g.id ? ' on' : ''}`}
              onClick={() => setGroupFilter(g.id)}
              onDoubleClick={() => {
                setEditingGroup(g.id);
                setEditDraft(g.name);
              }}
              onContextMenu={(e) => {
                e.preventDefault();
                void removeGroup(g.id);
              }}
              title="双击重命名，右键删除"
            >
              <Icon name="folder" size={14} /> {g.name}
              <span className="count">{g.bookIds?.length ?? 0}</span>
            </button>
          ),
        )}
        <button
          className={`group-chip${groupFilter === 'ungrouped' ? ' on' : ''}`}
          onClick={() => setGroupFilter('ungrouped')}
        >
          未分组
        </button>
        {creating ? (
          <input
            className="group-input"
            autoFocus
            value={draft}
            placeholder="分组名称"
            onChange={(e) => setDraft(e.target.value)}
            onBlur={() => void submitGroup()}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void submitGroup();
              if (e.key === 'Escape') {
                setCreating(false);
                setDraft('');
              }
            }}
          />
        ) : (
          <button className="group-chip ghost" onClick={() => setCreating(true)} title="新建分组">
            <Icon name="plus" size={14} /> 新建
          </button>
        )}
      </div>

      <div className="shelf-toolbar">
        <div className="input shelf-search">
          <Icon name="search" size={14} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="在书架中筛选"
            style={{ border: 0, background: 'transparent', outline: 'none', width: '100%' }}
          />
        </div>
        <div className="grow" />
        <div className="seg">
          {(
            [
              ['recent', '最近阅读'],
              ['added', '加入书架'],
              ['title', '书名'],
            ] as [Sort, string][]
          ).map(([k, label]) => (
            <button key={k} className={sort === k ? 'on' : ''} onClick={() => setSort(k)}>
              {label}
            </button>
          ))}
        </div>
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
                className={`book-card${openingId === b.id ? ' opening' : ''}`}
                onClick={() => void open(b)}
                title="点击继续阅读；右键打开更多操作"
                aria-busy={openingId === b.id}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setMenuPickingGroup(false);
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
                  {b.isAudio ? <span className="audio-badge">有声</span> : null}
                  {openingId === b.id ? (
                    <span className="book-opening">
                      <span className="spinner" />
                    </span>
                  ) : null}
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
                className={`book-row${openingId === b.id ? ' opening' : ''}`}
                onClick={() => void open(b)}
                title="点击继续阅读；右键打开更多操作"
                aria-busy={openingId === b.id}
                onContextMenu={(e) => {
                  e.preventDefault();
                  setMenuPickingGroup(false);
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
                    {b.groupName ? ` · ${b.groupName}` : ''}
                    {b.category ? ` · ${b.category}` : ''}
                    {b.isAudio ? ' · 有声' : ''}
                  </div>
                </div>
                {openingId === b.id ? <span className="spinner" /> : null}
                {b.finished ? <span className="chip">已读完</span> : null}
                {b.secret ? <span className="chip">私密</span> : null}
                {b.isMp ? <span className="chip accent">公众号</span> : null}
                {b.isAudio ? <span className="chip">有声</span> : null}
              </div>
            ))}
          </div>
        )}
      </div>

      {menu ? (
        <>
          <div style={{ position: 'fixed', inset: 0, zIndex: 40 }} onClick={() => { setMenu(null); setMenuPickingGroup(false); }} onContextMenu={(e) => e.preventDefault()} />
          <div
            style={{
              position: 'fixed',
              left: Math.min(menu.x, window.innerWidth - 190),
              top: Math.min(menu.y, window.innerHeight - 200),
              zIndex: 41,
              width: 178,
              maxHeight: 'min(420px, 70vh)',
              overflowY: 'auto',
              padding: 5,
              borderRadius: 9,
              background: '#232838',
              border: '1px solid rgba(255,255,255,0.1)',
              boxShadow: 'var(--shadow-2)',
            }}
          >
            {menuPickingGroup ? (
              <>
                <button className="menu-back" onClick={() => setMenuPickingGroup(false)}>
                  <Icon name="left" size={13} /> 返回
                </button>
                <MenuItem icon="folder" label="不分组" onClick={() => void moveToGroup(menu.book, '')} />
                {app.groups.map((g) => (
                  <MenuItem
                    key={g.id}
                    icon="folder"
                    label={g.name}
                    onClick={() => void moveToGroup(menu.book, g.id)}
                  />
                ))}
                {app.groups.length === 0 ? <div className="menu-empty">还没有分组</div> : null}
              </>
            ) : (
              <>
                <MenuItem icon="book" label="继续阅读" onClick={() => { setMenu(null); void open(menu.book); }} />
                <MenuItem icon="book" label="打开详情" onClick={() => { setMenu(null); navigate({ name: 'detail', bookId: menu.book.id }); }} />
                <MenuItem icon="folder" label="移动到分组" onClick={() => setMenuPickingGroup(true)} />
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
              </>
            )}
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
  icon: 'book' | 'check' | 'lock' | 'close' | 'folder';
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
