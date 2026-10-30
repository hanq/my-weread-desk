import { Icon } from '../icons';
import type { ReaderChromeProps } from '../views/ReaderView';

/**
 * Word disguise: the reading canvas sits on an A4 sheet inside a document
 * window. The ribbon menus are decorative; the right-hand cluster holds the
 * real reader controls so the user is never trapped in the disguise.
 */
export function wordSkin(p: ReaderChromeProps) {
  const pageLabel = p.mode === 'scroll' ? `滚动阅读 · ${p.percent.toFixed(0)}%` : `第 ${p.page + 1} 页，共 ${p.pages} 页`;
  return (
    <div className="skin-root word-skin" style={p.themeStyle}>
      <div className="word-titlebar">
        <div className="qat">
          <i>
            <Icon name="book" size={13} />
          </i>
          <i>
            <Icon name="file" size={13} />
          </i>
          <i>
            <Icon name="download" size={13} />
          </i>
        </div>
        <div className="doc-title truncate">{p.chapter.title || p.bookTitle} - Word</div>
        <div className="win">
          <button onClick={p.onToggleZen} title="禅模式">
            <Icon name={p.zen ? 'exitZen' : 'zen'} size={13} />
          </button>
          <button onClick={p.onTogglePin} title="置顶">
            <Icon name={p.pinned ? 'pin' : 'unpin'} size={13} />
          </button>
          <button onClick={p.onExit} className="close" title="关闭文档">
            <Icon name="close" size={13} />
          </button>
        </div>
      </div>

      <div className="word-ribbon">
        <div className="word-tabs">
          {['文件', '开始', '插入', '绘图', '设计', '布局', '引用', '审阅', '视图', '帮助'].map((t, i) => (
            <span key={t} className={i === 1 ? 'on' : ''}>
              {t}
            </span>
          ))}
        </div>
        <div className="word-tools">
          <span className="tool">
            <Icon name="file" size={14} />
          </span>
          <span className="tool">
            <Icon name="folderOpen" size={14} />
          </span>
          <span className="sep" />
          <span className="selectish">
            {p.typing ? '等线' : '宋体'} <Icon name="chevronDown" size={10} />
          </span>
          <span className="selectish" onClick={p.onToggleTheme} style={{ cursor: 'pointer' }}>
            {p.themeStyle['--r-size' as keyof typeof p.themeStyle] as string} <Icon name="chevronDown" size={10} />
          </span>
          <span className="tool big" title="加粗">
            <b>B</b>
          </span>
          <span className="tool big" title="斜体">
            <i>I</i>
          </span>
          <span className="tool big" title="下划线">
            <u>U</u>
          </span>
          <span className="sep" />
          <span className="tool" onClick={p.onToggleToc} title="目录" style={{ cursor: 'pointer' }}>
            <Icon name="list" size={14} />
          </span>
          <span className="tool" onClick={p.onPrev} title="上一页" style={{ cursor: 'pointer' }}>
            <Icon name="left" size={14} />
          </span>
          <span className="tool" onClick={p.onNext} title="下一页" style={{ cursor: 'pointer' }}>
            <Icon name="right" size={14} />
          </span>
          <span className="sep" />
          <span
            className="tool"
            onClick={p.onToggleTyping}
            title="打字模式"
            style={{ cursor: 'pointer', color: p.typing ? '#2b579a' : undefined }}
          >
            <Icon name="keyboard" size={14} />
          </span>
          <span className="tool" onClick={p.onToggleTheme} title="字体与主题" style={{ cursor: 'pointer' }}>
            <Icon name="palette" size={14} />
          </span>
        </div>
      </div>

      <div className="word-ruler" />

      <div className="word-body">
        <div className="word-paper">
          {p.children}
          <span className="word-page-badge">{p.mode === 'scroll' ? '滚动' : p.page + 1}</span>
        </div>
      </div>

      {p.showStatusBar ? (
        <div className="word-status">
          <span>{pageLabel}</span>
          <span>字数 {p.chapter.wordCount.toLocaleString('zh-CN')}</span>
          <span className="spacer" />
          <span>{p.chapter.title}</span>
          <span className="zoom">
            <Icon name="numbers" size={11} /> {p.percent.toFixed(0)}%
          </span>
          <span>中文(中国)</span>
          <span>
            <Icon name="sun" size={11} />
          </span>
        </div>
      ) : null}
    </div>
  );
}
