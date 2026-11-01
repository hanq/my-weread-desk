import { useEffect, useMemo, useRef, useState } from 'react';

import { api, events } from '../api';
import { coverUrl, handleCoverError } from '../bookCover';
import { bookKindLabel } from '../bookKind';
import { Icon } from '../icons';
import type { IconName } from '../icons';
import { navigate, toast, useApp } from '../store';
import type { Book, DownloadState, SmartBook } from '../types';

interface QuickState {
  bookId: string;
  title: string;
  stage: string;
  detail?: string;
  error?: string;
}

interface SmartAction {
  key: string;
  label: string;
  hint: string;
  icon: IconName;
  status: string;
  primary?: boolean;
  disabled?: boolean;
  onClick: () => void;
}

function stageLabel(stage: { status: string }, ready = '已就绪', missing = '未建立'): string {
  switch (stage.status) {
    case 'ready':
      return ready;
    case 'running':
      return '进行中';
    case 'failed':
      return '失败';
    case 'stale':
      return '需重建';
    case 'partial':
      return '部分完成';
    default:
      return missing;
  }
}

function Cover({ book, className }: { book: Book; className: string }) {
  const src = coverUrl(book);
  return (
    <span className={className}>
      {src ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={(e) => handleCoverError(book, e)}
        />
      ) : (
        <span className="smart-cover-letter">{Array.from(book.title)[0] ?? '书'}</span>
      )}
    </span>
  );
}

export function SmartView({ initialBookId }: { initialBookId?: string } = {}) {
  const app = useApp();
  const [books, setBooks] = useState<SmartBook[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [picker, setPicker] = useState(false);
  const [pickerQuery, setPickerQuery] = useState('');
  const [quick, setQuick] = useState<QuickState | null>(null);
  const cardRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const runningRef = useRef(false);

  const load = async (prefer?: string) => {
    setLoading(true);
    try {
      const list = await api.smartBooks();
      setBooks(list);
      setError('');
      const wanted = prefer || initialBookId || app.settings?.lastBookId;
      const pick = list.find((item) => item.book.id === wanted) ?? list[0];
      setSelectedId(pick?.book.id ?? '');
    } catch (e) {
      setError(String((e as Error).message || e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load(initialBookId);
    // The shelf is reloaded explicitly after indexing; this effect only handles
    // entering the route with an optional book.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialBookId]);

  const selected = useMemo(
    () => books.find((item) => item.book.id === selectedId) ?? books[0],
    [books, selectedId],
  );

  useEffect(() => {
    if (!selected) return;
    const node = cardRefs.current[selected.book.id];
    const behavior: ScrollBehavior = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
      ? 'auto'
      : 'smooth';
    node?.scrollIntoView({ inline: 'center', block: 'nearest', behavior });
  }, [selected?.book.id]);

  useEffect(() => {
    if (!picker) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPicker(false);
        setPickerQuery('');
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [picker]);

  const smartIds = useMemo(() => new Set(books.map((item) => item.book.id)), [books]);

  const candidates = useMemo(() => {
    const query = pickerQuery.trim().toLowerCase();
    return app.books
      .filter((book) => !book.isAudio && !book.isMp)
      .filter(
        (book) =>
          !query ||
          book.title.toLowerCase().includes(query) ||
          (book.author ?? '').toLowerCase().includes(query),
      )
      .sort((a, b) => {
        const aa = smartIds.has(a.id) ? 1 : 0;
        const bb = smartIds.has(b.id) ? 1 : 0;
        return aa - bb || a.title.localeCompare(b.title);
      });
  }, [app.books, pickerQuery, smartIds]);

  const waitForDownload = (bookId: string, title: string, initial?: DownloadState) =>
    new Promise<boolean>((resolve) => {
      let settled = false;
      let off = () => {};
      const finish = (ok: boolean, message?: string) => {
        if (settled) return;
        settled = true;
        off();
        if (!ok) {
          setQuick({
            bookId,
            title,
            stage: '下载没有完成',
            error: message || '没有下载到章节正文',
          });
        }
        resolve(ok);
      };
      const update = (state: DownloadState) => {
        if (state.bookId !== bookId || settled) return;
        const progress = state.total > 0 ? `${state.done}/${state.total} 章` : state.message || '准备中';
        setQuick({
          bookId,
          title,
          stage: state.running ? '正在下载书籍…' : '下载完成',
          detail: state.current ? `${progress} · ${state.current}` : progress,
        });
        if (!state.running) {
          if (state.done > 0) finish(true);
          else finish(false, state.errors?.[0] || state.message || '没有下载到章节正文');
        }
      };
      // Subscribe before starting so a fast failure cannot race past us.
      off = events.on('download', (state: DownloadState) => update(state));
      if (initial) update(initial);
      void api.startDownload(bookId).then(update).catch((e) => {
        finish(false, String((e as Error).message || e));
      });
    });

  const ensureChapters = async (bookId: string, title: string): Promise<boolean> => {
    let cached = 0;
    try {
      const coverage = await api.cachedChapterCount(bookId);
      cached = coverage.cached;
    } catch {
      cached = 0;
    }
    if (cached > 0) return true;
    setQuick({ bookId, title, stage: '正在下载书籍…', detail: '正在获取章节目录…' });
    return waitForDownload(bookId, title);
  };

  const runIndex = async (bookId: string, title: string): Promise<boolean> => {
    if (runningRef.current) return false;
    runningRef.current = true;
    setQuick({ bookId, title, stage: '正在准备书籍…', detail: '检查本地缓存' });
    try {
      if (!(await ensureChapters(bookId, title))) return false;
      setQuick({ bookId, title, stage: '正在建立文本索引…', detail: '索引只读取已缓存章节' });
      await api.indexBook(bookId);
      await load(bookId);
      setQuick(null);
      return true;
    } catch (e) {
      setQuick({
        bookId,
        title,
        stage: '索引没有完成',
        error: String((e as Error).message || e),
      });
      return false;
    } finally {
      runningRef.current = false;
    }
  };

  const ensureText = async (book: SmartBook, next: () => void) => {
    if (book.text.status === 'ready') {
      next();
      return;
    }
    if (await runIndex(book.book.id, book.book.title)) next();
  };

  const addBook = async (book: Book) => {
    setPicker(false);
    setPickerQuery('');
    if (smartIds.has(book.id)) return;
    try {
      const status = await api.aiStatus(book.id);
      if (status.text.status === 'ready') {
        await load(book.id);
        toast('已加入智能书架', 'success');
        return;
      }
    } catch {
      // Fall through to building the index; the overlay explains a real failure.
    }
    if (await runIndex(book.id, book.title)) {
      toast('已加入智能书架', 'success');
    }
  };

  let actions: SmartAction[] = [];
  if (selected) {
    const textReady = selected.text.status === 'ready';
    const scenesReady = selected.scenes.status === 'ready';
    const worldReady = selected.world.status === 'ready';
    const askStatus = textReady ? '已就绪' : '需快速索引';
    const traverseStatus = textReady ? (scenesReady ? '已就绪' : '需剧情索引') : '需快速索引';
    const worldStatus = textReady
      ? scenesReady
        ? worldReady
          ? '已就绪'
          : '需世界设定'
        : '需剧情索引'
      : '需快速索引';

    actions = [
      {
        key: 'ask',
        label: '读书助手',
        hint: '原文问答、定位情节、引用回跳',
        icon: 'sparkle',
        status: askStatus,
        primary: true,
        onClick: () => {
          void ensureText(selected, () => navigate({ name: 'assistant', bookId: selected.book.id }));
        },
      },
    ];

    if (selected.kind === 'fiction') {
      actions.push(
        {
          key: 'immerse',
          label: '故事穿越',
          hint: '时间线、角色视角与沉浸体验',
          icon: 'clock',
          status: traverseStatus,
          onClick: () => {
            void ensureText(selected, () => {
              if (!selected.capabilities.immerse) {
                toast('进入后点击「建立时间线」完成剧情索引', 'info');
              }
              navigate({ name: 'traversal', bookId: selected.book.id });
            });
          },
        },
        {
          key: 'world',
          label: '平行世界',
          hint: '做出选择，生成另一条世界线',
          icon: 'columns',
          status: worldStatus,
          onClick: () => {
            void ensureText(selected, () => {
              if (!selected.capabilities.rewrite) {
                toast('进入后先建立时间线与世界表，再开始改写世界', 'info');
              }
              navigate({ name: 'traversal', bookId: selected.book.id });
            });
          },
        },
      );
    } else if (selected.kind === 'textbook') {
      actions.push({
        key: 'exam',
        label: '考试助手',
        hint: '自测卷、错题本与知识点复习',
        icon: 'file',
        status: '即将推出',
        disabled: true,
        onClick: () => {},
      });
    } else if (selected.kind === 'periodical') {
      actions.push({
        key: 'digest',
        label: '刊物速览',
        hint: '跨期议题追踪与文章摘要',
        icon: 'file',
        status: '即将推出',
        disabled: true,
        onClick: () => {},
      });
    } else if (selected.kind === 'medical') {
      actions.push({
        key: 'evidence',
        label: '循证问答',
        hint: '严格引用的医学检索（非诊断）',
        icon: 'file',
        status: '即将推出',
        disabled: true,
        onClick: () => {},
      });
    } else if (selected.kind === 'nonfiction') {
      actions.push({
        key: 'study',
        label: '研读工具',
        hint: '论点地图、概念卡与章节摘要',
        icon: 'file',
        status: '即将推出',
        disabled: true,
        onClick: () => {},
      });
    }
  }

  const closePicker = () => {
    setPicker(false);
    setPickerQuery('');
  };

  return (
    <>
      <div className="page-head smart-head">
        <div>
          <h1>智能书架</h1>
          <p className="sub">先选一本书，再选择怎么读它。</p>
        </div>
        <div className="grow" />
        <span className="smart-count">{books.length > 0 ? `${books.length} 本已准备` : '还没有准备的书'}</span>
        <button className="btn btn-primary" onClick={() => setPicker(true)}>
          <Icon name="plus" size={15} /> 添加书籍
        </button>
      </div>

      <div className="page-body smart-body">
        {loading ? (
          <div className="smart-skeleton" role="status" aria-label="正在加载智能书架">
            {Array.from({ length: 4 }).map((_, index) => (
              <span key={index} />
            ))}
            <span className="wide" />
          </div>
        ) : error ? (
          <div className="empty">
            <p>{error}</p>
            <button className="btn" onClick={() => void load(initialBookId)}>
              重试
            </button>
          </div>
        ) : books.length === 0 ? (
          <div className="empty smart-empty">
            <Icon name="sparkle" size={30} />
            <p>智能书架还是空的。</p>
            <p className="faint">
              添加一本书，App 会快速建立文本索引；之后可以问书、穿越或改写世界。
            </p>
            <button className="btn btn-primary" onClick={() => setPicker(true)}>
              添加第一本书
            </button>
          </div>
        ) : (
          <>
            <div className="smart-shelf-wrap">
              <div className="smart-shelf" role="list" aria-label="智能书籍">
                {books.map((item) => {
                  const active = selected?.book.id === item.book.id;
                  return (
                    <button
                      key={item.book.id}
                      ref={(node) => {
                        cardRefs.current[item.book.id] = node;
                      }}
                      type="button"
                      role="listitem"
                      className={`smart-card${active ? ' on' : ''}`}
                      onClick={() => setSelectedId(item.book.id)}
                      aria-pressed={active}
                      title={`${item.book.title}${item.book.author ? ` · ${item.book.author}` : ''}`}
                    >
                      <Cover book={item.book} className="smart-card-cover" />
                      <span className="smart-card-body">
                        <span className="smart-card-title">{item.book.title}</span>
                        <span className="smart-card-meta">
                          {bookKindLabel(item.kind)}
                          {item.branchCount > 0 ? ` · ${item.branchCount} 条世界线` : ''}
                        </span>
                        <span className="smart-card-badges" aria-hidden="true">
                          <span className={`smart-badge${item.text.status === 'ready' ? ' on' : ''}`}>文</span>
                          <span className={`smart-badge${item.scenes.status === 'ready' ? ' on' : ''}`}>剧</span>
                          <span className={`smart-badge${item.world.status === 'ready' ? ' on' : ''}`}>世</span>
                        </span>
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            {selected ? (
              <section className="smart-focus">
                <div className="smart-focus-main">
                  <Cover book={selected.book} className="smart-focus-cover" />
                  <div className="smart-focus-info">
                    <div className="smart-kicker">
                      {bookKindLabel(selected.kind)} ·{' '}
                      {selected.kindSource === 'user' ? '已确认类型' : '自动识别'}
                      {selected.book.author ? ` · ${selected.book.author}` : ''}
                    </div>
                    <h2>{selected.book.title}</h2>
                    <p className="faint">
                      {selected.book.category || '未分类'}
                      {selected.book.wordCount > 0
                        ? ` · ${Math.round(selected.book.wordCount / 10000)} 万字`
                        : ''}
                    </p>
                    <div className="smart-status-row">
                      <span className={`smart-status${selected.text.status === 'ready' ? ' on' : ''}`}>
                        文本 {stageLabel(selected.text)}
                      </span>
                      <span className={`smart-status${selected.scenes.status === 'ready' ? ' on' : ''}`}>
                        剧情 {stageLabel(selected.scenes)}
                      </span>
                      <span className={`smart-status${selected.world.status === 'ready' ? ' on' : ''}`}>
                        世界 {stageLabel(selected.world)}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="smart-actions">
                  {actions.map((action) => (
                    <button
                      key={action.key}
                      type="button"
                      className={`smart-action${action.primary ? ' primary' : ''}`}
                      disabled={action.disabled}
                      onClick={action.onClick}
                    >
                      <span className="smart-action-icon">
                        <Icon name={action.icon} size={18} />
                      </span>
                      <span className="smart-action-copy">
                        <span className="smart-action-label">{action.label}</span>
                        <span className="smart-action-hint">{action.hint}</span>
                      </span>
                      <span className={`smart-action-status${action.status === '已就绪' ? ' on' : ''}`}>
                        {action.status}
                      </span>
                    </button>
                  ))}
                </div>
              </section>
            ) : null}
          </>
        )}
      </div>

      {picker ? (
        <>
          <div className="smart-picker-backdrop" onClick={closePicker} />
          <aside className="smart-picker" role="dialog" aria-modal="true" aria-label="加入智能书架">
            <header className="smart-picker-head">
              <div>
                <h2>加入智能书架</h2>
                <p>选择一本书，App 会快速建立文本索引。</p>
              </div>
              <button className="icon-btn" onClick={closePicker} aria-label="关闭">
                <Icon name="close" size={16} />
              </button>
            </header>
            <label className="smart-picker-search">
              <Icon name="search" size={15} />
              <input
                autoFocus
                value={pickerQuery}
                onChange={(e) => setPickerQuery(e.target.value)}
                placeholder="搜索书名或作者"
              />
            </label>
            <div className="smart-picker-list">
              {candidates.length === 0 ? (
                <div className="empty" style={{ height: 160 }}>
                  没有匹配的书
                </div>
              ) : (
                candidates.map((book) => {
                  const added = smartIds.has(book.id);
                  return (
                    <button
                      key={book.id}
                      type="button"
                      className="smart-picker-item"
                      disabled={added}
                      onClick={() => void addBook(book)}
                    >
                      <Cover book={book} className="smart-picker-cover" />
                      <span className="smart-picker-copy">
                        <span className="smart-picker-title">{book.title}</span>
                        <span className="smart-picker-meta">
                          {book.author || '佚名'} · {bookKindLabel(book.kind)}
                        </span>
                      </span>
                      <span className={`smart-picker-add${added ? ' added' : ''}`}>
                        {added ? '已加入' : '加入'}
                      </span>
                    </button>
                  );
                })
              )}
            </div>
          </aside>
        </>
      ) : null}

      {quick ? (
        <div className="smart-loading-backdrop" role="alertdialog" aria-modal="true" aria-label="正在加载书籍">
          <div className={`smart-loading${quick.error ? ' error' : ''}`}>
            {quick.error ? (
              <span className="smart-loading-error" aria-hidden="true">
                <Icon name="close" size={22} />
              </span>
            ) : (
              <span className="smart-loading-ring" aria-hidden="true" />
            )}
            <div className="smart-loading-title">
              {quick.error ? '加载失败' : '正在快速加载书籍内容'}
            </div>
            <div className="smart-loading-book">{quick.title}</div>
            <div className="smart-loading-stage">{quick.stage}</div>
            {quick.detail && !quick.error ? (
              <div className="smart-loading-detail">{quick.detail}</div>
            ) : null}
            {quick.error ? (
              <>
                <div className="smart-loading-error-text">{quick.error}</div>
                <div className="smart-loading-actions">
                  <button className="btn" onClick={() => void runIndex(quick.bookId, quick.title)}>
                    重试
                  </button>
                  <button className="btn btn-ghost" onClick={() => setQuick(null)}>
                    关闭
                  </button>
                </div>
              </>
            ) : (
              <div className="smart-loading-hint">
                {quick.stage.startsWith('正在下载')
                  ? '下载完成后会自动建立索引'
                  : '建立文本索引，通常只需要几秒'}
              </div>
            )}
          </div>
        </div>
      ) : null}
    </>
  );
}
