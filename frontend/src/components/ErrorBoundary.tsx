import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';

import { Icon } from '../icons';
import { store } from '../store';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

// Packaged builds have no browser error overlay: an uncaught render error makes
// React unmount the whole tree and leaves a blank window that looks frozen.
// This boundary turns that dead end into a readable page with recovery actions.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[render]', error, info.componentStack);
  }

  private backToShelf = () => {
    store.set({ route: { name: 'shelf' } });
    this.setState({ error: null });
  };

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div className="terms">
        <div className="terms-card">
          <div className="row">
            <div
              className="mark"
              style={{
                width: 40,
                height: 40,
                borderRadius: 12,
                background: 'linear-gradient(145deg,#4d84ff,#7a5cff)',
                display: 'grid',
                placeItems: 'center',
                color: '#fff',
              }}
            >
              <Icon name="chart" size={20} />
            </div>
            <h1 style={{ margin: 0 }}>界面出错了</h1>
          </div>
          <p className="muted">
            这一页在渲染时发生异常，应用已停止绘制它。返回书架后其他页面仍可继续使用。
          </p>
          <pre
            className="faint"
            style={{
              maxWidth: 560,
              whiteSpace: 'pre-wrap',
              fontSize: 12,
              lineHeight: 1.5,
              margin: 0,
            }}
          >
            {error.message}
          </pre>
          <div className="terms-actions">
            <button className="btn btn-primary" onClick={this.backToShelf}>
              返回书架
            </button>
            <button className="btn btn-ghost" onClick={() => window.location.reload()}>
              重新加载
            </button>
          </div>
        </div>
      </div>
    );
  }
}
