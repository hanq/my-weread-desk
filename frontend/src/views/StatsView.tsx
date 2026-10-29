import { useEffect, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { toast } from '../store';
import type { ReadStats } from '../types';

const MODES: [string, string][] = [
  ['weekly', '本周'],
  ['monthly', '本月'],
  ['annually', '今年'],
  ['overall', '总计'],
];

export function StatsView() {
  const [mode, setMode] = useState('weekly');
  const [stats, setStats] = useState<ReadStats | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    api
      .stats(mode, 0)
      .then(setStats)
      .catch((e: unknown) => toast(String((e as Error).message || e), 'error'))
      .finally(() => setLoading(false));
  }, [mode]);

  const max = Math.max(1, ...(stats?.dailyReadTimes ?? [0]));

  return (
    <>
      <div className="page-head">
        <h1>阅读统计</h1>
        <span className="sub">数据来自微信读书</span>
        <div className="grow" />
        <div className="seg">
          {MODES.map(([k, label]) => (
            <button key={k} className={mode === k ? 'on' : ''} onClick={() => setMode(k)}>
              {label}
            </button>
          ))}
        </div>
        <button
          className="icon-btn"
          title="刷新"
          onClick={() =>
            api
              .stats(mode, 0)
              .then(setStats)
              .catch((e: unknown) => toast(String((e as Error).message || e), 'error'))
          }
        >
          <Icon name="refresh" size={16} />
        </button>
      </div>

      <div className="page-body">
        {loading && !stats ? (
          <div className="empty">
            <div className="spinner" />
          </div>
        ) : !stats ? (
          <div className="empty">
            <Icon name="chart" size={32} />
            <p>暂无统计数据</p>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
            <div className="stat-grid">
              <StatCard k="阅读时长" v={formatDuration(stats.totalReadTime)} />
              <StatCard k="阅读天数" v={`${stats.readDays} 天`} />
              <StatCard k="日均时长" v={formatDuration(stats.dayAverageReadTime)} />
              <StatCard k="排名" v={stats.rank > 0 ? `第 ${stats.rank} 名` : '—'} />
            </div>

            <div className="stat-card">
              <div className="k">每日阅读时长</div>
              <div className="bars">
                {(stats.dailyReadTimes.length ? stats.dailyReadTimes : new Array(7).fill(0)).map((v, i) => (
                  <div
                    key={i}
                    style={{ height: `${Math.max(2, (v / max) * 100)}%` }}
                    title={formatDuration(v)}
                  />
                ))}
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <div className="stat-card">
                <div className="k" style={{ marginBottom: 10 }}>
                  阅读最多的书
                </div>
                {(stats.readLongest ?? []).length === 0 ? (
                  <div className="faint">暂无记录</div>
                ) : (
                  stats.readLongest.slice(0, 6).map((s, i) => (
                    <div key={`${s.bookId}-${i}`} className="row" style={{ padding: '6px 0', fontSize: 12.5 }}>
                      <span className="faint mono" style={{ width: 18 }}>
                        {i + 1}
                      </span>
                      <span className="grow truncate">{s.title || s.bookId}</span>
                      <span className="faint">{formatDuration(s.seconds)}</span>
                    </div>
                  ))
                )}
              </div>

              <div className="stat-card">
                <div className="k" style={{ marginBottom: 10 }}>
                  偏好分类
                </div>
                {(stats.preferCategory ?? []).length === 0 ? (
                  <div className="faint">暂无记录</div>
                ) : (
                  stats.preferCategory.slice(0, 6).map((c) => (
                    <div key={c.name} style={{ padding: '6px 0' }}>
                      <div className="row" style={{ fontSize: 12.5 }}>
                        <span className="grow truncate">{c.name}</span>
                        <span className="faint">{formatDuration(c.seconds)}</span>
                      </div>
                      <div style={{ height: 3, borderRadius: 99, background: 'rgba(255,255,255,0.08)', marginTop: 5 }}>
                        <div
                          style={{
                            height: '100%',
                            width: `${Math.min(100, c.ratio * 100 || 0)}%`,
                            borderRadius: 99,
                            background: 'var(--accent)',
                          }}
                        />
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

function StatCard({ k, v }: { k: string; v: string }) {
  return (
    <div className="stat-card">
      <div className="k">{k}</div>
      <div className="v">{v}</div>
    </div>
  );
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds < 60) return `${Math.max(0, Math.round(seconds))} 秒`;
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分钟`;
}
