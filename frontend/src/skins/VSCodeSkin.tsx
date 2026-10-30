import { Icon } from '../icons';
import type { ReaderChromeProps } from '../views/ReaderView';

/**
 * VS Code disguise: chapters become files in an explorer tree, the reading
 * canvas becomes the editor pane. Line numbers are forced on so the editor
 * looks like a real source file.
 */
export function vsCodeSkin(p: ReaderChromeProps) {
  // No hooks here: the skin is a plain render function called inside
  // ReaderView, so a hook would change the caller's hook order.
  const minimap = buildMinimap(p.chapter.wordCount);
  const language = p.chapter.title.endsWith('.md') ? 'Markdown' : 'Plain Text';

  return (
    <div className="skin-root vscode-skin" style={p.themeStyle}>
      <div className="vs-titlebar">
        <div className="menu">
          {['文件', '编辑', '选择', '查看', '转到', '运行', '终端', '帮助'].map((m) => (
            <span key={m}>{m}</span>
          ))}
        </div>
        <div className="center truncate">
          {p.chapter.title || 'untitled'} — {p.bookTitle}
        </div>
        <div className="win">
          <button onClick={p.onToggleZen} title="禅模式">
            <Icon name={p.zen ? 'exitZen' : 'zen'} size={13} />
          </button>
          <button onClick={p.onTogglePin} title="置顶">
            <Icon name={p.pinned ? 'pin' : 'unpin'} size={13} />
          </button>
          <button onClick={p.onExit} className="close" title="关闭编辑器">
            <Icon name="close" size={13} />
          </button>
        </div>
      </div>

      <div className="vs-main">
        <div className="vs-activity">
          <button className="on" title="资源管理器">
            <Icon name="file" size={21} />
          </button>
          <button title="搜索" onClick={p.onToggleToc}>
            <Icon name="search" size={21} />
          </button>
          <button title="大纲" onClick={p.onToggleToc}>
            <Icon name="list" size={21} />
          </button>
          <button title="外观" onClick={p.onToggleTheme}>
            <Icon name="palette" size={21} />
          </button>
          <button title="打字模式" onClick={p.onToggleTyping} style={{ color: p.typing ? '#fff' : undefined }}>
            <Icon name="keyboard" size={21} />
          </button>
        </div>

        <div className="vs-explorer">
          <header>
            <span>资源管理器</span>
            <span>···</span>
          </header>
          <div className="folder">
            <Icon name="chevronDown" size={11} /> {p.bookTitle || 'books'}
          </div>
          <div className="tree">
            {p.chapters.map((c) => (
              <div
                key={c.uid}
                className={`tree-item${c.uid === p.chapter.uid ? ' on' : ''}`}
                onClick={() => p.onSelectChapter(c.uid)}
                title={c.title}
              >
                <Icon name="file" size={12} />
                <span className="truncate">{(c.title || `chapter-${c.index}`).replace(/\s+/g, '-')}.txt</span>
              </div>
            ))}
          </div>
        </div>

        <div className="vs-editor">
          <div className="vs-tabs">
            <div className="vs-tab on">
              <span className="dot" />
              <span className="truncate">{(p.chapter.title || 'untitled').slice(0, 24)}.txt</span>
              <Icon name="close" size={11} />
            </div>
          </div>
          <div className="vs-breadcrumb">
            <span>{p.bookTitle}</span>
            <Icon name="chevronRight" size={10} />
            <span className="truncate">{p.chapter.title}</span>
            <Icon name="chevronRight" size={10} />
            <span className="cur">{p.mode === 'scroll' ? 'Scroll' : `Ln ${Math.max(1, (p.page + 1) * 24)}`}</span>
          </div>
          <div className="vs-editor-area">
            {p.children}
            <div className="vs-minimap" aria-hidden>
              {minimap.map((w, i) => (
                <i key={i} className={w > 0.72 ? 'c' : w > 0.5 ? 'w' : ''} style={{ width: `${Math.max(8, w * 100)}%` }} />
              ))}
            </div>
          </div>
          {p.showStatusBar ? (
            <div className="vs-status">
              <span>⎇ main*</span>
              <span>0 ⊗ 0 ⚠</span>
              <span className="spacer" />
              {p.mode === 'scroll' ? <span>Scroll</span> : <span>Ln {Math.max(1, (p.page + 1) * 24)}, Col 1</span>}
              <span>Spaces: 4</span>
              <span>UTF-8</span>
              <span>{language}</span>
              {p.mode === 'page' ? (
                <span>
                  {p.page + 1}/{p.pages}
                </span>
              ) : null}
              <span>{p.percent.toFixed(0)}%</span>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function buildMinimap(chars: number): number[] {
  const rows = 46;
  const seed = Math.max(1, chars);
  return Array.from({ length: rows }, (_, i) => {
    const v = Math.abs(Math.sin((i + 1) * 1.7 + seed * 0.0007));
    return 0.15 + v * 0.8;
  });
}
