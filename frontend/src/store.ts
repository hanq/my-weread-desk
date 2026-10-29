// A 40-line external store. The app has one global store; views keep their own
// local state for transient UI (pagination cursors, typing reveal, ...).

import { useSyncExternalStore } from 'react';

import type { AuthState, Book, DownloadState, Settings, WindowState } from './types';

export type Route =
  | { name: 'shelf' }
  | { name: 'search' }
  | { name: 'stats' }
  | { name: 'settings' }
  | { name: 'mp' }
  | { name: 'detail'; bookId: string }
  | { name: 'reader'; bookId: string; chapterUid: number; fromStart?: boolean };

export interface AppState {
  settings: Settings | null;
  window: WindowState | null;
  auth: AuthState | null;
  books: Book[];
  downloads: Record<string, DownloadState>;
  route: Route;
  shelfState: 'idle' | 'loading' | 'error';
  shelfError: string;
  online: boolean;
  toast: { text: string; kind: 'info' | 'error' | 'success'; id: number } | null;
}

const initial: AppState = {
  settings: null,
  window: null,
  auth: null,
  books: [],
  downloads: {},
  route: { name: 'shelf' },
  shelfState: 'idle',
  shelfError: '',
  online: true,
  toast: null,
};

type Listener = () => void;

let state: AppState = initial;
const listeners = new Set<Listener>();

export const store = {
  get(): AppState {
    return state;
  },
  set(patch: Partial<AppState> | ((s: AppState) => Partial<AppState>)) {
    const next = typeof patch === 'function' ? patch(state) : patch;
    state = { ...state, ...next };
    listeners.forEach((l) => l());
  },
  subscribe(listener: Listener) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  },
};

// Expose the store in dev builds so the UI can be inspected from devtools.
if (import.meta.env.DEV) {
  (window as unknown as { __store?: unknown }).__store = store;
}

export function useAppState<U>(selector: (s: AppState) => U): U {
  return useSyncExternalStore(
    store.subscribe,
    () => selector(store.get()),
    () => selector(initial),
  );
}

export function useApp(): AppState {
  return useSyncExternalStore(store.subscribe, store.get, () => initial);
}

let toastId = 0;

export function toast(text: string, kind: 'info' | 'error' | 'success' = 'info') {
  toastId += 1;
  const id = toastId;
  store.set({ toast: { text, kind, id } });
  window.setTimeout(() => {
    if (store.get().toast?.id === id) store.set({ toast: null });
  }, kind === 'error' ? 5200 : 2800);
}

export function navigate(route: Route) {
  store.set({ route });
}
