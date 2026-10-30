import { useEffect, useLayoutEffect, useRef } from 'react';

import { avatarInitial, avatarSrc } from '../skins/registry';
import type { Block } from '../types';
import { BlockView } from './BlockView';
import type { BlockMetrics } from './BlockView';

interface Props {
  blocks: Block[];
  metrics: BlockMetrics[];
  reveal: number | null;
  caretRef: React.RefObject<HTMLSpanElement | null>;
  cursorShape: string;
  faintUnread: boolean;
  nickname: string;
  avatar: string;
  onScrollProgress: (fraction: number) => void;
  /** Scrolling up stops typing so the user can reread; scrolling down keeps it. */
  onCancelTyping: () => void;
  hasNextChapter: boolean;
  onNextChapter: () => void;
  onLoadNext: () => void;
  onLoadPrevious: () => Promise<boolean>;
}

/**
 * Chat transcript for the IM disguises. Blocks from adjacent chapters are
 * concatenated into the same transcript, so scrolling or typing across a
 * chapter boundary does not replace the conversation.
 */
export function ChatTranscript({
  blocks,
  metrics,
  reveal,
  caretRef,
  cursorShape,
  faintUnread,
  nickname,
  avatar,
  onScrollProgress,
  onCancelTyping,
  hasNextChapter,
  onNextChapter,
  onLoadNext,
  onLoadPrevious,
}: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const cancelRef = useRef(false);
  const prependSnapshotRef = useRef<{ height: number; top: number } | null>(null);
  const previousPendingRef = useRef(false);
  const userScrolledRef = useRef(false);
  const lastTopRef = useRef(0);

  let caretBlock = -1;
  if (reveal !== null) {
    for (let i = 0; i < metrics.length; i += 1) {
      const metric = metrics[i];
      if (metric.length === 0) continue;
      if (reveal > metric.start && reveal <= metric.start + metric.length) {
        caretBlock = i;
        break;
      }
    }
  }

  const caretClass = cursorShape === 'block' ? 'block' : cursorShape === 'underline' ? 'underline' : '';

  useEffect(() => {
    if (reveal === null) return;
    const caret = caretRef.current;
    const box = scrollRef.current;
    if (!caret || !box) return;
    const message = caret.closest('.chat-msg') as HTMLElement | null;
    if (!message) return;
    const top = message.offsetTop - box.clientHeight + message.offsetHeight + 20;
    box.scrollTo({ top: Math.max(0, top), behavior: 'smooth' });
  }, [reveal, caretRef]);

  useEffect(() => {
    // Re-arm the guard whenever typing stops. Programmatic caret scrolling does
    // not emit wheel/touch events, so only genuine user scrolling cancels it.
    if (reveal === null) cancelRef.current = false;
  }, [reveal]);

  // Prepending a chapter keeps the currently visible messages in place.
  useLayoutEffect(() => {
    const snapshot = prependSnapshotRef.current;
    if (!snapshot) return;
    prependSnapshotRef.current = null;
    const box = scrollRef.current;
    if (!box) return;
    const delta = box.scrollHeight - snapshot.height;
    if (delta !== 0) box.scrollTop = snapshot.top + delta;
  }, [blocks]);

  const cancelTypingOnScroll = () => {
    if (reveal === null || cancelRef.current) return;
    cancelRef.current = true;
    onCancelTyping();
  };

  const requestPrevious = async () => {
    if (previousPendingRef.current) return;
    const box = scrollRef.current;
    if (!box || box.scrollTop > 2) return;
    prependSnapshotRef.current = { height: box.scrollHeight, top: box.scrollTop };
    previousPendingRef.current = true;
    try {
      const added = await onLoadPrevious();
      if (!added) prependSnapshotRef.current = null;
    } catch {
      prependSnapshotRef.current = null;
    } finally {
      previousPendingRef.current = false;
    }
  };

  const onScroll = () => {
    const box = scrollRef.current;
    if (!box) return;
    const top = box.scrollTop;
    const goingUp = top < lastTopRef.current;
    lastTopRef.current = top;
    const max = box.scrollHeight - box.clientHeight;
    onScrollProgress(max > 0 ? top / max : 1);

    // While typing, the next chapter is loaded by the reveal progress. Once
    // typing is off, ordinary scrolling continues across chapter boundaries.
    if (reveal !== null || !userScrolledRef.current) return;
    if (!goingUp && max > 0 && top >= max - 48) onLoadNext();
    if (goingUp && top <= 48) void requestPrevious();
  };

  const onWheel = (event: React.WheelEvent) => {
    const box = scrollRef.current;
    if (!box) return;
    userScrolledRef.current = true;

    if (event.deltaY < 0) {
      // Scrolling up is the explicit "I want to reread" signal.
      cancelTypingOnScroll();
      if (box.scrollTop <= 2) void requestPrevious();
      return;
    }

    const max = box.scrollHeight - box.clientHeight;
    if (max <= 0 || box.scrollTop >= max - 2) onLoadNext();
  };

  const onTouchMove = () => {
    userScrolledRef.current = true;
    cancelTypingOnScroll();
  };

  const stamp = (index: number) => `${10 + Math.floor(index / 4)}:${String((index * 13) % 60).padStart(2, '0')}`;

  return (
    <div
      className="chat-transcript"
      ref={scrollRef}
      onScroll={onScroll}
      onWheel={onWheel}
      onTouchMove={onTouchMove}
      onPointerDown={() => {
        userScrolledRef.current = true;
      }}
    >
      <div className="chat-day">今天</div>
      {blocks.map((block, index) => {
        const metric = metrics[index];
        if (!metric) return null;

        if (block.kind === 'chapter') {
          // Chapter markers have zero rune length. Hide the marker until the
          // typing cursor reaches its boundary.
          if (reveal !== null && reveal < metric.start) return null;
          return (
            <div key={index} className="chat-chapter">
              {block.text}
            </div>
          );
        }

        if (reveal !== null && metric.length > 0 && reveal <= metric.start) return null;
        if (block.kind === 'divider') return <div key={index} className="chat-divider" />;
        if (block.kind === 'heading') {
          return (
            <div key={index} className="chat-system">
              {block.text}
            </div>
          );
        }
        return (
          <div key={index} className="chat-msg">
            <div className="chat-avatar">
              {avatar ? <img src={avatarSrc(avatar)} alt="" /> : avatarInitial(nickname)}
            </div>
            <div className="chat-msg-main">
              <div className="chat-bubble">
                <BlockView
                  block={block}
                  metric={metric}
                  reveal={reveal}
                  caretClass={caretClass}
                  caretRef={caretRef}
                  showCaret={reveal !== null && index === caretBlock}
                  faintUnread={faintUnread}
                />
              </div>
              <span className="chat-time">{stamp(index)}</span>
            </div>
          </div>
        );
      })}
      {hasNextChapter ? (
        <button className="chat-next" onClick={onNextChapter}>
          下一章
        </button>
      ) : null}
    </div>
  );
}
