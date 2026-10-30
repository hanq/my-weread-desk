// Single source of truth for the disguise skins. The settings page, the reader
// and the global skin cycle all read this list so a new shell is added once.
export type SkinId = 'none' | 'word' | 'vscode' | 'wechat' | 'dingtalk' | 'feishu' | 'wecom';

export interface SkinOption {
  id: SkinId;
  label: string;
  /** plain = original reader, document = office shell, im = chat shell. */
  kind: 'plain' | 'document' | 'im';
}

export const SKIN_OPTIONS: SkinOption[] = [
  { id: 'none', label: '纯净', kind: 'plain' },
  { id: 'word', label: 'Word', kind: 'document' },
  { id: 'vscode', label: 'VS Code', kind: 'document' },
  { id: 'wechat', label: '微信', kind: 'im' },
  { id: 'dingtalk', label: '钉钉', kind: 'im' },
  { id: 'feishu', label: '飞书', kind: 'im' },
  { id: 'wecom', label: '企业微信', kind: 'im' },
];

export function isChatSkin(skin: string): boolean {
  return skin === 'wechat' || skin === 'dingtalk' || skin === 'feishu' || skin === 'wecom';
}

export function isDocumentSkin(skin: string): boolean {
  return skin === 'word' || skin === 'vscode';
}

/** avatarInitial is the fallback shown when no image is configured. */
export function avatarInitial(nickname: string): string {
  const text = (nickname || '我').trim();
  return text ? Array.from(text)[0] : '我';
}

/** avatarSrc points at the asset-server route for the stored avatar file. */
export function avatarSrc(avatar: string): string {
  return `/avatar/${encodeURIComponent(avatar)}`;
}

export function skinLabel(skin: string): string {
  return SKIN_OPTIONS.find((option) => option.id === skin)?.label ?? '纯净';
}
