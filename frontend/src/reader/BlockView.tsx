import { memo } from 'react';
import type { JSX } from 'react';

import type { Block } from '../types';

export interface BlockMetrics {
  start: number;
  length: number;
}

export function measureBlocks(blocks: Block[]): { metrics: BlockMetrics[]; total: number } {
  let acc = 0;
  const metrics = blocks.map((b) => {
    const len = blockTextLength(b);
    const start = acc;
    acc += len;
    return { start, length: len };
  });
  return { metrics, total: acc };
}

export function blockTextLength(b: Block): number {
  if (b.kind === 'image' || b.kind === 'divider') return 0;
  return Array.from(b.text ?? '').length;
}

interface Props {
  block: Block;
  metric: BlockMetrics;
  /** null disables typing; otherwise the number of runes revealed. */
  reveal: number | null;
  caretClass: string;
  caretRef: React.RefObject<HTMLSpanElement | null>;
  showCaret: boolean;
  faintUnread: boolean;
}

function split(text: string, show: number): [string, string] {
  const chars = Array.from(text);
  if (show >= chars.length) return [text, ''];
  if (show <= 0) return ['', text];
  return [chars.slice(0, show).join(''), chars.slice(show).join('')];
}

function TextContent({ block, metric, reveal, caretClass, caretRef, showCaret, faintUnread }: Props) {
  const text = block.text ?? '';
  if (reveal === null) return <>{text}</>;

  const show = Math.max(0, Math.min(metric.length, reveal - metric.start));
  const [shown, rest] = split(text, show);
  const hiddenClass = faintUnread ? 'tw-faint' : 'tw-hidden';
  const caret = showCaret ? <span ref={caretRef} className={`tw-caret ${caretClass}`} /> : null;

  return (
    <>
      {shown}
      {caret}
      {rest ? <span className={hiddenClass}>{rest}</span> : null}
    </>
  );
}

export const BlockView = memo(function BlockView(props: Props) {
  const { block, metric, reveal } = props;
  const hiddenByTyping = reveal !== null && reveal <= metric.start && metric.length > 0;

  switch (block.kind) {
    case 'heading': {
      const level = Math.min(6, Math.max(1, block.level ?? 2));
      const Tag = `h${level}` as keyof JSX.IntrinsicElements;
      return (
        <Tag
          className={`blk heading h${level} ${hiddenByTyping ? 'tw-hidden' : ''}`}
          data-k={`h${metric.start}`}
        >
          <TextContent {...props} />
        </Tag>
      );
    }
    case 'divider':
      return <div className="blk divider" />;
    case 'image':
      if (!block.src) {
        return (
          <div className="blk image missing">
            <span>{block.alt || '图片'}</span>
          </div>
        );
      }
      return (
        <div className={`blk image ${hiddenByTyping ? 'tw-hidden' : ''}`}>
          <img
            src={block.src}
            alt={block.alt || ''}
            loading="lazy"
            onLoad={() => window.dispatchEvent(new Event('reader:relayout'))}
            onError={() => window.dispatchEvent(new Event('reader:relayout'))}
          />
        </div>
      );
    case 'quote':
      return (
        <blockquote className="blk quote">
          <TextContent {...props} />
        </blockquote>
      );
    case 'pre':
      return (
        <pre className="blk pre">
          <TextContent {...props} />
        </pre>
      );
    default: {
      const cls = block.class ? ` ${block.class}` : '';
      return (
        <p className={`blk para${cls}`}>
          <TextContent {...props} />
        </p>
      );
    }
  }
});
