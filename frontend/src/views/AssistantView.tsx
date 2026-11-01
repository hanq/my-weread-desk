import { useEffect, useMemo, useRef, useState } from 'react';

import { api, events } from '../api';
import { Icon } from '../icons';
import { navigate, store, toast, useApp } from '../store';
import type { AISettings, AISource, AskEvent, RAGStatus } from '../types';

export function AssistantView({ initialBookId }: { initialBookId?: string } = {}) {
  const app = useApp();
  const books = useMemo(() => app.books.filter((b) => !b.isAudio && !b.isMp), [app.books]);

  const [showSetup, setShowSetup] = useState(false);
  const [form, setForm] = useState<AISettings | null>(null);
  const [saving, setSaving] = useState(false);

  const [bookId, setBookId] = useState('');
  const [status, setStatus] = useState<RAGStatus | null>(null);
  const [indexing, setIndexing] = useState(false);

  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');
  const [sources, setSources] = useState<AISource[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const requestRef = useRef<string | null>(null);
  const answerRef = useRef('');
  const appliedInitial = useRef('');

  const ai = app.settings?.ai;
  const configured = Boolean(ai?.enabled && ai.baseUrl.trim() && ai.model.trim() && ai.consent);

  useEffect(() => {
    if (!form && ai) setForm({ ...ai });
  }, [ai, form]);

  // The shelf and the detail page can open the assistant with a specific book.
  // Apply that book once so the reader can still switch books afterwards.
  useEffect(() => {
    if (books.length === 0) return;
    if (initialBookId && books.some((b) => b.id === initialBookId)) {
      if (appliedInitial.current !== initialBookId) {
        appliedInitial.current = initialBookId;
        setBookId(initialBookId);
      }
      return;
    }
    if (bookId) return;
    const last = app.settings?.lastBookId;
    const pick = books.find((b) => b.id === last) ?? books[0];
    setBookId(pick.id);
  }, [initialBookId, app.settings?.lastBookId, bookId, books]);

  useEffect(() => {
    if (!bookId) return;
    let alive = true;
    setStatus(null);
    api
      .ragStatus(bookId)
      .then((s) => alive && setStatus(s))
      .catch(() => alive && setStatus(null));
    return () => {
      alive = false;
    };
  }, [bookId]);

  useEffect(() => {
    return events.on('ai:ask', (ev: AskEvent) => {
      if (!requestRef.current || ev.requestId !== requestRef.current) return;
      if (ev.delta) {
        answerRef.current += ev.delta;
        setAnswer(answerRef.current);
      }
      if (ev.error) {
        setError(ev.error);
        setBusy(false);
        requestRef.current = null;
      }
      if (ev.text) {
        answerRef.current = ev.text;
        setAnswer(ev.text);
        setSources(ev.sources ?? []);
        setBusy(false);
        requestRef.current = null;
      }
    });
  }, []);

  const saveConfig = async () => {
    if (!form || !app.settings) return;
    setSaving(true);
    try {
      const next = await api.saveSettings({ ...app.settings, ai: form });
      store.set({ settings: next });
      setShowSetup(false);
      toast('AI 配置已保存', 'success');
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    } finally {
      setSaving(false);
    }
  };

  const index = async () => {
    if (!bookId || indexing) return;
    setIndexing(true);
    try {
      const next = await api.indexBook(bookId);
      setStatus(next);
      toast(
        next.indexed ? `索引完成：${next.chunkCount} 块` : next.error || '索引失败',
        next.indexed ? 'success' : 'error',
      );
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    } finally {
      setIndexing(false);
    }
  };

  const ask = async () => {
    if (!configured) {
      setShowSetup(true);
      return;
    }
    if (!question.trim() || busy) return;
    setBusy(true);
    setError('');
    setAnswer('');
    setSources([]);
    answerRef.current = '';
    try {
      const id = await api.askBook(bookId, question.trim());
      requestRef.current = id;
    } catch (e) {
      setBusy(false);
      setError(String((e as Error).message || e));
    }
  };

  const cancel = () => {
    if (requestRef.current) void api.cancelAsk(requestRef.current);
    requestRef.current = null;
    setBusy(false);
  };

  const jump = (source: AISource) => {
    navigate({ name: 'reader', bookId: source.bookId, chapterUid: source.chapterUid });
  };

  const book = books.find((b) => b.id === bookId);
  const indexed = Boolean(status?.indexed);

  return (
    <>
      <div className="page-head">
        <h1>读书助手</h1>
        <span className="sub">{book ? `原文问答 · 《${book.title}》` : '原文问答 · 只依据书中原文'}</span>
        <div className="grow" />
        <select className="assistant-select" value={bookId} onChange={(e) => setBookId(e.target.value)}>
          {books.map((b) => (
            <option key={b.id} value={b.id}>
              {b.title}
            </option>
          ))}
        </select>
        <button
          className="btn btn-ghost"
          title="改写剧情、分叉世界线请使用故事穿越"
          onClick={() => navigate({ name: 'traversal', bookId: bookId || undefined })}
        >
          <Icon name="clock" size={14} /> 故事穿越
        </button>
        <button className="btn btn-ghost" onClick={() => setShowSetup((v) => !v)}>
          <Icon name="settings" size={14} /> 配置
        </button>
      </div>

      <div className="page-body assistant">
        {showSetup ? (
          <div className="assistant-setup">
            <div className="assistant-setup-title">AI 服务</div>
            <p className="faint">
              兼容 OpenAI 协议的服务均可：OpenAI、DeepSeek、Moonshot、硅基流动、Ollama、LM Studio。
              只有检索命中的正文片段会发送到该地址。
            </p>
            <div className="assistant-form">
              <label>
                <span>服务地址</span>
                <input
                  value={form?.baseUrl ?? ''}
                  placeholder="https://api.deepseek.com 或 http://127.0.0.1:11434"
                  onChange={(e) => form && setForm({ ...form, baseUrl: e.target.value })}
                />
              </label>
              <label>
                <span>模型</span>
                <input
                  value={form?.model ?? ''}
                  placeholder="deepseek-chat / qwen2.5:7b"
                  onChange={(e) => form && setForm({ ...form, model: e.target.value })}
                />
              </label>
              <label>
                <span>API Key</span>
                <input
                  type="password"
                  value={form?.apiKey ?? ''}
                  placeholder="本地模型可留空"
                  onChange={(e) => form && setForm({ ...form, apiKey: e.target.value })}
                />
              </label>
              <label>
                <span>温度</span>
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  max="2"
                  value={form?.temperature ?? 0.3}
                  onChange={(e) => form && setForm({ ...form, temperature: Number(e.target.value) })}
                />
              </label>
            </div>
            <label className="assistant-check">
              <input
                type="checkbox"
                checked={Boolean(form?.enabled)}
                onChange={(e) => form && setForm({ ...form, enabled: e.target.checked })}
              />
              启用助手
            </label>
            <label className="assistant-check">
              <input
                type="checkbox"
                checked={Boolean(form?.consent)}
                onChange={(e) => form && setForm({ ...form, consent: e.target.checked })}
              />
              我已了解：检索到的书籍正文片段会发送到上面配置的服务
            </label>
            <div className="assistant-actions">
              <button className="btn btn-primary" disabled={saving} onClick={() => void saveConfig()}>
                {saving ? '保存中…' : '保存'}
              </button>
              <button className="btn btn-ghost" onClick={() => setShowSetup(false)}>
                取消
              </button>
            </div>
          </div>
        ) : null}

        <div className="assistant-status">
          <Icon name="folder" size={14} />
          {status?.status === 'indexing' || indexing ? (
            <span>正在建立索引…</span>
          ) : indexed ? (
            <span>
              已建索引 · {status?.chunkCount} 块 · {Math.round((status?.charCount ?? 0) / 1000)}k 字
            </span>
          ) : (
            <span>{status?.error ? `索引失败：${status.error}` : '尚未建立索引'}</span>
          )}
          <div className="grow" />
          <button className="btn btn-ghost" disabled={!bookId || indexing} onClick={() => void index()}>
            <Icon name="refresh" size={14} /> {indexed ? '重建索引' : '建立索引'}
          </button>
        </div>

        <div className="assistant-chat">
          {!answer && !busy && !error ? (
            <div className="empty">
              <Icon name="sparkle" size={30} />
              <p>问这本书记了什么，或者让助手定位某个情节。</p>
              <p className="faint">回答只依据书中原文，并附上可跳转的段落引用。</p>
              <p className="faint">需要改写剧情、分叉世界线，请使用「故事穿越」。</p>
            </div>
          ) : null}

          {answer ? <div className="assistant-answer">{answer}</div> : null}
          {busy && !answer ? (
            <div className="assistant-answer">
              <span className="spinner" />
            </div>
          ) : null}
          {error ? <div className="assistant-error">{error}</div> : null}

          {sources.length > 0 ? (
            <div className="assistant-sources">
              <div className="assistant-sources-title">引用</div>
              {sources.map((s) => (
                <button key={`${s.chapterUid}-${s.blockStart}`} className="source-item" onClick={() => jump(s)}>
                  <span className="source-index">[{s.index}]</span>
                  <span className="grow">
                    <span className="truncate">
                      {s.chapterTitle || `第 ${s.chapterIdx} 章`} · blocks {s.blockStart}–{s.blockEnd}
                    </span>
                    <span className="source-excerpt">{s.excerpt}</span>
                  </span>
                  <Icon name="right" size={13} />
                </button>
              ))}
            </div>
          ) : null}
        </div>

        <div className="assistant-input">
          <input
            value={question}
            placeholder={indexed ? '例如：主角第一次改变主意是在哪一章？' : '请先为这本书建立索引'}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                void ask();
              }
            }}
          />
          {busy ? (
            <button className="btn btn-ghost" onClick={cancel}>
              停止
            </button>
          ) : (
            <button className="btn btn-primary" disabled={!question.trim() || !indexed} onClick={() => void ask()}>
              提问
            </button>
          )}
        </div>
      </div>
    </>
  );
}
