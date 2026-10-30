import { useMemo } from 'react';

import type { Block, Theme, TypingSettings } from '../types';
import { BlockView, measureBlocks } from './BlockView';
import type { Geometry } from './layout';

interface Props {
  blocks: Block[];
  theme: Theme;
  geometry: Geometry;
  page: number;
  mode: 'page' | 'scroll';
  showScrollbar: boolean;
  clickTurn: boolean;
  animated: boolean;
  fading: boolean;
  gutter: boolean;
  typing: TypingSettings;
  reveal: number | null;
  caretRef: React.RefObject<HTMLSpanElement | null>;
  stageRef: React.RefObject<HTMLDivElement | null>;
  viewportRef: React.RefObject<HTMLDivElement | null>;
  trackRef: React.RefObject<HTMLDivElement | null>;
  onPrev?: () => void;
  onNext?: () => void;
  /** Skins draw their own frame, so the paper must not paint a surface. */
  transparentPaper?: boolean;
  /** Hides the gutter even when the theme asks for line numbers. */
  forceNoGutter?: boolean;
  /** Skins own the outer margins, so the stage can override its padding. */
  stagePadding?: string;
}

export function ReaderCanvas({
  blocks,
  theme,
  geometry,
  page,
  mode,
  showScrollbar,
  clickTurn,
  animated,
  fading,
  gutter,
  typing,
  reveal,
  caretRef,
  stageRef,
  viewportRef,
  trackRef,
  onPrev,
  onNext,
  transparentPaper,
  forceNoGutter,
  stagePadding,
}: Props) {
  const { metrics, total } = useMemo(() => measureBlocks(blocks), [blocks]);

  const caretBlock = useMemo(() => {
    if (reveal === null) return -1;
    for (let i = 0; i < metrics.length; i += 1) {
      const m = metrics[i];
      if (m.length === 0) continue;
      if (reveal > m.start && reveal <= m.start + m.length) return i;
    }
    if (reveal > 0 && reveal >= total) {
      for (let i = metrics.length - 1; i >= 0; i -= 1) {
        if (metrics[i].length > 0) return i;
      }
    }
    return -1;
  }, [metrics, reveal, total]);

  const showGutter = gutter && !forceNoGutter && mode === 'page';
  const scroll = mode === 'scroll';
  const lines = geometry.linesPerPage;
  const startLine = page * lines;
  const gutterNumbers = showGutter
    ? Array.from({ length: lines }, (_, i) => startLine + i + 1)
    : [];

  const caretClass = typing.cursorShape === 'block' ? 'block' : typing.cursorShape === 'underline' ? 'underline' : '';

  return (
    <div
      ref={stageRef}
      className={`reader-stage${animated ? '' : ' no-anim'}`}
      style={{ padding: stagePadding ?? `${theme.marginY}px ${theme.marginX}px` }}
    >
      {showGutter ? (
        <div className="gutter">
          {gutterNumbers.map((n) => (
            <span key={n}>{n}</span>
          ))}
        </div>
      ) : null}

      <div className={`paper${transparentPaper ? ' no-surface' : ''}${scroll ? ' scroll-paper' : ''}`}>
        <div
          className={`viewport${scroll ? ' scroll-mode' : ''}${scroll && !showScrollbar ? ' hide-scrollbar' : ''}`}
          ref={viewportRef}
        >
          <div
            className={`track${scroll ? ' scroll-track' : ''}${fading ? ' turn-fade' : ''}`}
            ref={trackRef}
            style={scroll ? undefined : { transform: `translateX(${-page * geometry.step}px)` }}
          >
            {blocks.map((b, i) => (
              <BlockView
                key={i}
                block={b}
                metric={metrics[i]}
                reveal={reveal}
                caretClass={caretClass}
                caretRef={caretRef}
                showCaret={reveal !== null && i === caretBlock && reveal !== null}
                faintUnread={typing.faintUnread}
              />
            ))}
          </div>
        </div>

        {!scroll && clickTurn ? (
          <>
            <button className="turn-zone left" aria-label="上一页" onClick={onPrev} tabIndex={-1} />
            <button className="turn-zone right" aria-label="下一页" onClick={onNext} tabIndex={-1} />
          </>
        ) : null}
      </div>
    </div>
  );
}
