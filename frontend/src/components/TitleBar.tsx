import { api } from '../api';
import { Icon } from '../icons';
import { store, useApp } from '../store';
import { TitleOrb } from './TitleOrb';

const ROUTE_LABELS: Record<string, string> = {
  shelf: '书架',
  search: '书城',
  stats: '统计',
  smart: '智能书架',
  assistant: '读书助手',
  traversal: '故事穿越',
  settings: '设置',
  detail: '书籍详情',
  mp: '公众号',
  branch: '故事分支',
  reader: '阅读',
};

export function TitleBar({ onOpenNav }: { onOpenNav?: () => void }) {
  const app = useApp();
  const win = app.window;
  const pinned = win?.alwaysOnTop ?? false;
  const routeLabel = app.auth?.loggedIn ? ROUTE_LABELS[app.route.name] ?? '' : '';

  const togglePin = () => {
    void api.setAlwaysOnTop(!pinned).then((w) => store.set({ window: w }));
  };

  return (
    <div className="titlebar">
      <TitleOrb />
      {onOpenNav ? (
        <button className="icon-btn nav-toggle" onClick={onOpenNav} title="菜单" aria-label="菜单">
          <Icon name="list" size={15} />
        </button>
      ) : null}
      {routeLabel ? (
        <>
          <span className="title-divider" aria-hidden="true" />
          <span className="title-context">{routeLabel}</span>
        </>
      ) : null}
      {app.online ? null : <span className="offline-badge">离线</span>}
      <div className="spacer" />
      <div className="win-actions">
        <button
          className={`icon-btn${pinned ? ' active' : ''}`}
          title="窗口置顶 (Ctrl+Alt+T)"
          onClick={togglePin}
        >
          <Icon name={pinned ? 'pin' : 'unpin'} size={15} />
        </button>
        <button className="icon-btn" title="最小化" onClick={() => void api.minimiseWindow()}>
          <Icon name="minimise" size={15} />
        </button>
        <button className="icon-btn" title="最大化" onClick={() => void api.maximiseWindow()}>
          <Icon name="maximise" size={14} />
        </button>
        <button className="icon-btn close" title="关闭" onClick={() => void api.closeWindow()}>
          <Icon name="close" size={15} />
        </button>
      </div>
      <span className="titlebar-dew dew-a" aria-hidden="true" />
      <span className="titlebar-dew dew-b" aria-hidden="true" />
    </div>
  );
}
