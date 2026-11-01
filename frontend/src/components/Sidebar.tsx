import { Fragment } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import type { IconName } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { Route } from '../store';

interface NavItem {
  key: Route['name'];
  label: string;
  hint: string;
  icon: IconName;
  group: '阅读' | '智能';
}

const NAV: NavItem[] = [
  { key: 'shelf', label: '书架', hint: '书架与阅读进度', icon: 'book', group: '阅读' },
  { key: 'search', label: '搜索', hint: '搜索书籍与全文', icon: 'search', group: '阅读' },
  { key: 'stats', label: '统计', hint: '阅读时长与统计', icon: 'chart', group: '阅读' },
  { key: 'smart', label: '智能书架', hint: '已准备的书与按书类型开放的能力', icon: 'sparkle', group: '智能' },
];
const GROUPS: NavItem['group'][] = ['阅读', '智能'];

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
      <span className="sidebar-ambient" aria-hidden="true">
        <span className="ambient-lamp" />
        <span className="ambient-aurora" />
        <span className="ambient-caustic caustic-a" />
        <span className="ambient-caustic caustic-b" />
        <span className="ambient-grid" />
        <span className="ambient-dew dew-a" />
        <span className="ambient-dew dew-b" />
        <span className="ambient-dew dew-c" />
      </span>
      {GROUPS.map((group) => (
        <Fragment key={group}>
          <div className="group-label">{group}</div>
          {NAV.filter((item) => item.group === group).map((item) => {
            const smartRoute =
              route.name === 'smart' || route.name === 'assistant' ||
              route.name === 'traversal' || route.name === 'branch';
            const active =
              route.name === item.key ||
              (item.key === 'shelf' && (route.name === 'detail' || route.name === 'reader')) ||
              (item.key === 'smart' && smartRoute);
            return (
              <button
                key={item.key}
                className={`nav-item${active ? ' on' : ''}`}
                onClick={() => go(item.key)}
                title={item.hint}
              >
                <span className="nav-aura" aria-hidden="true" />
                <Icon name={item.icon} size={16} />
                <span className="grow">{item.label}</span>
                {item.key === 'shelf' && app.books.length > 0 ? <span className="count">{app.books.length}</span> : null}
              </button>
            );
          })}
        </Fragment>
      ))}

      <div className="sidebar-foot">
        <button
          className={`nav-item${route.name === 'settings' ? ' on' : ''}`}
          onClick={() => go('settings')}
          title="界面、阅读与窗口设置"
        >
          <span className="nav-aura" aria-hidden="true" />
          <Icon name="settings" size={16} />
          <span className="grow">设置</span>
        </button>
        <button className="nav-item" onClick={sync}>
          <span className="nav-aura" aria-hidden="true" />
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
      </div>
    </nav>
  );
}
