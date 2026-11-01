import { useEffect, useMemo, useRef, useState } from 'react';

import { api } from '../api';
import { Icon } from '../icons';
import { store, toast, useApp } from '../store';
import type { Settings, Theme } from '../types';
import { SKIN_OPTIONS, avatarInitial, isChatSkin } from '../skins/registry';
import { SHELF_MOTIFS, setShelfMotif, useShelfMotif } from '../skins/shelfMotifs';
import { PatternGrid } from '../reader/PatternGrid';
import { DEFAULT_PATTERN_OPACITY } from '../reader/patterns';

type Section = 'appearance' | 'reading' | 'disguise' | 'window' | 'hotkeys' | 'about';

const NAV: [Section, string][] = [
  ['appearance', '外观'],
  ['reading', '阅读'],
  ['disguise', '伪装'],
  ['window', '窗口'],
  ['hotkeys', '快捷键'],
  ['about', '关于'],
];

export function SettingsView() {
  const app = useApp();
  const settings = app.settings;
  const shelfMotif = useShelfMotif();
  const [section, setSection] = useState<Section>('appearance');
  const [themes, setThemes] = useState<Theme[]>([]);
  const [fonts, setFonts] = useState<string[]>([]);
  const [dataDir, setDataDir] = useState('');
  const [version, setVersion] = useState('');
  const [screen, setScreen] = useState({ width: 0, height: 0 });
  const saveTimer = useRef<number>(0);

  useEffect(() => {
    void api.presetThemes().then(setThemes).catch(() => undefined);
    void api.fontFamilies().then(setFonts).catch(() => undefined);
    void api.dataDir().then(setDataDir).catch(() => undefined);
    void api.appVersion().then(setVersion).catch(() => undefined);
    void api.cursorPosition().then((p) => setScreen({ width: p.width, height: p.height })).catch(() => undefined);
  }, []);

  const persist = useMemo(
    () => (next: Settings) => {
      store.set({ settings: next });
      window.clearTimeout(saveTimer.current);
      saveTimer.current = window.setTimeout(() => {
        void api.saveSettings(next).then((saved) => store.set({ settings: saved })).catch((e: unknown) => toast(String((e as Error).message || e), 'error'));
      }, 260);
    },
    [],
  );

  if (!settings) return null;

  const update = (patch: Partial<Settings>) => persist({ ...settings, ...patch });
  const updateTheme = (patch: Partial<Theme>) => {
    // Two columns are a page spread; selecting 双栏 switches out of scroll mode.
    const reader =
      patch.columns === 2 && settings.reader.pageMode === 'scroll'
        ? { ...settings.reader, pageMode: 'page' as const }
        : settings.reader;
    persist({ ...settings, theme: { ...settings.theme, ...patch }, reader });
  };
  const setPageMode = (pageMode: 'page' | 'scroll') => {
    // Scroll mode is intentionally single-column; clear a previous 双栏 choice.
    const theme =
      pageMode === 'scroll' && settings.theme.columns === 2
        ? { ...settings.theme, columns: 1 }
        : settings.theme;
    persist({ ...settings, theme, reader: { ...settings.reader, pageMode } });
  };
  const updateReader = (patch: Partial<Settings['reader']>) =>
    persist({ ...settings, reader: { ...settings.reader, ...patch } });
  const updateTyping = (patch: Partial<Settings['typing']>) =>
    persist({ ...settings, typing: { ...settings.typing, ...patch } });
  const updateWindow = (patch: Partial<Settings['window']>) =>
    persist({ ...settings, window: { ...settings.window, ...patch } });
  const disguise = settings.disguise ?? {
    taskbarIcon: true,
    hideTaskbar: false,
    contactName: '文件传输助手',
    nickname: '我',
    avatar: '',
    sendOnEnter: true,
    floating: false,
    hoverReveal: false,
    showZone: { x: 0, y: 120, w: 8, h: 180 },
    hideZone: { x: 0, y: 420, w: 8, h: 180 },
    migrated: true,
  };
  const updateDisguise = (patch: Partial<Settings['disguise']>) =>
    persist({ ...settings, disguise: { ...disguise, ...patch } });
  const selectSkin = (next: string) => {
    const enableTyping = isChatSkin(next) && !settings.typing.enabled;
    persist({
      ...settings,
      skin: next,
      typing: { ...settings.typing, enabled: settings.typing.enabled || enableTyping },
    });
    void api.setSkin(next).then((w) => store.set({ window: w }));
    if (enableTyping) {
      void api.setTyping(true).then((w) => store.set({ window: w }));
    }
  };
  const updateZone = (key: 'showZone' | 'hideZone', patch: Partial<Settings['disguise']['showZone']>) => {
    updateDisguise({ [key]: { ...disguise[key], ...patch } } as Partial<Settings['disguise']>);
  };
  const pickZone = async (key: 'showZone' | 'hideZone') => {
    try {
      const pos = await api.cursorPosition();
      updateZone(key, {
        x: pos.x,
        y: pos.y,
        w: Math.max(8, disguise[key].w || 8),
        h: Math.max(60, disguise[key].h || 180),
      });
      setScreen({ width: pos.width, height: pos.height });
    } catch {
      /* ignore */
    }
  };
  const pickAvatar = async () => {
    try {
      const saved = await api.chooseAvatar();
      // A nickname edit may have a debounced full-document save in flight; that
      // snapshot predates the avatar and would clear it on the next tick.
      window.clearTimeout(saveTimer.current);
      store.set({ settings: saved });
      toast('头像已更新');
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };
  const clearAvatar = async () => {
    try {
      const saved = await api.clearAvatar();
      window.clearTimeout(saveTimer.current);
      store.set({ settings: saved });
    } catch (e) {
      toast(String((e as Error).message || e), 'error');
    }
  };
  const setHoverReveal = (on: boolean) => {
    if (!on) {
      updateDisguise({ hoverReveal: false });
      return;
    }
    const patch: Partial<Settings['disguise']> = { hoverReveal: true };
    if (disguise.showZone.w === 0 && disguise.showZone.h === 0) {
      patch.showZone = { x: 0, y: 120, w: 8, h: 180 };
    }
    if (disguise.hideZone.w === 0 && disguise.hideZone.h === 0) {
      patch.hideZone = { x: 0, y: 420, w: 8, h: 180 };
    }
    updateDisguise(patch);
  };

  return (
    <>
      <div className="page-head">
        <h1>设置</h1>
        <span className="sub">所有设置即时生效并自动保存</span>
      </div>

      <div className="settings">
        <div className="settings-nav">
          {NAV.map(([k, label]) => (
            <button key={k} className={section === k ? 'on' : ''} onClick={() => setSection(k)}>
              {label}
            </button>
          ))}
        </div>

        <div className="settings-panel">
          {section === 'appearance' ? (
            <>
              <section className="settings-section">
                <header>预设主题</header>
                <div className="theme-grid">
                  {themes.map((t) => (
                    <button
                      key={t.name}
                      className={`theme-swatch${t.name === settings.theme.name ? ' on' : ''}`}
                      style={{ background: t.background === 'transparent' ? '#20242e' : t.background, color: t.textColor }}
                      onClick={() =>
                        persist({
                          ...settings,
                          theme: {
                            ...t,
                            pattern: settings.theme.pattern,
                            patternOpacity: settings.theme.patternOpacity,
                          },
                        })
                      }
                    >
                      <span>{t.name}</span>
                      <span className="sample" style={{ fontFamily: t.fontFamily }}>
                        Aa 文
                      </span>
                    </button>
                  ))}
                </div>
              </section>

              <section className="settings-section">
                <header>界面承载</header>
                <div className="setting-row">
                  <div className="label">
                    <b>界面材质</b>
                    <small>同步改变标题栏、侧栏与书架的材质与光感；阅读器主题独立</small>
                  </div>
                </div>
                <div className="motif-choices">
                  {SHELF_MOTIFS.map((option) => (
                    <button
                      key={option.id}
                      type="button"
                      className={`motif-choice${shelfMotif === option.id ? ' on' : ''}`}
                      data-motif={option.id}
                      aria-pressed={shelfMotif === option.id}
                      onClick={() => setShelfMotif(option.id)}
                    >
                      <span className="motif-choice-glyph" aria-hidden="true">
                        {option.glyph}
                      </span>
                      <span className="motif-choice-text">
                        <b>{option.label}</b>
                        <small>{option.hint}</small>
                      </span>
                    </button>
                  ))}
                </div>
              </section>

              <section className="settings-section">
                <header>字体与排版</header>
                <div className="setting-row">
                  <div className="label">
                    <b>字体</b>
                    <small>优先使用本机已安装字体</small>
                  </div>
                  <div className="control">
                    <select
                      className="select"
                      value={settings.theme.fontFamily}
                      onChange={(e) => updateTheme({ fontFamily: e.target.value })}
                    >
                      {[settings.theme.fontFamily, ...fonts.filter((f) => f !== settings.theme.fontFamily)].map((f) => (
                        <option key={f} value={f}>
                          {f}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
                <Slider label="字号" hint="正文字号（px）" value={settings.theme.fontSize} min={12} max={40} step={1} onChange={(v) => updateTheme({ fontSize: v })} />
                <Slider label="行距" hint="行高倍数" value={settings.theme.lineHeight} min={1.2} max={2.6} step={0.05} onChange={(v) => updateTheme({ lineHeight: v })} display={settings.theme.lineHeight.toFixed(2)} />
                <Slider label="字距" hint="字间距（em）" value={settings.theme.letterSpacing} min={-0.02} max={0.2} step={0.01} onChange={(v) => updateTheme({ letterSpacing: v })} display={settings.theme.letterSpacing.toFixed(2)} />
                <Slider label="段距" hint="段落后间距（px）" value={settings.theme.paragraphSpacing} min={0} max={40} step={1} onChange={(v) => updateTheme({ paragraphSpacing: v })} />
                <Slider label="首行缩进" hint="以字符为单位" value={settings.theme.textIndent} min={0} max={4} step={1} onChange={(v) => updateTheme({ textIndent: v })} />
                <Slider label="版心宽度" hint="0 表示自适应；上下滚动时自动限制在约 920px" value={settings.theme.pageWidth} min={0} max={1200} step={20} onChange={(v) => updateTheme({ pageWidth: v })} display={settings.theme.pageWidth === 0 ? '自适应' : `${settings.theme.pageWidth}px`} />
                <Slider label="页边距" hint="上下左右留白（px）" value={settings.theme.marginX} min={0} max={120} step={4} onChange={(v) => updateTheme({ marginX: v, marginY: Math.round(v * 0.8) })} />
                <Row label="对齐方式" hint="两端对齐会调整字间距">
                  <div className="seg">
                    <button className={settings.theme.textAlign === 'justify' ? 'on' : ''} onClick={() => updateTheme({ textAlign: 'justify' })}>
                      两端对齐
                    </button>
                    <button className={settings.theme.textAlign === 'left' ? 'on' : ''} onClick={() => updateTheme({ textAlign: 'left' })}>
                      左对齐
                    </button>
                  </div>
                </Row>
                <Row label="分栏" hint="自动按窗口宽度；双栏会切到左右翻页；上下滚动固定单栏">
                  <div className="seg">
                    {(
                      [
                        [0, '自动'],
                        [1, '单栏'],
                        [2, '双栏'],
                      ] as [number, string][]
                    ).map(([n, label]) => (
                      <button
                        key={n}
                        className={settings.theme.columns === n ? 'on' : ''}
                        onClick={() => updateTheme({ columns: n })}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </Row>
                <Row label="行号" hint="在正文左侧显示连续行号（仅左右翻页）">
                  <Toggle on={settings.theme.lineNumbers} onChange={(v) => updateTheme({ lineNumbers: v })} />
                </Row>
              </section>

              <section className="settings-section">
                <header>颜色</header>
                <ColorRow label="文字颜色" value={settings.theme.textColor} onChange={(v) => updateTheme({ textColor: v })} />
                <ColorRow label="背景颜色" value={settings.theme.background} onChange={(v) => updateTheme({ background: v })} allowTransparent />
                <ColorRow label="纸面颜色" value={settings.theme.surface} onChange={(v) => updateTheme({ surface: v })} allowTransparent />
                <ColorRow label="标题颜色" value={settings.theme.headingColor} onChange={(v) => updateTheme({ headingColor: v })} />
                <ColorRow label="强调色" value={settings.theme.accentColor} onChange={(v) => updateTheme({ accentColor: v })} />
              </section>

              <section className="settings-section">
                <header>纸面底纹</header>
                <div className="setting-row">
                  <div className="label">
                    <b>中国纹样</b>
                    <small>叠在纸面上的低对比纹样，不影响正文识别；浓度可调</small>
                  </div>
                </div>
                <PatternGrid
                  value={settings.theme.pattern ?? ''}
                  onChange={(pattern) => updateTheme({ pattern })}
                  paper={settings.theme.surface}
                  ink={settings.theme.textColor}
                />
                {settings.theme.pattern ? (
                  <Slider
                    label="底纹浓度"
                    hint="数值越高，纹样越明显"
                    value={settings.theme.patternOpacity ?? DEFAULT_PATTERN_OPACITY}
                    min={0.04}
                    max={0.32}
                    step={0.01}
                    onChange={(v) => updateTheme({ patternOpacity: v })}
                    display={`${Math.round((settings.theme.patternOpacity ?? DEFAULT_PATTERN_OPACITY) * 100)}%`}
                  />
                ) : null}
              </section>
            </>
          ) : null}

          {section === 'reading' ? (
            <section className="settings-section">
              <header>阅读行为</header>
              <Row label="翻页方式" hint="左右翻页或连续上下滚动">
                <div className="seg">
                  <button
                    className={settings.reader.pageMode !== 'scroll' ? 'on' : ''}
                    onClick={() => setPageMode('page')}
                  >
                    左右翻页
                  </button>
                  <button
                    className={settings.reader.pageMode === 'scroll' ? 'on' : ''}
                    onClick={() => setPageMode('scroll')}
                  >
                    上下滚动
                  </button>
                </div>
              </Row>
              <Row label="翻页效果" hint="仅左右翻页生效">
                <select
                  className="select"
                  value={settings.reader.pageTurnEffect || 'slide'}
                  onChange={(e) =>
                    updateReader({ pageTurnEffect: e.target.value, pageTurnAnimation: e.target.value !== 'none' })
                  }
                >
                  <option value="slide">滑动</option>
                  <option value="fade">淡入</option>
                  <option value="none">无动画</option>
                </select>
              </Row>
              <Row label="滚轮翻页" hint="左右翻页模式下滚轮切换页码">
                <Toggle on={settings.reader.wheelTurn !== false} onChange={(v) => updateReader({ wheelTurn: v })} />
              </Row>
              <Row label="点击两侧翻页" hint="点击正文左右边缘切换页码">
                <Toggle on={settings.reader.clickTurn !== false} onChange={(v) => updateReader({ clickTurn: v })} />
              </Row>
              <Row label="显示滚动条" hint="仅上下滚动模式生效">
                <Toggle on={settings.reader.showScrollbar !== false} onChange={(v) => updateReader({ showScrollbar: v })} />
              </Row>
              <Row label="平滑滚动" hint="滚动、定位与章节跳转使用平滑动画">
                <Toggle on={settings.reader.smoothScroll !== false} onChange={(v) => updateReader({ smoothScroll: v })} />
              </Row>
              <Row label="自动隐藏工具栏" hint="鼠标停止后淡出顶栏与底栏">
                <Toggle on={settings.reader.autoHideBars} onChange={(v) => updateReader({ autoHideBars: v })} />
              </Row>
              <Row label="预取下一章" hint="临近章节末尾时后台缓存下一章">
                <Toggle on={settings.reader.prefetchNext} onChange={(v) => updateReader({ prefetchNext: v })} />
              </Row>
              <Row label="同步阅读进度" hint="把当前章节与位置写回微信读书">
                <Toggle on={settings.reader.reportProgress} onChange={(v) => updateReader({ reportProgress: v })} />
              </Row>
              <Row label="上报阅读时长" hint="仅在窗口活跃时累计，避免挂机">
                <Toggle on={settings.reader.reportReadTime} onChange={(v) => updateReader({ reportReadTime: v })} />
              </Row>
              <Slider
                label="上报间隔"
                hint="两次进度上报的最小间隔（秒）"
                value={settings.reader.reportInterval}
                min={15}
                max={180}
                step={5}
                onChange={(v) => updateReader({ reportInterval: v })}
                display={`${settings.reader.reportInterval}s`}
              />
            </section>
          ) : null}

          {section === 'disguise' ? (
            <>
              <section className="settings-section">
                <header>皮肤</header>
                <Row label="外观皮肤" hint="同一份正文，不同的外壳">
                  <div className="skin-choices">
                    {SKIN_OPTIONS.map((option) => (
                      <button
                        key={option.id}
                        className={settings.skin === option.id ? 'on' : ''}
                        onClick={() => selectSkin(option.id)}
                      >
                        <b>{option.label}</b>
                        <small>
                          {option.kind === 'im' ? '聊天伪装' : option.kind === 'document' ? '文档伪装' : '原始界面'}
                        </small>
                      </button>
                    ))}
                  </div>
                </Row>
                <Row label="窗口标题" hint="任务栏与标题栏显示的文字">
                  <input
                    className="input"
                    style={{ width: 220 }}
                    value={settings.windowTitle}
                    onChange={(e) => update({ windowTitle: e.target.value })}
                    onBlur={(e) => void api.setWindowTitle(e.target.value)}
                  />
                </Row>
                <Row label="任务栏图标" hint="随皮肤切换任务栏与 Alt+Tab 图标">
                  <Toggle on={disguise.taskbarIcon} onChange={(v) => updateDisguise({ taskbarIcon: v })} />
                </Row>
              </section>

              <section className="settings-section">
                <header>IM 聊天伪装</header>
                <Row label="聊天对象" hint="微信 / 钉钉 / 飞书皮肤显示的会话名">
                  <input
                    className="input"
                    style={{ width: 220 }}
                    value={disguise.contactName}
                    onChange={(e) => updateDisguise({ contactName: e.target.value })}
                  />
                </Row>
                <Row label="我的昵称" hint="聊天资料卡与消息头像显示的名字">
                  <input
                    className="input"
                    style={{ width: 220 }}
                    maxLength={24}
                    value={disguise.nickname}
                    placeholder="我"
                    onChange={(e) => updateDisguise({ nickname: e.target.value })}
                  />
                </Row>
                <Row label="我的头像" hint="PNG / JPG / GIF / WebP，不超过 8MB">
                  <div className="avatar-picker">
                    <span className="avatar-preview">
                      {disguise.avatar ? (
                        <img src={`/avatar/${encodeURIComponent(disguise.avatar)}`} alt="头像预览" />
                      ) : (
                        avatarInitial(disguise.nickname)
                      )}
                    </span>
                    <button className="btn" onClick={() => void pickAvatar()}>
                      选择图片
                    </button>
                    {disguise.avatar ? (
                      <button className="btn" onClick={() => void clearAvatar()}>
                        恢复默认
                      </button>
                    ) : null}
                  </div>
                </Row>
                <Row label="回车发送段落" hint="在聊天输入框按 Enter 发送当前段落">
                  <Toggle on={disguise.sendOnEnter} onChange={(v) => updateDisguise({ sendOnEnter: v })} />
                </Row>
              </section>

              <section className="settings-section">
                <header>老板键与任务栏</header>
                <Row label="老板键" hint={`${settings.hotkeys.panic || 'Ctrl+Alt+H'} 隐藏 / 恢复窗口`}>
                  <button className="btn" onClick={() => void api.hideWindow()}>
                    立即隐藏
                  </button>
                </Row>
                <Row label="隐藏任务栏按钮" hint="窗口仍可用老板键或悬停区域恢复">
                  <Toggle on={disguise.hideTaskbar} onChange={(v) => updateDisguise({ hideTaskbar: v })} />
                </Row>
              </section>

              <section className="settings-section">
                <header>悬浮透明窗口</header>
                <Row label="透明无边距" hint="无边框、透明背景、正文贴边">
                  <Toggle on={disguise.floating} onChange={(v) => updateDisguise({ floating: v })} />
                </Row>
                <Row label="悬停显示 / 隐藏" hint="进入显示区显示，进入隐藏区隐藏">
                  <Toggle on={disguise.hoverReveal} onChange={setHoverReveal} />
                </Row>
                <ZoneEditor
                  label="显示区域 A"
                  zone={disguise.showZone}
                  onChange={(patch) => updateZone('showZone', patch)}
                  onPick={() => void pickZone('showZone')}
                />
                <ZoneEditor
                  label="隐藏区域 B"
                  zone={disguise.hideZone}
                  onChange={(patch) => updateZone('hideZone', patch)}
                  onPick={() => void pickZone('hideZone')}
                />
                <Row label="屏幕" hint="坐标为物理像素；可用“取光标”直接拾取">
                  <span className="faint" style={{ fontSize: 12 }}>
                    {screen.width > 0 ? `${screen.width} × ${screen.height}` : '—'}
                  </span>
                </Row>
              </section>

              <section className="settings-section">
                <header>打字模式</header>
                <Row label="启用打字模式" hint="任意按键逐字显示正文，像在输入一样">
                  <Toggle
                    on={settings.typing.enabled}
                    onChange={(v) => {
                      updateTyping({ enabled: v });
                      void api.setTyping(v).then((w) => store.set({ window: w }));
                    }}
                  />
                </Row>
                <Row label="隐藏未输入文字" hint="未输入的部分保持占位但不可见">
                  <Toggle on={settings.typing.hideUnread} onChange={(v) => updateTyping({ hideUnread: v })} />
                </Row>
                <Row label="未输入文字淡显" hint="以极低透明度显示，便于预判下一行">
                  <Toggle on={settings.typing.faintUnread} onChange={(v) => updateTyping({ faintUnread: v })} />
                </Row>
                <Row label="回车跳到下一段" hint="关闭后回车也按普通字符处理">
                  <Toggle on={settings.typing.skipOnEnter} onChange={(v) => updateTyping({ skipOnEnter: v })} />
                </Row>
                <Row label="光标样式">
                  <div className="seg">
                    {(
                      [
                        ['bar', '竖线'],
                        ['block', '方块'],
                        ['underline', '下划线'],
                      ] as [string, string][]
                    ).map(([k, label]) => (
                      <button key={k} className={settings.typing.cursorShape === k ? 'on' : ''} onClick={() => updateTyping({ cursorShape: k })}>
                        {label}
                      </button>
                    ))}
                  </div>
                </Row>
              </section>
            </>
          ) : null}

          {section === 'window' ? (
            <section className="settings-section">
              <header>窗口</header>
              <Row label="窗口置顶" hint="始终显示在其他窗口之上">
                <Toggle
                  on={settings.window.alwaysOnTop}
                  onChange={(v) => {
                    updateWindow({ alwaysOnTop: v });
                    void api.setAlwaysOnTop(v).then((w) => store.set({ window: w }));
                  }}
                />
              </Row>
              <Row label="鼠标穿透" hint="点击穿透到下层窗口；用 Ctrl+Alt+C 关闭">
                <Toggle
                  on={settings.window.clickThrough}
                  onChange={(v) => {
                    updateWindow({ clickThrough: v });
                    void api.setClickThrough(v).then((w) => store.set({ window: w }));
                  }}
                />
              </Row>
              <Row label="记住窗口位置" hint="下次启动恢复到上次的位置与大小">
                <Toggle on={settings.window.remembered} onChange={(v) => updateWindow({ remembered: v })} />
              </Row>
              <Slider
                label="窗口不透明度"
                hint="整体透明度；透明背景下可当悬浮阅读器使用"
                value={Math.round(settings.window.opacity * 100)}
                min={30}
                max={100}
                step={1}
                onChange={(v) => {
                  updateWindow({ opacity: v / 100 });
                  void api.setOpacity(v / 100).then((w) => store.set({ window: w }));
                }}
                display={`${Math.round(settings.window.opacity * 100)}%`}
              />
              <Slider
                label="窗体圆角"
                hint="窗口四角弧度；0 为直角，越大越圆，可与背景窗口融合"
                value={settings.window.radius}
                min={0}
                max={40}
                step={1}
                onChange={(v) => {
                  updateWindow({ radius: v });
                  void api.setWindowRadius(v);
                }}
                display={`${settings.window.radius}px`}
              />
              <Row label="快捷键" hint="全局快捷键修改后需要重新注册">
                <button className="btn" onClick={() => void api.registerHotkeys().then(() => toast('已重新注册全局快捷键', 'success')).catch((e: unknown) => toast(String((e as Error).message || e), 'error'))}>
                  重新注册
                </button>
              </Row>
            </section>
          ) : null}

          {section === 'hotkeys' ? (
            <section className="settings-section">
              <header>快捷键</header>
              <div className="setting-row">
                <div className="label">
                  <b>翻页</b>
                  <small>左右翻页 ←/→ 翻页；上下滚动 ↑/↓ 滚动、←/→ 切章</small>
                </div>
                <div className="control">
                  <span className="kbd">←</span>
                  <span className="kbd">→</span>
                  <span className="kbd">↑</span>
                  <span className="kbd">↓</span>
                  <span className="kbd">Space</span>
                  <span className="kbd">PgUp</span>
                  <span className="kbd">PgDn</span>
                </div>
              </div>
              <div className="setting-row">
                <div className="label">
                  <b>章节</b>
                  <small>上一章 / 下一章</small>
                </div>
                <div className="control">
                  <span className="kbd">Ctrl</span>+<span className="kbd">←</span>
                  <span className="kbd">Ctrl</span>+<span className="kbd">→</span>
                </div>
              </div>
              {Object.entries(settings.hotkeys).map(([name, acc]) => (
                <div className="setting-row" key={name}>
                  <div className="label">
                    <b>{hotkeyLabel(name)}</b>
                    <small>{hotkeyHint(name)}</small>
                  </div>
                  <div className="control">
                    <input
                      className="input mono"
                      style={{ width: 150, textAlign: 'center' }}
                      value={acc}
                      onChange={(e) => update({ hotkeys: { ...settings.hotkeys, [name]: e.target.value } })}
                    />
                  </div>
                </div>
              ))}
            </section>
          ) : null}

          {section === 'about' ? (
            <section className="settings-section">
              <header>关于</header>
              <div className="setting-row">
                <div className="label">
                  <b>my-weread-desk</b>
                  <small>非官方桌面客户端 · 仅供个人学习研究</small>
                </div>
                <div className="control mono faint">v{version || '0.1.0'}</div>
              </div>
              <div className="setting-row">
                <div className="label">
                  <b>数据目录</b>
                  <small className="mono">{dataDir}</small>
                </div>
                <div className="control">
                  <button className="btn" onClick={() => void api.openDataDir()}>
                    <Icon name="folderOpen" size={14} /> 打开
                  </button>
                </div>
              </div>
              <div className="setting-row">
                <div className="label">
                  <b>重置设置</b>
                  <small>恢复默认外观与行为，不影响账号与缓存</small>
                </div>
                <div className="control">
                  <button
                    className="btn btn-danger"
                    onClick={() => {
                      if (!window.confirm('恢复默认设置？')) return;
                      void api.resetSettings().then((s) => {
                        store.set({ settings: s });
                        toast('已恢复默认设置', 'success');
                      });
                    }}
                  >
                    重置
                  </button>
                </div>
              </div>
              <div className="setting-row">
                <div className="label">
                  <b>退出登录</b>
                  <small>保留本机缓存与设备指纹</small>
                </div>
                <div className="control">
                  <button
                    className="btn btn-danger"
                    onClick={() => {
                      if (!window.confirm('退出登录？')) return;
                      void api.logout().then(() => store.set({ auth: { loggedIn: false, hasApiKey: false }, books: [] }));
                    }}
                  >
                    退出
                  </button>
                </div>
              </div>
            </section>
          ) : null}
        </div>
      </div>
    </>
  );
}

function Row({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="setting-row">
      <div className="label">
        <b>{label}</b>
        {hint ? <small>{hint}</small> : null}
      </div>
      <div className="control">{children}</div>
    </div>
  );
}

function Slider({
  label,
  hint,
  value,
  min,
  max,
  step,
  onChange,
  display,
}: {
  label: string;
  hint?: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
  display?: string;
}) {
  return (
    <div className="setting-row">
      <div className="label">
        <b>{label}</b>
        {hint ? <small>{hint}</small> : null}
      </div>
      <div className="control">
        <input
          className="range"
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
        />
        <span className="mono faint" style={{ width: 58, textAlign: 'right', fontSize: 12 }}>
          {display ?? value}
        </span>
      </div>
    </div>
  );
}

function ColorRow({
  label,
  value,
  onChange,
  allowTransparent,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  allowTransparent?: boolean;
}) {
  const hex = /^#[0-9a-f]{6}$/i.test(value) ? value : '#ffffff';
  return (
    <div className="setting-row">
      <div className="label">
        <b>{label}</b>
        <small className="mono">{value}</small>
      </div>
      <div className="control">
        <input className="color-input" type="color" value={hex} onChange={(e) => onChange(e.target.value)} />
        {allowTransparent ? (
          <button className="btn btn-ghost" style={{ fontSize: 11.5 }} onClick={() => onChange('transparent')}>
            透明
          </button>
        ) : null}
      </div>
    </div>
  );
}

function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return <button className={`switch${on ? ' on' : ''}`} onClick={() => onChange(!on)} aria-label="切换" />;
}

function hotkeyLabel(name: string): string {
  const map: Record<string, string> = {
    zen: '禅模式',
    toggleTop: '窗口置顶',
    typing: '打字模式',
    panic: '紧急隐藏',
    cycleSkin: '切换皮肤',
    clickThrough: '鼠标穿透',
  };
  return map[name] ?? name;
}

function hotkeyHint(name: string): string {
  const map: Record<string, string> = {
    zen: '隐藏所有界面元素，仅保留正文',
    toggleTop: '窗口未聚焦时也能切换',
    typing: '任意按键逐字显示',
    panic: '立刻隐藏窗口',
    cycleSkin: '纯净 → Word → VS Code → 微信 → 钉钉 → 飞书',
    clickThrough: '点击穿透到下层窗口',
  };
  return map[name] ?? '';
}

function ZoneEditor({
  label,
  zone,
  onChange,
  onPick,
}: {
  label: string;
  zone: { x: number; y: number; w: number; h: number };
  onChange: (patch: Partial<{ x: number; y: number; w: number; h: number }>) => void;
  onPick: () => void;
}) {
  const fields: Array<['x' | 'y' | 'w' | 'h', string]> = [
    ['x', 'X'],
    ['y', 'Y'],
    ['w', 'W'],
    ['h', 'H'],
  ];
  return (
    <div className="zone-editor">
      <span className="zone-label">{label}</span>
      {fields.map(([key, text]) => (
        <label key={key}>
          <span>{text}</span>
          <input
            className="input"
            type="number"
            value={zone[key]}
            onChange={(e) =>
              onChange({ [key]: Number(e.target.value) } as Partial<{ x: number; y: number; w: number; h: number }>)
            }
          />
        </label>
      ))}
      <button className="btn btn-ghost" onClick={onPick}>
        取光标
      </button>
    </div>
  );
}
