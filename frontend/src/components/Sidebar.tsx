import { api } from '../api';
import { Icon } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { Route } from '../store';

const NAV: { key: Route['name']; label: string; icon: 'book' | 'search' | 'chart' | 'settings' | 'at' | 'sparkle' | 'clock' }[] = [
  { key: 'shelf', label: '书架', icon: 'book' },
  { key: 'search', label: '搜索', icon: 'search' },
  { key: 'assistant', label: '助手', icon: 'sparkle' },
  { key: 'traversal', label: '穿越', icon: 'clock' },
  { key: 'stats', label: '统计', icon: 'chart' },
  { key: 'settings', label: '设置', icon: 'settings' },
];

export function Sidebar({ onNavigate }: { onNavigate?: () => void } = {}) {
  const app = useApp();
  const route = app.route;

  const go = (name: Route['name']) => {
    onNavigate?.();
    navigate({ name } as Route);
  };

  const sync = () => {
    store.set({ shelfState: 'loading' });
    api
      .syncShelf()
      .then((books) => {
        store.set({ books, shelfState: 'idle', online: true });
        toast(`已同步 ${books.length} 本`, 'success');
      })
      .catch((e: unknown) => {
        store.set({ shelfState: 'error', online: false, shelfError: String((e as Error).message || e) });
        toast(String((e as Error).message || e), 'error');
      });
  };

  const account = app.auth?.account;

  return (
    <nav className="sidebar">
      <div className="group-label">阅读</div>
      {NAV.slice(0, 4).map((item) => (
        <button
          key={item.key}
          className={`nav-item${route.name === item.key || (item.key === 'shelf' && route.name === 'detail') || (item.key === 'shelf' && route.name === 'reader') ? ' on' : ''}`}
          onClick={() => go(item.key)}
          title={item.label}
        >
          <Icon name={item.icon} size={16} />
          <span className="grow">{item.label}</span>
          {item.key === 'shelf' && app.books.length > 0 ? <span className="count">{app.books.length}</span> : null}
        </button>
      ))}

      <div className="group-label">分析与设置</div>
      {NAV.slice(4).map((item) => (
        <button
          key={item.key}
          className={`nav-item${route.name === item.key ? ' on' : ''}`}
          onClick={() => go(item.key)}
          title={item.label}
        >
          <Icon name={item.icon} size={16} />
          <span className="grow">{item.label}</span>
        </button>
      ))}

      <div style={{ flex: 1 }} />

      <button className="nav-item" onClick={sync}>
        <Icon name="refresh" size={16} />
        <span className="grow">同步书架</span>
        {app.shelfState === 'loading' ? <span className="spinner" /> : null}
      </button>

      <div className="account">
        {account?.avatar ? (
          <img className="avatar" src={account.avatar} alt="" />
        ) : (
          <div className="avatar" style={{ display: 'grid', placeItems: 'center' }}>
            <Icon name="book" size={15} />
          </div>
        )}
        <div className="grow">
          <div className="truncate" style={{ fontSize: 12.5 }}>
            {account?.name || '微信读书账号'}
          </div>
          <div className="faint" style={{ fontSize: 11 }}>
            {account?.hasApiKey ? '已开通 Skill' : '未开通 Skill'}
          </div>
        </div>
        <button
          className="icon-btn"
          title="退出登录"
          onClick={() => {
            if (!window.confirm('退出登录后需要重新扫码，确定吗？')) return;
            void api.logout().then(() => {
              store.set({ auth: { loggedIn: false, hasApiKey: false }, books: [] });
              toast('已退出登录');
            });
          }}
        >
          <Icon name="external" size={15} />
        </button>
      </div>
    </nav>
  );
}
