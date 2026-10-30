import { useEffect, useMemo, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { toast, useApp } from '../store';
import { BlockView, measureBlocks } from '../reader/BlockView';
import { fontStack } from '../reader/themeVars';
import type { Book, ChapterContent, MPArticle } from '../types';

export function MPView() {
  const app = useApp();
  const theme = app.settings?.theme;
  const sources = useMemo(() => app.books.filter((b) => b.isMp), [app.books]);
  const [active, setActive] = useState<Book | null>(sources[0] ?? null);
  const [articles, setArticles] = useState<MPArticle[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState<ChapterContent | null>(null);

  useEffect(() => {
    if (!active) setActive(sources[0] ?? null);
  }, [sources, active]);

  useEffect(() => {
    if (!active) return;
    setLoading(true);
    api
      .mpArticles(active.id, 0)
      .then((rows) => setArticles(rows ?? []))
      .catch((e: unknown) => {
        setArticles([]);
        toast(String((e as Error).message || e), 'error');
      })
      .finally(() => setLoading(false));
  }, [active]);

  const read = async (a: MPArticle) => {
    try {
      const content = await api.openMPArticle(a.reviewId);
      setOpen(content);
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };

  return (
    <>
      <div className="page-head">
        <h1>公众号</h1>
        <span className="sub">{sources.length} 个订阅</span>
        <div className="grow" />
        {active ? (
          <button
            className="btn btn-ghost"
            onClick={() =>
              api
                .mpArticles(active.id, (articles ?? []).length)
                .then((more) => setArticles((prev) => [...(prev ?? []), ...(more ?? [])]))
                .catch((e: unknown) => toast(String((e as Error).message || e), 'error'))
            }
          >
            加载更多
          </button>
        ) : null}
      </div>

      <div className="page-body">
        {sources.length === 0 ? (
          <div className="empty">
            <Icon name="at" size={32} />
            <p>书架里还没有公众号。请在微信读书中添加订阅后同步书架。</p>
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: '220px minmax(0,1fr)', gap: 18 }}>
            <div className="settings-nav">
              {sources.map((s) => (
                <button key={s.id} className={active?.id === s.id ? 'on' : ''} onClick={() => setActive(s)}>
                  <span className="truncate">{s.title}</span>
                </button>
              ))}
            </div>
            <div>
              {loading ? (
                <div className="empty" style={{ height: 180 }}>
                  <div className="spinner" />
                </div>
              ) : (articles ?? []).length === 0 ? (
                <div className="empty" style={{ height: 180 }}>
                  <p>暂无文章</p>
                </div>
              ) : (
                <div className="article-list">
                  {(articles ?? []).map((a) => (
                    <div key={a.reviewId} className="article-row" onClick={() => void read(a)}>
                      {a.cover ? (
                        <img
                          src={a.cover}
                          alt=""
                          style={{ width: 46, height: 46, borderRadius: 6, objectFit: 'cover', flex: 'none' }}
                          referrerPolicy="no-referrer"
                        />
                      ) : (
                        <div className="avatar" style={{ borderRadius: 6 }}>
                          <Icon name="at" size={16} />
                        </div>
                      )}
                      <div className="grow">
                        <div className="truncate" style={{ fontSize: 13.5 }}>
                          {a.title}
                        </div>
                        <div className="faint" style={{ fontSize: 11.5 }}>
                          {formatDate(a.createTime)}
                        </div>
                      </div>
                      <Icon name="right" size={14} />
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {open && theme ? <ArticleReader content={open} onClose={() => setOpen(null)} theme={theme} /> : null}
    </>
  );
}

function ArticleReader({ content, onClose, theme }: { content: ChapterContent; onClose: () => void; theme: any }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const blocks = content.blocks ?? [];
  const metrics = useMemo(() => measureBlocks(blocks), [blocks]);

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 60,
        background: 'rgba(6,8,12,0.72)',
        backdropFilter: 'blur(6px)',
        display: 'flex',
        justifyContent: 'center',
        padding: '46px 24px 24px',
      }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          width: 'min(760px, 100%)',
          height: '100%',
          overflow: 'auto',
          borderRadius: 12,
          background: theme.background === 'transparent' ? '#fff' : theme.background,
          color: theme.textColor,
          fontFamily: fontStack(theme),
          fontSize: theme.fontSize,
          lineHeight: theme.lineHeight,
          padding: '34px 44px 60px',
          boxShadow: '0 30px 80px rgba(0,0,0,0.5)',
          position: 'relative',
        }}
      >
        <button
          className="icon-btn"
          style={{ position: 'sticky', top: 0, float: 'right', color: 'inherit' }}
          onClick={onClose}
          aria-label="关闭"
        >
          ×
        </button>
        <h1 style={{ fontSize: '1.4em', margin: '0 0 1em', color: theme.headingColor }}>{content.title}</h1>
        {blocks.map((b, i) => (
          <BlockView
            key={i}
            block={b}
            metric={metrics.metrics[i]}
            reveal={null}
            caretClass=""
            caretRef={{ current: null }}
            showCaret={false}
            faintUnread={false}
          />
        ))}
      </div>
    </div>
  );
}

function formatDate(ts: number): string {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
