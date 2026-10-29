// Inline stroke icons on the 24x24 grid (stroke-width 2, round joins), matching
// the iconfont/iconpark linear style used across this project.
import type { ReactElement, SVGProps } from 'react';

const base: SVGProps<SVGSVGElement> = {
  width: 18,
  height: 18,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
  'aria-hidden': true,
};

export type IconName =
  | 'book' | 'search' | 'chart' | 'settings' | 'at' | 'pin' | 'unpin'
  | 'zen' | 'exitZen' | 'minimise' | 'maximise' | 'close' | 'left' | 'right'
  | 'list' | 'palette' | 'text' | 'moon' | 'sun' | 'download' | 'refresh'
  | 'more' | 'keyboard' | 'columns' | 'numbers' | 'eye' | 'eyeOff' | 'check'
  | 'code' | 'file' | 'folder' | 'chevronDown' | 'chevronRight' | 'star'
  | 'clock' | 'type' | 'sparkle' | 'folderOpen' | 'lock' | 'external';

const paths: Record<IconName, ReactElement> = {
  book: <><path d="M4 5a2 2 0 0 1 2-2h13v18H6a2 2 0 0 1-2-2z" /><path d="M8 3v18" /></>,
  search: <><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></>,
  chart: <><path d="M4 20V10M10 20V4M16 20v-7M22 20H2" /></>,
  settings: <><circle cx="12" cy="12" r="3" /><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1" /></>,
  at: <><circle cx="12" cy="12" r="4" /><path d="M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-3.9 7.9" /></>,
  pin: <><path d="M9 4h6l-1 6 3 3v2H7v-2l3-3z" /><path d="M12 15v6" /></>,
  unpin: <><path d="M9 4h6l-1 6 3 3v2H7v-2l3-3z" /><path d="M12 15v6M3 3l18 18" /></>,
  zen: <><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5" /></>,
  exitZen: <><path d="M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5" /></>,
  minimise: <path d="M5 12h14" />,
  maximise: <rect x="4" y="4" width="16" height="16" rx="2" />,
  close: <path d="M6 6l12 12M18 6 6 18" />,
  left: <path d="m15 5-7 7 7 7" />,
  right: <path d="m9 5 7 7-7 7" />,
  list: <><path d="M8 6h13M8 12h13M8 18h13" /><path d="M3 6h.01M3 12h.01M3 18h.01" /></>,
  palette: <><path d="M12 3a9 9 0 1 0 0 18c1.7 0 2-1 1.5-2s0-2 1.5-2H18a3 3 0 0 0 3-3 9 9 0 0 0-9-8z" /><circle cx="8" cy="10" r="1" /><circle cx="12" cy="7.5" r="1" /><circle cx="16" cy="10" r="1" /></>,
  text: <><path d="M4 6h16M12 6v13M9 19h6" /></>,
  moon: <path d="M20 14a8 8 0 1 1-9.9-9.9A7 7 0 0 0 20 14z" />,
  sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4" /></>,
  download: <><path d="M12 3v12M7 10l5 5 5-5" /><path d="M4 19h16" /></>,
  refresh: <><path d="M3 12a9 9 0 0 1 15.5-6.2M21 12a9 9 0 0 1-15.5 6.2" /><path d="M18 3v5h-5M6 21v-5h5" /></>,
  more: <><circle cx="5" cy="12" r="1.2" /><circle cx="12" cy="12" r="1.2" /><circle cx="19" cy="12" r="1.2" /></>,
  keyboard: <><rect x="2" y="6" width="20" height="12" rx="2" /><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10" /></>,
  columns: <><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M12 4v16" /></>,
  numbers: <><path d="M5 5v14M10 5v14M3 9h4M8 15h4" /><path d="M15 6h6M15 12h6M15 18h6" /></>,
  eye: <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" /><circle cx="12" cy="12" r="3" /></>,
  eyeOff: <><path d="M3 3l18 18" /><path d="M10.6 5.2A10.9 10.9 0 0 1 12 5c6.5 0 10 7 10 7a17 17 0 0 1-3.2 4.2M6.6 6.6A17 17 0 0 0 2 12s3.5 7 10 7a10.9 10.9 0 0 0 4.2-.9" /><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" /></>,
  check: <path d="m4 12 5 5L20 6" />,
  code: <><path d="m8 8-5 4 5 4M16 8l5 4-5 4M13 4l-2 16" /></>,
  file: <><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" /><path d="M14 3v5h5" /></>,
  folder: <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />,
  folderOpen: <><path d="M3 8a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v1H3z" /><path d="M3 11h18l-2 8H5z" /></>,
  chevronDown: <path d="m6 9 6 6 6-6" />,
  chevronRight: <path d="m9 6 6 6-6 6" />,
  star: <path d="m12 3 2.8 6 6.2.9-4.5 4.4 1 6.2-5.5-3-5.5 3 1-6.2L3 10.9 9.2 9z" />,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  type: <><path d="M3 7V5h10v2M8 5v14M5 19h6" /><path d="M14 12h7M17.5 12v7M15 19h5" /></>,
  sparkle: <><path d="M12 3l1.6 4.4L18 9l-4.4 1.6L12 15l-1.6-4.4L6 9l4.4-1.6z" /><path d="M18 15l.8 2.2L21 18l-2.2.8L18 21l-.8-2.2L15 18l2.2-.8z" /></>,
  lock: <><rect x="4" y="10" width="16" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></>,
  external: <><path d="M14 4h6v6M20 4l-9 9" /><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" /></>,
};

export function Icon({ name, size = 18, ...rest }: { name: IconName; size?: number } & SVGProps<SVGSVGElement>) {
  return (
    <svg {...base} width={size} height={size} {...rest}>
      {paths[name]}
    </svg>
  );
}
