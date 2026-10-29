import { useCallback, useEffect, useState } from 'react';

import { api, events, runtime } from './api';
import { Icon } from './icons';
import { navigate, store, toast, useApp } from './store';
import { TitleBar } from './components/TitleBar';
import { Sidebar } from './components/Sidebar';
import { LoginView } from './views/LoginView';
import { ShelfView } from './views/ShelfView';
import { SearchView } from './views/SearchView';
import { BookDetailView } from './views/BookDetailView';
import { ReaderView } from './views/ReaderView';
import { SettingsView } from './views/SettingsView';
import { StatsView } from './views/StatsView';
import { MPView } from './views/MPView';
import type { DownloadState, Settings } from './types';

export default function App() {
  const app = useApp();
  const [booting, setBooting] = useState(true);
  const [bootError, setBootError] = useState('');

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [settings, auth, books] = await Promise.all([api.getSettings(), api.getAuthState(), api.books().catch(() => [])]);
        if (!alive) return;
        store.set({ settings, auth, books });
        if (auth.loggedIn) {
          // Refresh the shelf in the background; the cached list is already rendered.
          api
            .syncShelf()
            .then((list) => store.set({ books: list, online: true, shelfState: 'idle' }))
            .catch(() => store.set({ online: false, shelfState: 'error' }));
        }
      } catch (e) {
        if (alive) setBootError(String((e as Error).message || e));
      } finally {
        if (alive) setBooting(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  // Global events from the Go side (global hotkeys, download progress).
  useEffect(() => {
    const offHotkey = events.on('hotkey', (name: string) => {
      void handleHotkey(name);
    });
    const offDownload = events.on('download', (state: DownloadState) => {
      const downloads = { ...store.get().downloads, [state.bookId]: state };
      store.set({ downloads });
      if (!state.running && state.total > 0) {
        toast(
          state.failed > 0 ? `下载结束：成功 ${state.done}，失败 ${state.failed}` : `已缓存 ${state.done} 章`,
          state.failed > 0 ? 'error' : 'success',
        );
      }
    });
    const offLog = events.on('log', (line: string) => {
      console.debug('[weread]', line);
    });
    return () => {
      offHotkey?.();
      offDownload?.();
      offLog?.();
    };
  }, []);

  // Alt + drag moves the window anywhere, which is the only way to reposition
  // it once the chrome is hidden in zen mode or inside a skin.
  useEffect(() => {
    let dragging = false;
    let startX = 0;
    let startY = 0;
    let originX = 0;
    let originY = 0;

    const down = async (e: MouseEvent) => {
      if (!e.altKey || e.button !== 0) return;
      e.preventDefault();
      dragging = true;
      startX = e.screenX;
      startY = e.screenY;
      try {
        const [x, y] = await runtime.windowGetPosition();
        originX = x;
        originY = y;
      } catch {
        originX = 0;
        originY = 0;
      }
    };
    const move = (e: MouseEvent) => {
      if (!dragging) return;
      runtime.windowSetPosition(originX + (e.screenX - startX), originY + (e.screenY - startY));
    };
    const up = () => {
      dragging = false;
    };
    window.addEventListener('mousedown', down);
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
    return () => {
      window.removeEventListener('mousedown', down);
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
    };
  }, []);

  if (booting) {
    return (
      <div style={{ height: '100%', display: 'grid', placeItems: 'center', background: '#0f1116' }}>
        <div className="spinner" />
      </div>
    );
  }

  if (bootError) {
    return (
      <div className="terms">
        <div className="terms-card">
          <h1>启动失败</h1>
          <p className="muted">{bootError}</p>
          <button className="btn btn-primary" onClick={() => void api.openDataDir()}>
            打开数据目录
          </button>
        </div>
      </div>
    );
  }

  if (!app.settings?.acceptedTerms) {
    return (
      <div className="terms">
        <div className="terms-card">
          <div className="row">
            <div className="mark" style={{ width: 40, height: 40, borderRadius: 12, background: 'linear-gradient(145deg,#4d84ff,#7a5cff)', display: 'grid', placeItems: 'center', color: '#fff' }}>
              <Icon name="book" size={20} />
            </div>
            <h1 style={{ margin: 0 }}>使用前请阅读</h1>
          </div>
          <ul>
            <li>本项目是第三方桌面客户端，与腾讯、微信读书没有任何隶属或合作关系。</li>
            <li>只能读取当前账号本就有权阅读的内容，不会绕过付费或会员限制。</li>
            <li>扫码登录后凭据仅保存在本机，使用系统凭据保护；不会上传到任何服务器。</li>
            <li>请控制请求频率并遵守微信读书用户协议。使用本工具产生的账号风险由使用者自行承担。</li>
            <li>本项目仅供个人学习与研究，不得用于商业用途。</li>
          </ul>
          <div className="terms-actions">
            <button
              className="btn btn-primary"
              onClick={() => {
                void api.acceptTerms().then(() => {
                  const s = store.get().settings;
                  if (s) store.set({ settings: { ...s, acceptedTerms: true } });
                });
              }}
            >
              我已阅读并同意
            </button>
            <button className="btn btn-ghost" onClick={() => void api.closeWindow()}>
              退出
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (!app.auth?.loggedIn) {
    return (
      <div className="app-root" style={{ borderRadius: 0, border: 0 }}>
        <TitleBar />
        <LoginView />
      </div>
    );
  }

  const route = app.route;

  if (route.name === 'reader') {
    return (
      <div style={{ height: '100%', background: app.settings?.theme.background ?? '#f5f2ea' }}>
        <ReaderView
          key={`${route.bookId}:${route.chapterUid}:${route.fromStart ? "start" : ""}`}
          bookId={route.bookId}
          chapterUid={route.chapterUid}
          skipRestore={Boolean(route.fromStart)}
        />
      </div>
    );
  }

  return (
    <div className="app-root">
      <TitleBar />
      <div className="shell">
        <Sidebar />
        <div className="main">
          {route.name === 'shelf' ? <ShelfView /> : null}
          {route.name === 'search' ? <SearchView /> : null}
          {route.name === 'detail' ? <BookDetailView bookId={route.bookId} /> : null}
          {route.name === 'settings' ? <SettingsView /> : null}
          {route.name === 'stats' ? <StatsView /> : null}
          {route.name === 'mp' ? <MPView /> : null}
        </div>
      </div>
      {app.toast ? <div className={`toast ${app.toast.kind}`}>{app.toast.text}</div> : null}
    </div>
  );
}

async function handleHotkey(name: string) {
  const settings = store.get().settings;
  if (!settings) return;
  switch (name) {
    case 'zen': {
      const on = !(store.get().window?.zen ?? settings.zen);
      const w = await api.setZen(on);
      store.set({ window: w, settings: { ...settings, zen: w.zen } });
      break;
    }
    case 'toggleTop': {
      const w = await api.setAlwaysOnTop(!(store.get().window?.alwaysOnTop ?? settings.window.alwaysOnTop));
      store.set({ window: w, settings: { ...settings, window: { ...settings.window, alwaysOnTop: w.alwaysOnTop } } });
      break;
    }
    case 'typing': {
      const w = await api.setTyping(!(store.get().window?.typing ?? settings.typing.enabled));
      store.set({ window: w, settings: { ...settings, typing: { ...settings.typing, enabled: w.typing } } });
      break;
    }
    case 'panic': {
      await api.hideWindow();
      break;
    }
    case 'cycleSkin': {
      const order = ['none', 'word', 'vscode'];
      const current = store.get().window?.skin ?? settings.skin;
      const next = order[(order.indexOf(current) + 1) % order.length];
      const w = await api.setSkin(next);
      store.set({ window: w, settings: { ...store.get().settings!, skin: w.skin } });
      toast(`皮肤：${next === 'none' ? '纯净' : next === 'word' ? 'Word' : 'VS Code'}`);
      break;
    }
    case 'clickThrough': {
      const w = await api.setClickThrough(!(store.get().window?.clickThrough ?? settings.window.clickThrough));
      store.set({ window: w, settings: { ...store.get().settings!, window: { ...settings.window, clickThrough: w.clickThrough } } });
      toast(w.clickThrough ? '已开启鼠标穿透（Ctrl+Alt+C 关闭）' : '已关闭鼠标穿透');
      break;
    }
    default:
      break;
  }
}

// Keep the settings type referenced for the store patch above.
export type { Settings };
