import { Icon } from '../icons';
import { avatarInitial, avatarSrc } from './registry';
import type { ReaderChromeProps } from '../views/ReaderView';

type Brand = 'wechat' | 'dingtalk' | 'feishu' | 'wecom';

const BRANDS: Record<Brand, { name: string; accent: string; bg: string; header: string; side: string; bubble: string }> = {
  wechat: { name: '微信', accent: '#07c160', bg: '#ededed', header: '#ededed', side: '#f6f6f6', bubble: '#95ec69' },
  dingtalk: { name: '钉钉', accent: '#0089ff', bg: '#f2f3f5', header: '#ffffff', side: '#f7f8fa', bubble: '#d6ebff' },
  feishu: { name: '飞书', accent: '#3370ff', bg: '#f5f6f7', header: '#ffffff', side: '#fafbfc', bubble: '#e1eaff' },
  wecom: { name: '企业微信', accent: '#0082ef', bg: '#eef2f6', header: '#ffffff', side: '#f7f9fb', bubble: '#d7e9ff' },
};

/**
 * Shared IM disguise: a fake conversation list, a chat transcript fed by the
 * reader, and a compose bar whose send button moves the reading position to the
 * end of the current paragraph.
 */
export function ChatSkin({ brand, ...p }: { brand: Brand } & ReaderChromeProps) {
  const b = BRANDS[brand] ?? BRANDS.wechat;
  const chat = p.chat;
  const contact = chat?.contact || '文件传输助手';
  const nickname = chat?.nickname || '我';
  const avatar = chat?.avatar || '';
  return (
    <div
      className={`skin-root chat-skin ${brand}`}
      style={
        {
          ...p.themeStyle,
          '--im-accent': b.accent,
          '--im-bg': b.bg,
          '--im-header': b.header,
          '--im-side': b.side,
          '--im-bubble': b.bubble,
        } as React.CSSProperties
      }
    >
      <div className="chat-titlebar">
        <span className="chat-brand">{b.name}</span>
        <span className="chat-title-spacer" />
        <button className="icon-btn" onClick={p.onToggleZen} title="禅模式">
          <Icon name={p.zen ? 'exitZen' : 'zen'} size={14} />
        </button>
        <button className={`icon-btn${p.pinned ? ' active' : ''}`} onClick={p.onTogglePin} title="置顶">
          <Icon name={p.pinned ? 'pin' : 'unpin'} size={14} />
        </button>
        <button className="icon-btn" onClick={p.onToggleTheme} title="外观">
          <Icon name="palette" size={14} />
        </button>
        <button className={`icon-btn${p.typing ? ' active' : ''}`} onClick={p.onToggleTyping} title="打字模式">
          <Icon name="keyboard" size={14} />
        </button>
        <button className="icon-btn" onClick={p.onExit} title="关闭">
          <Icon name="close" size={14} />
        </button>
      </div>

      <div className="chat-shell">
        <aside className="chat-side">
          <div className="chat-search">
            <Icon name="search" size={13} />
            <span>搜索</span>
          </div>
          <div className="chat-item active">
            <span className="chat-avatar me">{contact.slice(0, 1)}</span>
            <span className="chat-item-text">
              <b>{contact}</b>
              <small>{chat?.next ? chat.next.slice(0, 16) : '还没有消息'}</small>
            </span>
          </div>
          <div className="chat-item">
            <span className="chat-avatar">产</span>
            <span className="chat-item-text">
              <b>产品讨论组</b>
              <small>周会纪要已更新</small>
            </span>
          </div>
          <div className="chat-item">
            <span className="chat-avatar">项</span>
            <span className="chat-item-text">
              <b>项目群</b>
              <small>今天 09:42</small>
            </span>
          </div>
          <div className="chat-item">
            <span className="chat-avatar">文</span>
            <span className="chat-item-text">
              <b>文件助手</b>
              <small>3 个文件</small>
            </span>
          </div>
          <div className="chat-side-me">
            <span className="chat-avatar me">
              {avatar ? <img src={avatarSrc(avatar)} alt="" /> : avatarInitial(nickname)}
            </span>
            <span className="chat-side-me-text">
              <b>{nickname}</b>
              <small>在线</small>
            </span>
          </div>
        </aside>

        <main className="chat-main">
          <header className="chat-head">
            <b>{contact}</b>
            <span className="chat-head-sub">{p.typing ? '正在输入…' : '在线'}</span>
            <span className="chat-head-pct">{p.percent.toFixed(1)}%</span>
          </header>
          {chat?.transcript}
          <div className="chat-compose">
            <div className="chat-compose-tools">
              <Icon name="file" size={14} />
              <Icon name="download" size={14} />
            </div>
            <input
              className="chat-draft"
              value={p.typing ? chat?.next ?? '' : ''}
              readOnly
              placeholder={p.typing ? '输入消息…' : '开启打字模式后逐段发送'}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && chat?.sendOnEnter) {
                  e.preventDefault();
                  chat.onSend();
                }
              }}
            />
            <button className="chat-send" onClick={() => chat?.onSend()}>
              发送
            </button>
          </div>
        </main>
      </div>
    </div>
  );
}
