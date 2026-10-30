import { memo, useState } from 'react';
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
  if (b.kind === 'image' || b.kind === 'divider' || b.kind === 'chapter') return 0;
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

function ReaderImage({ src, alt, hidden }: { src: string; alt: string; hidden: boolean }) {
  const [failed, setFailed] = useState(false);
  if (failed) {
    return (
      <div className={`blk image missing ${hidden ? 'tw-hidden' : ''}`}>
        <span>{alt ? `${alt}（加载失败）` : '图片加载失败'}</span>
      </div>
    );
  }
  return (
    <div className={`blk image ${hidden ? 'tw-hidden' : ''}`}>
      <img
        src={src}
        alt={alt}
        decoding="async"
        title="查看大图"
        role="button"
        tabIndex={0}
        aria-label={alt ? `查看大图：${alt}` : '查看大图'}
        onClick={(event) => {
          event.stopPropagation();
          window.dispatchEvent(new CustomEvent('reader:image-open', { detail: { src, alt } }));
        }}
        onKeyDown={(event) => {
          if (event.key !== 'Enter' && event.key !== ' ') return;
          event.preventDefault();
          event.stopPropagation();
          window.dispatchEvent(new CustomEvent('reader:image-open', { detail: { src, alt } }));
        }}
        onLoad={() => window.dispatchEvent(new Event('reader:relayout'))}
        onError={() => {
          setFailed(true);
          window.dispatchEvent(new Event('reader:relayout'));
        }}
      />
    </div>
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
    case 'chapter':
      // The title is rendered through CSS generated content so it does not
      // enter the text-node walk used for rune offsets.
      return <div className="blk chapter" data-title={block.text || '下一章'} />;
    case 'image':
      if (!block.src) {
        return (
          <div className="blk image missing">
            <span>{block.alt || '图片'}</span>
          </div>
        );
      }
      return (
        <ReaderImage
          key={block.src}
          src={block.src}
          alt={block.alt || ''}
          hidden={hiddenByTyping}
        />
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
