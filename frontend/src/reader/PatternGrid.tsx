import { READER_PATTERNS, patternMaskStyle } from './patterns';

interface Props {
  value: string;
  onChange: (id: string) => void;
  /** Paper colour used by the preview swatch. */
  paper?: string;
  /** Ink colour used by the preview swatch. */
  ink?: string;
}

// Shared by the reader theme panel and Settings → 外观. The swatches preview the
// pattern against the current paper/ink so the picker does not need its own
// colour scheme.
export function PatternGrid({ value, onChange, paper, ink }: Props) {
  const paperColor = paper && paper !== 'transparent' ? paper : '#20242e';
  const inkColor = ink && ink !== 'transparent' ? ink : '#f0e9db';

  return (
    <div className="pattern-grid">
      <button
        type="button"
        className={`pattern-option${!value ? ' on' : ''}`}
        onClick={() => onChange('')}
        title="不使用底纹"
      >
        <span className="pattern-swatch pattern-none" style={{ background: paperColor, color: inkColor }} />
        <span className="pattern-option-name">无</span>
      </button>
      {READER_PATTERNS.map((pattern) => (
        <button
          key={pattern.id}
          type="button"
          className={`pattern-option${value === pattern.id ? ' on' : ''}`}
          onClick={() => onChange(pattern.id)}
          title={`${pattern.label} · ${pattern.hint}`}
        >
          <span className="pattern-swatch" style={{ background: paperColor, color: inkColor }}>
            <span className="pattern-swatch-ink" style={patternMaskStyle(pattern)} />
          </span>
          <span className="pattern-option-name">{pattern.label}</span>
        </button>
      ))}
    </div>
  );
}
