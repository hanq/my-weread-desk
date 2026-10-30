import { useCallback, useEffect, useRef, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { store, toast } from '../store';
import type { DeviceReadStats, ReadStats } from '../types';

const MODES: [string, string][] = [
  ['weekly', '本周'],
  ['monthly', '本月'],
  ['annually', '今年'],
  ['overall', '总计'],
];

export function StatsView() {
  const [mode, setMode] = useState('weekly');
  const [stats, setStats] = useState<ReadStats | null>(null);
  const [deviceStats, setDeviceStats] = useState<DeviceReadStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const hasStats = useRef(false);

  const load = useCallback(() => {
    setLoading(true);
    setError('');
    api
      .deviceStats()
      .then(setDeviceStats)
      .catch(() => undefined);
    api
      .stats(mode, 0)
      .then((data) => {
        hasStats.current = true;
        setStats(data);
      })
      .catch((e: unknown) => {
        const message = String((e as Error).message || e);
        if (hasStats.current) {
          toast(message, 'error');
        } else {
          setError(message);
        }
      })
      .finally(() => setLoading(false));
  }, [mode]);

  useEffect(() => {
    load();
  }, [load]);

  const relogin = useCallback(async () => {
    try {
      await api.logout();
      store.set({ auth: { loggedIn: false, hasApiKey: false }, books: [] });
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  }, []);

  // Go marshals a nil slice as null, and the API omits dailyReadTimes for
  // periods without records, so the render below must not assume an array.
  const daily = stats?.dailyReadTimes ?? [];
  const dailyAt = stats?.dailyReadTimesAt ?? [];
  const max = Math.max(1, ...daily);
  const series = daily.length ? daily : new Array(7).fill(0);
  // A month has 28+ buckets: labeling every one of them collides in a narrow
  // window, so only every Nth bucket (and the last) gets a date.
  const labelStep = series.length <= 12 ? 1 : Math.ceil(series.length / 10);
  const labelAt = (i: number) => {
    const ts = dailyAt[i];
    if (!ts || (i % labelStep !== 0 && i !== series.length - 1)) return '';
    return formatBucket(ts, mode);
  };

  return (
    <>
      <div className="page-head">
        <h1>阅读统计</h1>
        <span className="sub">账号数据来自微信读书 · 本设备由 MyWeReadDesk 记录</span>
        <div className="grow" />
        <div className="seg">
          {MODES.map(([k, label]) => (
            <button key={k} className={mode === k ? 'on' : ''} onClick={() => setMode(k)}>
              {label}
            </button>
          ))}
        </div>
        <button className="icon-btn" title="刷新" onClick={load}>
          <Icon name="refresh" size={16} />
        </button>
      </div>

      <div className="page-body">
        {deviceStats ? (
          <div style={{ marginBottom: 20 }}>
            <div className="stat-grid">
              <StatCard k={`本设备阅读 · ${deviceStats.client}`} v={formatDuration(deviceStats.totalSeconds)} />
              <StatCard k="本设备今日" v={formatDuration(deviceStats.todaySeconds)} />
            </div>
            <p className="faint" style={{ fontSize: 12, marginTop: 8 }}>
              微信读书只提供账号汇总，不提供按客户端拆分；本设备时长由 {deviceStats.client} 在本地记录。
            </p>
          </div>
        ) : null}
        {loading && !stats ? (
          <div className="empty">
            <div className="spinner" />
          </div>
        ) : error ? (
          <div className="empty">
            <Icon name="chart" size={32} />
            <p style={{ maxWidth: 520 }}>{error}</p>
            {/旧版网页|需要登录|登录/.test(error) ? (
              <p className="faint" style={{ maxWidth: 520, fontSize: 12 }}>
                阅读统计需要新版扫码登录；旧版网页凭据只能读取书架、正文和划线。
              </p>
            ) : null}
            <div className="row">
              {/旧版网页|需要登录|登录/.test(error) ? (
                <button className="btn btn-primary" onClick={() => void relogin()}>
                  退出并重新扫码
                </button>
              ) : null}
              <button className="btn btn-ghost" onClick={load}>
                重试
              </button>
            </div>
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
                {series.map((v, i) => (
                  <div
                    key={i}
                    style={{ height: `${Math.max(2, (v / max) * 100)}%` }}
                    title={
                      dailyAt[i]
                        ? `${formatBucket(dailyAt[i], mode)} · ${formatDuration(v)}`
                        : formatDuration(v)
                    }
                  />
                ))}
              </div>
              <div className="chart-labels">
                {series.map((_, i) => (
                  <span key={i}>{labelAt(i)}</span>
                ))}
              </div>
            </div>

            <div className="stat-pair">
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

/** Labels a chart bucket: a day for week/month, a month for a year, a year for all time. */
function formatBucket(seconds: number, mode: string): string {
  const d = new Date(seconds * 1000);
  if (mode === 'overall') return String(d.getFullYear());
  if (mode === 'annually') return `${d.getMonth() + 1}月`;
  return `${d.getMonth() + 1}/${d.getDate()}`;
}
