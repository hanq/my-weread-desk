import { useCallback, useEffect, useRef, useState } from 'react';

import { api, runtime } from '../api';
import { Icon } from '../icons';
import { store, toast } from '../store';
import type { QRCode } from '../types';

type Phase = 'starting' | 'waiting' | 'otp' | 'expired' | 'blocked' | 'done';

/** Delay before the next poll after a *pending* answer. The server already held
 *  the request open for ~55s, so a short pause is enough. */
const PENDING_DELAY_MS = 500;
/** Backoff after a transport or parse error, capped, and only a few attempts. */
const ERROR_BACKOFF_MS = [3000, 6000, 12000, 20000];
const MAX_CONSECUTIVE_ERRORS = 4;

export function LoginView() {
  const [qr, setQr] = useState<QRCode | null>(null);
  const [phase, setPhase] = useState<Phase>('starting');
  const [message, setMessage] = useState('正在获取二维码…');
  const [otp, setOtp] = useState('');
  const [busy, setBusy] = useState(false);
  const [remaining, setRemaining] = useState(0);
  const [attempts, setAttempts] = useState(0);

  // The polling loop reads these through refs so a state change never spawns a
  // second loop; there is exactly one request in flight at any moment.
  const otpRef = useRef('');
  const errorCount = useRef(0);
  const generation = useRef(0);
  const timer = useRef(0);
  const alive = useRef(true);

  const finish = useCallback(async () => {
    setPhase('done');
    setMessage('登录成功，正在同步书架…');
    try {
      const [auth, books] = await Promise.all([api.getAuthState(), api.books()]);
      store.set({ auth, books, online: true });
    } catch {
      const auth = await api.getAuthState().catch(() => null);
      if (auth) store.set({ auth });
    }
    toast('登录成功', 'success');
  }, []);

  /** Requests a brand new QR code. Only user actions call this. */
  const start = useCallback(async () => {
    generation.current += 1;
    const gen = generation.current;
    window.clearTimeout(timer.current);
    errorCount.current = 0;
    setAttempts(0);
    setBusy(false);
    setOtp('');
    otpRef.current = '';
    setQr(null);
    setPhase('starting');
    setMessage('正在获取二维码…');
    try {
      const code = await api.startLogin();
      if (!alive.current || gen !== generation.current) return;
      setQr(code);
      setPhase('waiting');
      setMessage('请使用微信扫码，并在手机上确认登录');
      setRemaining(Math.max(0, code.expires - Math.floor(Date.now() / 1000)));
    } catch (e) {
      if (!alive.current || gen !== generation.current) return;
      setPhase('blocked');
      setMessage(String((e as Error).message || e));
    }
  }, []);

  useEffect(() => {
    alive.current = true;
    void start();
    return () => {
      alive.current = false;
      generation.current += 1;
      window.clearTimeout(timer.current);
    };
  }, [start]);

  // Single-flight poll loop. It reschedules itself and never creates a QR code.
  // It only runs while waiting for a scan: once the server asks for the SMS
  // code the loop stops and each submitted code costs exactly one request.
  useEffect(() => {
    if (!qr || phase !== 'waiting') return;
    const gen = generation.current;
    let stopped = false;

    const schedule = (ms: number) => {
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(poll, ms);
    };

    const poll = async () => {
      if (stopped || !alive.current || gen !== generation.current) return;
      try {
        const res = await api.pollLogin(qr.uid, otpRef.current);
        if (stopped || !alive.current || gen !== generation.current) return;
        errorCount.current = 0;
        setAttempts(0);

        if (res.done) {
          await finish();
          return;
        }
        if (res.expired) {
          setPhase('expired');
          setMessage(res.message || '二维码已过期，请重新获取');
          return;
        }
        if (res.needOtp) {
          setPhase('otp');
          setMessage(res.message || '请输入手机上的四位验证码');
          return;
        }
        if (res.message) setMessage(res.message);
        schedule(PENDING_DELAY_MS);
      } catch (e) {
        if (stopped || !alive.current || gen !== generation.current) return;
        errorCount.current += 1;
        setAttempts(errorCount.current);
        if (errorCount.current >= MAX_CONSECUTIVE_ERRORS) {
          setPhase('blocked');
          setMessage(`连续 ${errorCount.current} 次请求失败，已暂停轮询。请检查网络后手动刷新。`);
          return;
        }
        const delay = ERROR_BACKOFF_MS[Math.min(errorCount.current - 1, ERROR_BACKOFF_MS.length - 1)];
        setMessage(`${String((e as Error).message || e)}（${Math.round(delay / 1000)} 秒后重试）`);
        schedule(delay);
      }
    };

    schedule(PENDING_DELAY_MS);
    return () => {
      stopped = true;
      window.clearTimeout(timer.current);
    };
  }, [qr, phase, finish]);

  // Countdown of the QR validity window; purely informational.
  useEffect(() => {
    if (!qr || phase === 'done' || phase === 'blocked') return;
    const id = window.setInterval(() => {
      setRemaining(Math.max(0, qr.expires - Math.floor(Date.now() / 1000)));
    }, 1000);
    return () => window.clearInterval(id);
  }, [qr, phase]);

  const submitOtp = async () => {
    if (!/^\d{4}$/.test(otp)) {
      setMessage('请输入手机上的四位数字验证码');
      return;
    }
    otpRef.current = otp;
    setBusy(true);
    setMessage('正在验证…');
    try {
      const res = await api.pollLogin(qr?.uid ?? '', otp);
      if (res.done) {
        await finish();
        return;
      }
      if (res.expired) {
        setPhase('expired');
        setMessage(res.message || '二维码已过期，请重新获取');
        return;
      }
      setMessage(res.message || '验证码不正确，请重新输入');
      setPhase('otp');
    } catch (e) {
      setMessage(String((e as Error).message || e));
    } finally {
      setBusy(false);
    }
  };

  const polling = phase === 'waiting' || phase === 'otp' || phase === 'starting';

  return (
    <div className="login-screen">
      <div className="login-copy">
        <div className="mark">
          <img src="/weread-icon.png" alt="微信读书" />
        </div>
        <h1>
          你的书架，
          <br />
          搬回桌面。
        </h1>
        <p>扫码一次即可同步书架、搜索书籍、按章下载，并以你自己的排版阅读。进度与阅读时长会回写到微信读书。</p>
        <div className="login-facts">
          <div>
            <Icon name="check" size={14} /> 扫码登录，不需要输入密码
          </div>
          <div>
            <Icon name="check" size={14} /> 设备指纹独立，不与网页端互踢
          </div>
          <div>
            <Icon name="check" size={14} /> 正文与图片缓存在本地，可离线阅读
          </div>
          <div>
            <Icon name="check" size={14} /> 禅模式 · 主题 · 打字伪装 · Word / VS Code 皮肤
          </div>
        </div>
        <div className="faint" style={{ fontSize: 11.5, marginTop: 8 }}>
          非官方工具，仅供个人学习研究。请遵守微信读书用户协议。
        </div>
      </div>

      <div className="login-panel">
        <div className="qr-card">
          <h2>微信扫码登录</h2>
          <div className="qr-box">
            {qr?.image && phase !== 'expired' && phase !== 'blocked' ? (
              <img src={qr.image} alt="登录二维码" />
            ) : phase === 'done' ? (
              <div style={{ display: 'grid', placeItems: 'center', gap: 8, color: '#2f6fed' }}>
                <Icon name="check" size={40} />
                <span style={{ fontSize: 12 }}>已登录</span>
              </div>
            ) : (
              <div style={{ display: 'grid', placeItems: 'center', gap: 8, color: '#8a90a0' }}>
                <Icon name={phase === 'blocked' ? 'close' : 'refresh'} size={28} />
                <span style={{ fontSize: 12 }}>{phase === 'blocked' ? '已暂停轮询' : '二维码已过期'}</span>
              </div>
            )}
          </div>

          {phase === 'otp' ? (
            <div className="otp-row">
              <input
                value={otp}
                maxLength={4}
                inputMode="numeric"
                placeholder="····"
                autoFocus
                onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void submitOtp();
                }}
              />
              <button onClick={() => void submitOtp()} disabled={busy}>
                {busy ? '验证中' : '确认'}
              </button>
            </div>
          ) : null}

          <div className="qr-hint">{message}</div>

          <div className="faint" style={{ fontSize: 11, textAlign: 'center' }}>
            {polling && qr
              ? remaining > 0
                ? `二维码有效期剩余 ${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')}`
                : '二维码即将过期'
              : phase === 'blocked' && attempts > 0
                ? '轮询已暂停，不会自动重试'
                : '\u00a0'}
          </div>

          <button
            className="btn btn-ghost"
            style={{ color: '#4a5260', border: '1px solid #dde1ea', background: 'transparent' }}
            onClick={() => void start()}
            disabled={phase === 'starting'}
          >
            <Icon name="refresh" size={13} /> {qr ? '重新获取二维码' : '获取二维码'}
          </button>
          <button
            style={{ fontSize: 11.5, color: '#8a90a0', background: 'none' }}
            onClick={() => runtime.openURL('https://weread.qq.com/r/weread-skills')}
          >
            如何开通微信读书 Skill？
          </button>
        </div>
      </div>
    </div>
  );
}
