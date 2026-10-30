import { useCallback, useEffect, useRef, useState } from 'react';

interface ImageDetail {
  src: string;
  alt?: string;
}

const MIN_SCALE = 1;
const MAX_SCALE = 8;
const EDGE_PADDING = 32;

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/**
 * Full-window image viewer shared by the plain reader and every disguise skin.
 * ReaderImage dispatches `reader:image-open`; this component owns the overlay so
 * Word/VS Code/IM modes do not need their own implementation.
 */
export function ImageLightbox() {
  const [open, setOpen] = useState(false);
  const [src, setSrc] = useState('');
  const [alt, setAlt] = useState('');
  const [scale, setScale] = useState(1);
  const [tx, setTx] = useState(0);
  const [ty, setTy] = useState(0);
  const [dragging, setDragging] = useState(false);

  const overlayRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const lastFocus = useRef<HTMLElement | null>(null);
  const dragRef = useRef<{ x: number; y: number; tx: number; ty: number } | null>(null);

  const close = useCallback(() => {
    setOpen(false);
    setDragging(false);
    dragRef.current = null;
    window.setTimeout(() => lastFocus.current?.focus?.(), 0);
  }, []);

  const clampPan = useCallback((x: number, y: number, nextScale: number): [number, number] => {
    const img = imgRef.current;
    const overlay = overlayRef.current;
    if (!img || !overlay) return [0, 0];
    const vw = Math.max(0, overlay.clientWidth - EDGE_PADDING * 2);
    const vh = Math.max(0, overlay.clientHeight - EDGE_PADDING * 2);
    const maxX = Math.max(0, (img.offsetWidth * nextScale - vw) / 2);
    const maxY = Math.max(0, (img.offsetHeight * nextScale - vh) / 2);
    return [clamp(x, -maxX, maxX), clamp(y, -maxY, maxY)];
  }, []);

  const zoomAt = useCallback(
    (requested: number, clientX?: number, clientY?: number) => {
      const overlay = overlayRef.current;
      if (!overlay) return;
      const next = clamp(requested, MIN_SCALE, MAX_SCALE);
      if (next === scale) return;
      if (next === 1) {
        setScale(1);
        setTx(0);
        setTy(0);
        return;
      }

      // Keep the point under the cursor stable while the image scales around
      // its center. tx/ty are translations in screen pixels.
      const rect = overlay.getBoundingClientRect();
      const cx = clientX ?? rect.left + rect.width / 2;
      const cy = clientY ?? rect.top + rect.height / 2;
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const ratio = next / scale;
      const [x, y] = clampPan(
        tx + (cx - centerX - tx) * (1 - ratio),
        ty + (cy - centerY - ty) * (1 - ratio),
        next,
      );
      setScale(next);
      setTx(x);
      setTy(y);
    },
    [clampPan, scale, tx, ty],
  );

  useEffect(() => {
    const onOpen = (event: Event) => {
      const detail = (event as CustomEvent<ImageDetail>).detail;
      if (!detail?.src) return;
      lastFocus.current = document.activeElement as HTMLElement | null;
      setSrc(detail.src);
      setAlt(detail.alt || '');
      setScale(1);
      setTx(0);
      setTy(0);
      setDragging(false);
      setOpen(true);
      window.setTimeout(() => closeRef.current?.focus(), 0);
    };
    window.addEventListener('reader:image-open', onOpen);
    return () => window.removeEventListener('reader:image-open', onOpen);
  }, []);

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        close();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', onKey);
    };
  }, [open, close]);

  useEffect(() => {
    if (!open) return;
    const overlay = overlayRef.current;
    if (!overlay) return;
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      event.stopPropagation();
      // Trackpads report small continuous deltas; exponential scaling keeps
      // both a mouse notch and a gentle two-finger gesture usable.
      const factor = Math.exp(-event.deltaY * 0.0018);
      zoomAt(scale * factor, event.clientX, event.clientY);
    };
    overlay.addEventListener('wheel', onWheel, { passive: false });
    return () => overlay.removeEventListener('wheel', onWheel);
  }, [open, scale, zoomAt]);

  const onPointerDown = (event: React.PointerEvent<HTMLImageElement>) => {
    if (event.button !== 0 || scale <= 1) return;
    event.preventDefault();
    event.stopPropagation();
    dragRef.current = { x: event.clientX, y: event.clientY, tx, ty };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const onPointerMove = (event: React.PointerEvent<HTMLImageElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    const [x, y] = clampPan(drag.tx + event.clientX - drag.x, drag.ty + event.clientY - drag.y, scale);
    setTx(x);
    setTy(y);
  };

  const endDrag = (event: React.PointerEvent<HTMLImageElement>) => {
    dragRef.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="image-lightbox"
      role="dialog"
      aria-modal="true"
      aria-label="图片查看器"
      onClick={(event) => {
        if (event.target === event.currentTarget) close();
      }}
    >
      <div className="image-lightbox-tools">
        <button type="button" onClick={() => zoomAt(scale / 1.25)} aria-label="缩小">
          −
        </button>
        <button
          type="button"
          className="level"
          onClick={() => {
            setScale(1);
            setTx(0);
            setTy(0);
          }}
          title="重置为适屏大小"
        >
          {Math.round(scale * 100)}%
        </button>
        <button type="button" onClick={() => zoomAt(scale * 1.25)} aria-label="放大">
          +
        </button>
        <button ref={closeRef} type="button" className="close" onClick={close} aria-label="关闭">
          ×
        </button>
      </div>

      <img
        ref={imgRef}
        src={src}
        alt={alt}
        draggable={false}
        className={dragging ? 'dragging' : ''}
        style={{
          transform: `translate(${tx}px, ${ty}px) scale(${scale})`,
          cursor: scale > 1 ? (dragging ? 'grabbing' : 'grab') : 'zoom-in',
        }}
        onClick={(event) => event.stopPropagation()}
        onDoubleClick={(event) => {
          event.stopPropagation();
          zoomAt(scale > 1 ? 1 : 2);
        }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onDragStart={(event) => event.preventDefault()}
      />

      {alt ? <div className="image-lightbox-caption">{alt}</div> : null}
      <div className="image-lightbox-hint">滚轮缩放 · 拖拽移动 · 双击复位 · Esc 关闭</div>
    </div>
  );
}
