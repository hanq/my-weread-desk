import { useEffect, useMemo, useRef, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import type { IconName } from '../icons';
import {
  ShelfHeader,
  UNGROUPED,
  naturalDirection,
} from '../components/ShelfHeader';
import type { ShelfFilter, ShelfLayout, ShelfSort, SortDir } from '../components/ShelfHeader';
import { navigate, store, toast, useApp } from '../store';
import { canTraverse, firstReadableChapter } from '../bookKind';
import { coverUrl, handleCoverError } from '../bookCover';
import { useShelfMotif } from '../skins/shelfMotifs';
import type { Book } from '../types';

const LAYOUT_KEY = 'my-weread-desk:shelf-layout';
const SORT_KEY = 'my-weread-desk:shelf-sort';
const DIR_KEY = 'my-weread-desk:shelf-sort-dir';

export function ShelfView() {
  const app = useApp();
  const [filter, setFilter] = useState<ShelfFilter>('all');
  const [layout, setLayout] = useState<ShelfLayout>(readLayout);
  const [sort, setSort] = useState<ShelfSort>(readSort);
  const [dir, setDir] = useState<SortDir>(() => readDir(readSort()));
  const [query, setQuery] = useState('');
  const motif = useShelfMotif();
  const [menu, setMenu] = useState<{ book: Book; x: number; y: number } | null>(null);
  const [menuPickingGroup, setMenuPickingGroup] = useState(false);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const openingRef = useRef<string | null>(null);
  // '' = all books, UNGROUPED = books without a folder, otherwise a folder id.
  const [folderId, setFolderId] = useState('');
  const [manager, setManager] = useState(false);
  const [folderMenu, setFolderMenu] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState('');
  const [editingGroup, setEditingGroup] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState('');
  // Enter and blur both fire on an inline editor; the guard keeps one save.
  const submittingRef = useRef(false);

  useEffect(() => {
    writeStored(LAYOUT_KEY, layout);
  }, [layout]);

  useEffect(() => {
    writeStored(SORT_KEY, sort);
    writeStored(DIR_KEY, dir);
  }, [sort, dir]);

  const books = useMemo(() => {
    if (manager) return [];
    let list = app.books;
    if (filter === 'finished') list = list.filter((b) => b.finished);
    if (filter === 'reading') list = list.filter((b) => !b.finished && b.lastRead > 0);
    if (filter === 'mp') list = list.filter((b) => b.isMp);
    if (folderId === UNGROUPED) list = list.filter((b) => !b.groupId);
    else if (folderId) list = list.filter((b) => b.groupId === folderId);

    const q = query.trim().toLowerCase();
    if (q) {
      list = list.filter((b) =>
        [b.title, b.author, b.publisher ?? '', b.category ?? ''].some((field) =>
          field.toLowerCase().includes(q),
        ),
      );
    }

    // The server returns the shelf in its own order, so sort it locally.
    // `lastRead` is the last reading time, `updateAt` the shelf entry time.
    const factor = dir === 'asc' ? 1 : -1;
    const sorted = [...list];
    if (sort === 'recent') {
      sorted.sort(
        (a, b) =>
          factor * (a.lastRead - b.lastRead) ||
          factor * (a.updateAt - b.updateAt) ||
          a.title.localeCompare(b.title, 'zh-CN'),
      );
    } else if (sort === 'added') {
      sorted.sort(
        (a, b) => factor * (a.updateAt - b.updateAt) || a.title.localeCompare(b.title, 'zh-CN'),
      );
    } else {
      sorted.sort((a, b) => factor * a.title.localeCompare(b.title, 'zh-CN'));
    }
    return sorted;
  }, [app.books, dir, filter, folderId, manager, query, sort]);

  // Folder cards summarise membership from the live book list so the counts and
  // the latest read time stay correct after a book is moved.
  const folders = useMemo(() => {
    const counts = new Map<string, number>();
    const latest = new Map<string, number>();
    for (const b of app.books) {
      if (!b.groupId) continue;
      counts.set(b.groupId, (counts.get(b.groupId) ?? 0) + 1);
      latest.set(b.groupId, Math.max(latest.get(b.groupId) ?? 0, b.lastRead || 0));
    }
    const q = query.trim().toLowerCase();
    return app.groups
      .filter((g) => !q || g.name.toLowerCase().includes(q))
      .map((g) => ({ ...g, count: counts.get(g.id) ?? 0, latest: latest.get(g.id) ?? 0 }))
      .sort((a, b) => b.latest - a.latest || a.name.localeCompare(b.name, 'zh-CN'));
  }, [app.books, app.groups, query]);

  const ungroupedCount = useMemo(() => app.books.filter((b) => !b.groupId).length, [app.books]);

  const folderName =
    folderId === UNGROUPED ? '未分组' : app.groups.find((g) => g.id === folderId)?.name ?? '';

  const chooseFolder = (value: string) => {
    setFolderId(value);
    setManager(false);
    setFolderMenu(null);
    setEditingGroup(null);
  };

  const chooseSort = (value: ShelfSort) => {
    if (value !== sort) setDir(naturalDirection(value));
    setSort(value);
  };

  const open = async (book: Book) => {
    if (book.isAudio) {
      // Audiobook albums have tracks, not chapters. Opening the text reader
      // would fail with a confusing "no chapters" error.
      toast('有声书暂不支持在应用内播放', 'info');
      return;
    }
    if (book.isMp) {
      navigate({ name: 'mp', bookId: book.id });
      return;
    }
    if (openingRef.current) return;
    openingRef.current = book.id;
    setOpeningId(book.id);
    try {
      const detail = await api.bookDetail(book.id);
      const chapterUid = detail.progress?.chapterUid || firstReadableChapter(detail.chapters)?.uid || 0;
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

  const cardEvents = (book: Book) => ({
    onClick: () => void open(book),
    onContextMenu: (e: ReactMouseEvent) => {
      e.preventDefault();
      setMenuPickingGroup(false);
      setMenu({ book, x: e.clientX, y: e.clientY });
    },
    'aria-label': `${book.title}${book.author ? ` · ${book.author}` : ''}`,
    'aria-busy': openingId === book.id,
    title: `${book.title}${book.author ? ` · ${book.author}` : ''}｜点击继续阅读，右键更多操作`,
  });

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
    const [nextBooks, groups] = await Promise.all([api.books(), api.groups()]);
    store.set({ books: nextBooks, groups });
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
      if (folderId === groupId) setFolderId('');
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
    <div className="shelf-scope" data-motif={motif}>
      <ShelfHeader
        query={query}
        onQuery={setQuery}
        filter={filter}
        onFilter={setFilter}
        folders={folders}
        ungroupedCount={ungroupedCount}
        totalCount={app.books.length}
        resultCount={books.length}
        folderId={folderId}
        onFolder={chooseFolder}
        manager={manager}
        onManageFolders={() => {
          setManager(true);
          setFolderMenu(null);
          setEditingGroup(null);
          setQuery('');
        }}
        onExitManager={() => {
          setManager(false);
          setQuery('');
        }}
        sort={sort}
        onSort={chooseSort}
        dir={dir}
        onDir={setDir}
        layout={layout}
        onLayout={setLayout}
        onOpenStore={() => navigate({ name: 'search' })}
      />

      <div className="page-body shelf-body">
        {manager ? (
          <div className="shelf-folder-panel">
            <p className="shelf-folder-note">
              文件夹会同步到微信读书。创建后在书籍上右键「移动到分组」即可归档。
            </p>
            <div className="folder-grid shelf-folder-grid">
              {!query.trim() ? (
                <div
                  className="folder-card"
                  role="button"
                  tabIndex={0}
                  onClick={() => chooseFolder(UNGROUPED)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') chooseFolder(UNGROUPED);
                  }}
                >
                  <div className="folder-head">
                    <Icon name="folder" size={16} />
                    <span className="folder-name">未分组</span>
                  </div>
                  <div className="folder-meta">{ungroupedCount} 本</div>
                </div>
              ) : null}

              {folders.map((g) => (
                <div
                  key={g.id}
                  className="folder-card"
                  role="button"
                  tabIndex={0}
                  onClick={() => {
                    if (folderMenu) {
                      setFolderMenu(null);
                      return;
                    }
                    chooseFolder(g.id);
                  }}
                  onDoubleClick={() => {
                    setEditingGroup(g.id);
                    setEditDraft(g.name);
                  }}
                  onContextMenu={(e) => {
                    e.preventDefault();
                    void removeGroup(g.id);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') chooseFolder(g.id);
                  }}
                >
                  <div className="folder-head">
                    <Icon name="folder" size={16} />
                    {editingGroup === g.id ? (
                      <input
                        className="group-input folder-rename"
                        autoFocus
                        value={editDraft}
                        onChange={(e) => setEditDraft(e.target.value)}
                        onClick={(e) => e.stopPropagation()}
                        onBlur={() => void commitRename(g.id)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') void commitRename(g.id);
                          if (e.key === 'Escape') setEditingGroup(null);
                        }}
                      />
                    ) : (
                      <span className="folder-name truncate">{g.name}</span>
                    )}
                  </div>
                  <div className="folder-meta">
                    {g.count} 本{g.latest ? ` · ${formatReadDate(g.latest)}` : ''}
                  </div>
                  <button
                    type="button"
                    className="folder-more"
                    title="文件夹操作"
                    onClick={(e) => {
                      e.stopPropagation();
                      setFolderMenu(folderMenu === g.id ? null : g.id);
                    }}
                  >
                    <Icon name="more" size={14} />
                  </button>
                  {folderMenu === g.id ? (
                    <div className="folder-menu" onClick={(e) => e.stopPropagation()}>
                      <button
                        type="button"
                        onClick={() => {
                          setFolderMenu(null);
                          setEditingGroup(g.id);
                          setEditDraft(g.name);
                        }}
                      >
                        重命名
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setFolderMenu(null);
                          void removeGroup(g.id);
                        }}
                      >
                        删除
                      </button>
                    </div>
                  ) : null}
                </div>
              ))}

              {creating ? (
                <div className="folder-card folder-new">
                  <input
                    className="group-input"
                    autoFocus
                    value={draft}
                    placeholder="文件夹名称"
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
                </div>
              ) : (
                <button type="button" className="folder-card folder-new" onClick={() => setCreating(true)}>
                  <Icon name="plus" size={16} /> 新建文件夹
                </button>
              )}
            </div>
          </div>
        ) : app.shelfState === 'loading' && app.books.length === 0 ? (
          <div className="empty">
            <div className="spinner" />
            <span>正在同步书架…</span>
          </div>
        ) : books.length === 0 ? (
          <div className="empty">
            <Icon name="book" size={34} />
            <p>
              {query.trim()
                ? '没有匹配的书'
                : folderId
                  ? '这个文件夹还没有书籍'
                  : filter !== 'all'
                    ? '这个分类下暂时没有书'
                    : '书架还是空的，先同步一次吧'}
            </p>
            {query.trim() || folderId || filter !== 'all' ? (
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => {
                  setQuery('');
                  setFolderId('');
                  setFilter('all');
                }}
              >
                清除筛选条件
              </button>
            ) : (
              <button
                type="button"
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
            )}
          </div>
        ) : layout === 'list' ? (
          <div className="shelf-list">
            {books.map((b) => (
              <div
                key={b.id}
                className={`book-row${openingId === b.id ? ' opening' : ''}`}
                {...cardEvents(b)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    void open(b);
                  }
                }}
              >
                <div className="cover">
                  {coverUrl(b) ? (
                    <img
                      src={coverUrl(b)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      onError={(e) => handleCoverError(b, e)}
                    />
                  ) : null}
                </div>
                <div className="grow">
                  <div className="truncate shelf-row-title">{b.title}</div>
                  <div className="faint shelf-row-meta">
                    {b.author || (b.isMp ? '公众号' : '')}
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
        ) : motif === 'bamboo' ? (
          <div className="bamboo-grid">
            {books.map((b) => (
              <button
                key={b.id}
                type="button"
                className={`bamboo-slip${openingId === b.id ? ' opening' : ''}${b.finished ? ' finished' : ''}`}
                {...cardEvents(b)}
              >
                {coverUrl(b) ? (
                  <span className="slip-texture" aria-hidden="true">
                    <img
                      src={coverUrl(b)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      onError={(e) => handleCoverError(b, e)}
                    />
                  </span>
                ) : (
                  <span className="slip-watermark" aria-hidden="true">
                    {Array.from(b.title)[0] ?? '书'}
                  </span>
                )}
                {shelfMark(b) ? <span className="slip-seal">{shelfMark(b)}</span> : null}
                <span className="slip-cord slip-cord-top" aria-hidden="true" />
                <span className={slipTitleClass(b.title)}>{b.title}</span>
                <span className="slip-cord slip-cord-bottom" aria-hidden="true" />
                <span className="slip-author">{b.author || (b.isMp ? '公众号' : '')}</span>
                {openingId === b.id ? (
                  <span className="book-opening">
                    <span className="spinner" />
                  </span>
                ) : null}
              </button>
            ))}
          </div>
        ) : motif === 'jade' ? (
          <div className="jade-grid">
            {books.map((b) => (
              <button
                key={b.id}
                type="button"
                className={`jade-card${openingId === b.id ? ' opening' : ''}`}
                {...cardEvents(b)}
              >
                <span className="jade-cover">
                  {coverUrl(b) ? (
                    <img
                      src={coverUrl(b)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      onError={(e) => handleCoverError(b, e)}
                    />
                  ) : (
                    <span className="jade-blank">{Array.from(b.title)[0] ?? '书'}</span>
                  )}
                </span>
                <span className="jade-body">
                  <span className="jade-title">{b.title}</span>
                  <span className="jade-meta">
                    {b.author || (b.isMp ? '公众号' : '')}
                    {b.groupName ? ` · ${b.groupName}` : ''}
                  </span>
                  <span className="jade-tags">
                    {b.finished ? <span className="jade-tag seal">已读完</span> : null}
                    {b.isMp ? <span className="jade-tag">公众号</span> : null}
                    {b.isAudio ? <span className="jade-tag">有声</span> : null}
                    {b.secret ? <span className="jade-tag">私密</span> : null}
                  </span>
                </span>
                {openingId === b.id ? (
                  <span className="book-opening">
                    <span className="spinner" />
                  </span>
                ) : null}
              </button>
            ))}
          </div>
        ) : motif === 'orb' ? (
          <div className="orb-grid">
            {books.map((b) => (
              <button
                key={b.id}
                type="button"
                className={`orb-tile${openingId === b.id ? ' opening' : ''}`}
                {...cardEvents(b)}
              >
                <span className="orb-cover">
                  {coverUrl(b) ? (
                    <img
                      src={coverUrl(b)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      onError={(e) => handleCoverError(b, e)}
                    />
                  ) : (
                    <span className="orb-blank">{Array.from(b.title)[0] ?? '书'}</span>
                  )}
                  {openingId === b.id ? (
                    <span className="book-opening">
                      <span className="spinner" />
                    </span>
                  ) : null}
                </span>
                <span className="orb-title">{b.title}</span>
                <span className="orb-author">
                  {shelfMark(b) ? `${shelfMark(b)} · ` : ''}
                  {b.author || (b.isMp ? '公众号' : '')}
                </span>
              </button>
            ))}
          </div>
        ) : (
          <div className="shelf-grid">
            {books.map((b) => (
              <button
                key={b.id}
                type="button"
                className={`book-card${openingId === b.id ? ' opening' : ''}`}
                {...cardEvents(b)}
              >
                <div className="cover">
                  {coverUrl(b) ? (
                    <img
                      src={coverUrl(b)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      onError={(e) => handleCoverError(b, e)}
                    />
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
        )}
      </div>

      {menu ? (
        <>
          <div
            className="book-menu-backdrop"
            onClick={() => {
              setMenu(null);
              setMenuPickingGroup(false);
            }}
            onContextMenu={(e) => e.preventDefault()}
          />
          <div
            className="book-menu"
            style={{
              left: Math.min(menu.x, window.innerWidth - 196),
              top: Math.min(menu.y, window.innerHeight - 320),
            }}
          >
            {menuPickingGroup ? (
              <>
                <button type="button" className="book-menu-back" onClick={() => setMenuPickingGroup(false)}>
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
                {!menu.book.isAudio && !menu.book.isMp ? (
                  <>
                    <div className="book-menu-group">智能</div>
                    <MenuItem
                      icon="sparkle"
                      label="问读书助手"
                      onClick={() => {
                        setMenu(null);
                        navigate({ name: 'assistant', bookId: menu.book.id });
                      }}
                    />
                    {canTraverse(menu.book.kind) ? (
                      <MenuItem
                        icon="clock"
                        label="故事穿越"
                        onClick={() => {
                          setMenu(null);
                          navigate({ name: 'traversal', bookId: menu.book.id });
                        }}
                      />
                    ) : null}
                  </>
                ) : null}
                <div className="book-menu-sep" />
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

    </div>
  );
}

function MenuItem({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: IconName;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button type="button" className={`book-menu-item${danger ? ' danger' : ''}`} onClick={onClick}>
      <Icon name={icon} size={14} /> {label}
    </button>
  );
}

// Bamboo titles are vertical. writing-mode: vertical-rl wraps long text into
// another column to the left, and the default 1.4em block clips that column.
// The thresholds match the narrowest carrier sizes: about 8 units fit per
// column at 560px, so 1/2/3/4 columns need roughly 8/17/25 units. Count CJK as
// one unit and Latin as roughly half so the class matches the rendered width.
function slipTitleClass(title: string): string {
  let units = 0;
  for (const ch of title) {
    if (/\s/.test(ch)) units += 0.35;
    else if (/[\u2e80-\u9fff\uf900-\ufaff\uff00-\uffef]/.test(ch)) units += 1;
    else units += 0.55;
  }
  if (units > 25) return 'slip-title xxlong';
  if (units > 17) return 'slip-title xlong';
  if (units > 8) return 'slip-title long';
  return 'slip-title';
}

/** Marks a book on the compact carriers without using emoji. */
function shelfMark(book: Book): string {
  if (book.finished) return '毕';
  if (book.isMp) return '公';
  if (book.isAudio) return '声';
  if (book.secret) return '密';
  return '';
}

function readLayout(): ShelfLayout {
  return readStored(LAYOUT_KEY) === 'list' ? 'list' : 'grid';
}

function readSort(): ShelfSort {
  const raw = readStored(SORT_KEY);
  return raw === 'added' || raw === 'title' ? raw : 'recent';
}

function readDir(fallbackKey: ShelfSort): SortDir {
  const raw = readStored(DIR_KEY);
  return raw === 'asc' || raw === 'desc' ? raw : naturalDirection(fallbackKey);
}

function readStored(key: string): string {
  try {
    return window.localStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

function writeStored(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    /* Storage can be disabled; the view still works for this session. */
  }
}

/** Folder cards show the most recent read date of their books. */
function formatReadDate(ts: number): string {
  const d = new Date(ts * 1000);
  const now = new Date();
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}月${d.getDate()}日`;
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()}`;
}
