import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';

import { Icon } from '../icons';
import type { IconName } from '../icons';
import type { ShelfGroup } from '../types';

export const UNGROUPED = '__ungrouped__';

export type ShelfFilter = 'all' | 'reading' | 'finished' | 'mp';
export type ShelfSort = 'recent' | 'added' | 'title';
export type SortDir = 'asc' | 'desc';
export type ShelfLayout = 'grid' | 'list';

export const FILTERS: [ShelfFilter, string][] = [
  ['all', '全部'],
  ['reading', '在读'],
  ['finished', '读完'],
  ['mp', '公众号'],
];

export const SORTS: [ShelfSort, string][] = [
  ['recent', '最近阅读'],
  ['added', '加入书架'],
  ['title', '书名'],
];

export function naturalDirection(sort: ShelfSort): SortDir {
  return sort === 'title' ? 'asc' : 'desc';
}

interface HeaderProps {
  query: string;
  onQuery: (value: string) => void;
  filter: ShelfFilter;
  onFilter: (value: ShelfFilter) => void;
  folders: (ShelfGroup & { count: number })[];
  ungroupedCount: number;
  totalCount: number;
  resultCount: number;
  folderId: string;
  onFolder: (value: string) => void;
  manager: boolean;
  onManageFolders: () => void;
  onExitManager: () => void;
  sort: ShelfSort;
  onSort: (value: ShelfSort) => void;
  dir: SortDir;
  onDir: (value: SortDir) => void;
  layout: ShelfLayout;
  onLayout: (value: ShelfLayout) => void;
  onOpenStore: () => void;
}

export function ShelfHeader({
  query,
  onQuery,
  filter,
  onFilter,
  folders,
  ungroupedCount,
  totalCount,
  resultCount,
  folderId,
  onFolder,
  manager,
  onManageFolders,
  onExitManager,
  sort,
  onSort,
  dir,
  onDir,
  layout,
  onLayout,
  onOpenStore,
}: HeaderProps) {
  const folderLabel =
    folderId === UNGROUPED
      ? '未分组'
      : folderId
        ? folders.find((folder) => folder.id === folderId)?.name ?? '文件夹'
        : '文件夹';
  const sortLabel = SORTS.find(([key]) => key === sort)?.[1] ?? '最近阅读';
  const hasFilter = Boolean(folderId) || query.trim().length > 0;

  return (
    <header className="shelf-head">
      <div className="shelf-searchbar">
        <div className="shelf-searchbox">
          <Icon name="search" size={16} className="shelf-search-icon" />
          <input
            value={query}
            onChange={(e) => onQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                onQuery('');
                e.currentTarget.blur();
              }
            }}
            aria-label={manager ? '筛选文件夹名称' : '在书架中搜索书名或作者'}
            placeholder={manager ? '筛选文件夹名称' : '搜索书架里的书名、作者'}
            spellCheck={false}
          />
          {query ? (
            <button
              type="button"
              className="shelf-search-clear"
              onClick={() => onQuery('')}
              aria-label="清除搜索"
              title="清除搜索"
            >
              <Icon name="close" size={13} />
            </button>
          ) : (
            <span className="shelf-search-hint" aria-hidden="true">
              Esc 清空
            </span>
          )}
        </div>
        <button
          type="button"
          className="shelf-store"
          onClick={onOpenStore}
          title="打开微信读书书城搜索"
        >
          <Icon name="book" size={14} />
          <span>书城</span>
        </button>
      </div>

      <div className="shelf-controls">
        {manager ? (
          <button type="button" className="shelf-back" onClick={onExitManager}>
            <Icon name="left" size={14} /> 返回书架
          </button>
        ) : (
          <div className="shelf-tabs" role="group" aria-label="书架分类">
            {FILTERS.map(([key, label]) => (
              <button
                key={key}
                type="button"
                className={filter === key ? 'on' : ''}
                aria-pressed={filter === key}
                onClick={() => onFilter(key)}
              >
                {label}
              </button>
            ))}
          </div>
        )}

        {manager ? null : (
          <>
            <MenuButton icon="folder" label={folderLabel} active={Boolean(folderId)} width={214}>
              {(close) => (
                <>
                  <div className="shelf-menu-section">切换文件夹</div>
                  <button
                    type="button"
                    className={`shelf-menu-item${folderId === '' ? ' on' : ''}`}
                    onClick={() => {
                      onFolder('');
                      close();
                    }}
                  >
                    <Icon name={folderId === '' ? 'check' : 'book'} size={14} />
                    <span className="grow">全部书籍</span>
                    <span className="shelf-menu-count">{totalCount}</span>
                  </button>
                  <button
                    type="button"
                    className={`shelf-menu-item${folderId === UNGROUPED ? ' on' : ''}`}
                    onClick={() => {
                      onFolder(UNGROUPED);
                      close();
                    }}
                  >
                    <Icon name={folderId === UNGROUPED ? 'check' : 'folder'} size={14} />
                    <span className="grow">未分组</span>
                    <span className="shelf-menu-count">{ungroupedCount}</span>
                  </button>
                  {folders.length > 0 ? <div className="shelf-menu-divider" /> : null}
                  {folders.map((folder) => (
                    <button
                      key={folder.id}
                      type="button"
                      className={`shelf-menu-item${folderId === folder.id ? ' on' : ''}`}
                      onClick={() => {
                        onFolder(folder.id);
                        close();
                      }}
                    >
                      <Icon name={folderId === folder.id ? 'check' : 'folder'} size={14} />
                      <span className="grow truncate">{folder.name}</span>
                      <span className="shelf-menu-count">{folder.count}</span>
                    </button>
                  ))}
                  <div className="shelf-menu-divider" />
                  <button
                    type="button"
                    className="shelf-menu-item"
                    onClick={() => {
                      close();
                      onManageFolders();
                    }}
                  >
                    <Icon name="folderOpen" size={14} /> 管理文件夹
                  </button>
                </>
              )}
            </MenuButton>

            <MenuButton
              icon="numbers"
              label={sortLabel}
              active={false}
              width={208}
              trailing={<Icon name={dir === 'asc' ? 'arrowUp' : 'arrowDown'} size={13} />}
            >
              {(close) => (
                <>
                  <div className="shelf-menu-section">排序方式</div>
                  {SORTS.map(([key, label]) => (
                    <button
                      key={key}
                      type="button"
                      className={`shelf-menu-item${sort === key ? ' on' : ''}`}
                      onClick={() => {
                        onSort(key);
                        close();
                      }}
                    >
                      <Icon name={sort === key ? 'check' : 'chevronRight'} size={14} />
                      <span className="grow">{label}</span>
                    </button>
                  ))}
                  <div className="shelf-menu-divider" />
                  <div className="shelf-dir" role="group" aria-label="排序方向">
                    <button
                      type="button"
                      className={dir === 'asc' ? 'on' : ''}
                      aria-pressed={dir === 'asc'}
                      onClick={() => onDir('asc')}
                    >
                      <Icon name="arrowUp" size={13} /> 升序
                    </button>
                    <button
                      type="button"
                      className={dir === 'desc' ? 'on' : ''}
                      aria-pressed={dir === 'desc'}
                      onClick={() => onDir('desc')}
                    >
                      <Icon name="arrowDown" size={13} /> 降序
                    </button>
                  </div>
                </>
              )}
            </MenuButton>
          </>
        )}

        <span className="shelf-controls-gap" />

        {manager ? null : (
          <div className="shelf-layout" role="group" aria-label="书架排列">
            <button
              type="button"
              className={layout === 'grid' ? 'on' : ''}
              onClick={() => onLayout('grid')}
              title="封面网格"
              aria-pressed={layout === 'grid'}
            >
              <Icon name="columns" size={15} />
            </button>
            <button
              type="button"
              className={layout === 'list' ? 'on' : ''}
              onClick={() => onLayout('list')}
              title="列表"
              aria-pressed={layout === 'list'}
            >
              <Icon name="list" size={15} />
            </button>
          </div>
        )}
      </div>

      <div className="shelf-status">
        <span>
          {manager
            ? `管理文件夹 · ${folders.length} 个`
            : hasFilter
              ? `共 ${totalCount} 本 · 当前 ${resultCount} 本`
              : `共 ${totalCount} 本`}
        </span>
        {folderId ? (
          <button
            type="button"
            className="shelf-status-chip"
            onClick={() => onFolder('')}
            title="清除文件夹筛选"
          >
            <Icon name="folder" size={12} />
            {folderLabel}
            <Icon name="close" size={11} />
          </button>
        ) : null}
        {query.trim() ? (
          <button
            type="button"
            className="shelf-status-chip"
            onClick={() => onQuery('')}
            title="清除搜索"
          >
            <Icon name="search" size={12} />
            {query.trim()}
            <Icon name="close" size={11} />
          </button>
        ) : null}
      </div>
    </header>
  );
}

function MenuButton({
  icon,
  label,
  active,
  width,
  trailing,
  children,
}: {
  icon: IconName;
  label: string;
  active: boolean;
  width: number;
  trailing?: ReactNode;
  children: (close: () => void) => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const close = () => setOpen(false);

  return (
    <div className={`shelf-menu${open ? ' open' : ''}`} ref={ref}>
      <button
        type="button"
        className={`shelf-select${active || open ? ' active' : ''}`}
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <Icon name={icon} size={14} />
        <span className="shelf-select-label">{label}</span>
        {trailing ?? null}
        <Icon name="chevronDown" size={12} className="shelf-select-caret" />
      </button>
      {open ? (
        <div className="shelf-menu-pop" role="menu" style={{ minWidth: width }}>
          {children(close)}
        </div>
      ) : null}
    </div>
  );
}
