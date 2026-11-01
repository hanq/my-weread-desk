import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

import { Icon } from '../icons';
import { navigate } from '../store';
import { SHELF_MOTIFS, shelfMotifLabel, setShelfMotif, useShelfMotif } from '../skins/shelfMotifs';

// The carrier switcher used to live as a half-disc on the shelf's right edge.
// It now lives in the window's top-left orb, so the shelf is only books and the
// shell has one obvious place for "appearance / navigation" controls.
export function TitleOrb() {
  const carrier = useShelfMotif();
  const [open, setOpen] = useState(false);
  const current = shelfMotifLabel(carrier);
  const glyph = SHELF_MOTIFS.find((option) => option.id === carrier)?.glyph ?? '';
  // Portal into the app shell rather than <body> so the panel inherits the
  // active carrier's CSS variables (chrome, ink, accent). <body> only has the
  // neutral defaults, which made the panel look detached from the theme.
  const portalTarget =
    typeof document === 'undefined' ? null : document.querySelector('.app-root') ?? document.body;

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <div className={`title-orb${open ? ' open' : ''}`}>
      <button
        type="button"
        className="title-orb-button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-label={`界面承载：${current}，点击切换`}
        title={`界面承载：${current}`}
      >
        <span className="title-orb-ring" aria-hidden="true" />
        <span className="title-orb-core" data-motif={carrier}>
          {glyph}
        </span>
      </button>

      {open
        ? createPortal(
            <>
              <div className="title-orb-backdrop" onClick={() => setOpen(false)} />
              <div className="title-orb-panel" role="dialog" aria-label="界面承载">
                <div className="title-orb-head">
                  <span>界面承载</span>
                  <b>{current}</b>
                </div>
                <div className="title-orb-grid">
                  {SHELF_MOTIFS.map((option) => (
                    <button
                      key={option.id}
                      type="button"
                      className={`title-orb-option${option.id === carrier ? ' on' : ''}`}
                      data-motif={option.id}
                      onClick={() => {
                        setShelfMotif(option.id);
                        setOpen(false);
                      }}
                      aria-pressed={option.id === carrier}
                    >
                      <span className="title-orb-glyph">{option.glyph}</span>
                      <span className="title-orb-label">{option.label}</span>
                      <span className="title-orb-hint">{option.hint}</span>
                    </button>
                  ))}
                </div>
                <button
                  type="button"
                  className="title-orb-home"
                  onClick={() => {
                    setOpen(false);
                    navigate({ name: 'shelf' });
                  }}
                >
                  <Icon name="book" size={14} /> 回到书架
                </button>
              </div>
            </>,
            portalTarget ?? document.body,
          )
        : null}
    </div>
  );
}
