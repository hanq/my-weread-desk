import { useEffect, useState } from 'react';

import { api } from '../api';
import type { Theme } from '../types';

interface Props {
  theme: Theme;
  presets: Theme[];
  onChange: (patch: Partial<Theme>) => void;
  onPreset: (theme: Theme) => void;
  onClose: () => void;
}

function Row({ label, value, children }: { label: string; value?: string; children: React.ReactNode }) {
  return (
    <div className="line">
      <span>{label}</span>
      {children}
      {value ? <b className="val">{value}</b> : null}
    </div>
  );
}

export function ThemePanel({ theme, presets, onChange, onPreset, onClose }: Props) {
  const [fonts, setFonts] = useState<string[]>([]);

  useEffect(() => {
    let alive = true;
    api
      .fontFamilies()
      .then((list) => alive && setFonts(list))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className="theme-panel" onClick={(e) => e.stopPropagation()}>
      <div className="row">
        <h4 className="grow">外观</h4>
        <button className="icon-btn" onClick={onClose} aria-label="关闭">
          ×
        </button>
      </div>

      <div className="theme-swatches">
        {presets.map((p) => (
          <button
            key={p.name}
            className={`sw${p.name === theme.name ? ' on' : ''}`}
            style={{ background: p.background === 'transparent' ? '#20242e' : p.background, color: p.textColor }}
            onClick={() => onPreset(p)}
            title={p.name}
          >
            {p.name}
          </button>
        ))}
      </div>

      <h4>字体</h4>
      <Row label="字体">
        <select
          className="mini-input"
          value={theme.fontFamily}
          onChange={(e) => onChange({ fontFamily: e.target.value })}
        >
          {[theme.fontFamily, ...fonts.filter((f) => f !== theme.fontFamily)].map((f) => (
            <option key={f} value={f}>
              {f}
            </option>
          ))}
        </select>
      </Row>
      <Row label="字号" value={`${theme.fontSize}`}>
        <input
          type="range"
          min={12}
          max={38}
          step={1}
          value={theme.fontSize}
          onChange={(e) => onChange({ fontSize: Number(e.target.value) })}
        />
      </Row>
      <Row label="行距" value={theme.lineHeight.toFixed(2)}>
        <input
          type="range"
          min={1.2}
          max={2.6}
          step={0.05}
          value={theme.lineHeight}
          onChange={(e) => onChange({ lineHeight: Number(e.target.value) })}
        />
      </Row>
      <Row label="段距" value={`${theme.paragraphSpacing}`}>
        <input
          type="range"
          min={0}
          max={40}
          step={1}
          value={theme.paragraphSpacing}
          onChange={(e) => onChange({ paragraphSpacing: Number(e.target.value) })}
        />
      </Row>
      <Row label="字距" value={theme.letterSpacing.toFixed(2)}>
        <input
          type="range"
          min={-0.02}
          max={0.2}
          step={0.01}
          value={theme.letterSpacing}
          onChange={(e) => onChange({ letterSpacing: Number(e.target.value) })}
        />
      </Row>
      <Row label="缩进" value={`${theme.textIndent}字`}>
        <input
          type="range"
          min={0}
          max={4}
          step={1}
          value={theme.textIndent}
          onChange={(e) => onChange({ textIndent: Number(e.target.value) })}
        />
      </Row>

      <h4>颜色</h4>
      <Row label="字色">
        <input
          className="color-input"
          type="color"
          value={toHex(theme.textColor, '#2b2b2b')}
          onChange={(e) => onChange({ textColor: e.target.value })}
        />
        <input
          className="mini-input"
          value={theme.textColor}
          onChange={(e) => onChange({ textColor: e.target.value })}
        />
      </Row>
      <Row label="背景">
        <input
          className="color-input"
          type="color"
          value={toHex(theme.background, '#f5f2ea')}
          onChange={(e) => onChange({ background: e.target.value })}
        />
        <input
          className="mini-input"
          value={theme.background}
          onChange={(e) => onChange({ background: e.target.value })}
        />
      </Row>
      <Row label="纸面">
        <input
          className="color-input"
          type="color"
          value={toHex(theme.surface, '#fffdf8')}
          onChange={(e) => onChange({ surface: e.target.value })}
        />
        <button className="btn btn-ghost" style={{ padding: '3px 8px', fontSize: 11 }} onClick={() => onChange({ surface: 'transparent', background: 'transparent' })}>
          透明
        </button>
      </Row>

      <h4>版式</h4>
      <Row label="对齐">
        <select
          className="mini-input"
          value={theme.textAlign}
          onChange={(e) => onChange({ textAlign: e.target.value })}
        >
          <option value="justify">两端对齐</option>
          <option value="left">左对齐</option>
        </select>
      </Row>
      <Row label="版心">
        <input
          type="range"
          min={0}
          max={1100}
          step={20}
          value={theme.pageWidth}
          onChange={(e) => onChange({ pageWidth: Number(e.target.value) })}
        />
      </Row>
      <Row label="分栏">
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
              title={n === 0 ? '窗口足够宽时自动分为两栏' : undefined}
              className={theme.columns === n ? 'on' : ''}
              onClick={() => onChange({ columns: n })}
            >
              {label}
            </button>
          ))}
        </div>
      </Row>
      <Row label="行号">
        <button
          className={`switch${theme.lineNumbers ? ' on' : ''}`}
          onClick={() => onChange({ lineNumbers: !theme.lineNumbers })}
          aria-label="切换行号"
        />
      </Row>
    </div>
  );
}

function toHex(color: string, fallback: string): string {
  if (/^#[0-9a-f]{6}$/i.test(color)) return color;
  if (/^#[0-9a-f]{3}$/i.test(color)) {
    const [, r, g, b] = color;
    return `#${r}${r}${g}${g}${b}${b}`;
  }
  const m = color.match(/rgba?\((\d+)[,\s]+(\d+)[,\s]+(\d+)/);
  if (m) {
    const hex = (n: string) => Number(n).toString(16).padStart(2, '0');
    return `#${hex(m[1])}${hex(m[2])}${hex(m[3])}`;
  }
  return fallback;
}
