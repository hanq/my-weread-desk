import { api } from '../api';
import { Icon } from '../icons';
import { navigate, store, useApp } from '../store';

export function TitleBar() {
  const app = useApp();
  const win = app.window;
  const pinned = win?.alwaysOnTop ?? false;

  const togglePin = () => {
    void api.setAlwaysOnTop(!pinned).then((w) => store.set({ window: w }));
  };

  return (
    <div className="titlebar">
      <button className="brand" onClick={() => navigate({ name: 'shelf' })} style={{ cursor: 'pointer' }}>
        <span className="dot" />
        <span>MY WEREAD</span>
      </button>
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
    </div>
  );
}
