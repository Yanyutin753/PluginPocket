import type { Locale } from '../i18n';

// Error copy keyed by the backend wire code. The backend registry
// (server/internal/httpapi/codes.go) is the source of truth; error-codes.json
// is its generated contract and errors.test.ts fails when a code lacks copy.
// zh strings are stored without the trailing 。 so they render exactly like
// translate() output. invalid_json and unknown are frontend-local codes.
export const errorMessages: Record<string, { zh: string; en: string }> = {
  already_installed: {
    zh: '该插件已安装',
    en: 'This plugin is already installed.',
  },
  already_member: {
    zh: '你已经是此团队成员',
    en: 'You are already a member of this team.',
  },
  already_redeemed: {
    zh: '此兑换码已经使用，请核对额度流水',
    en: 'This code has already been redeemed. Check your credit transactions.',
  },
  auth_busy: {
    zh: '登录服务繁忙，请稍后再试',
    en: 'Sign-in is busy right now. Please try again later.',
  },
  authorization_pending: {
    zh: '尚未批准设备授权，请继续等待',
    en: 'Authorization is still pending. Keep waiting on your device.',
  },
  cannot_disable_self: {
    zh: '不能停用当前登录账号',
    en: 'You cannot disable the account you are signed in with.',
  },
  config_required: {
    zh: '缺少上游连接配置',
    en: 'Upstream connection configuration is required.',
  },
  directory_unavailable: {
    zh: '插件目录暂不可用，请稍后重试',
    en: 'The plugin directory is temporarily unavailable. Please try again later.',
  },
  email_unavailable: {
    zh: '邮箱不可用或邮件服务暂不可用，请稍后重试',
    en: 'This email is unavailable or the email service is temporarily unavailable. Try again later.',
  },
  expired_token: {
    zh: '授权码已过期，请在设备上重新发起登录',
    en: 'This authorization code has expired. Start sign-in again on your device.',
  },
  forbidden: {
    zh: '没有权限执行此操作',
    en: 'You do not have permission to perform this action.',
  },
  forbidden_origin: {
    zh: '请求来源未通过验证，请从本站重新打开页面',
    en: 'The request origin could not be verified. Reopen the page from this site.',
  },
  gateway_unavailable: {
    zh: '网关暂不可用，请稍后重试',
    en: 'The gateway is temporarily unavailable. Please try again later.',
  },
  github_failed: {
    zh: 'GitHub 授权失败，请稍后重试',
    en: 'GitHub authorization failed. Please try again later.',
  },
  github_unavailable: {
    zh: 'GitHub 服务暂不可用，请稍后重试',
    en: 'GitHub is temporarily unavailable. Please try again later.',
  },
  idempotency_conflict: {
    zh: '这次操作与已有记录冲突，请关闭表单并重新核对',
    en: 'This operation conflicts with an existing record. Close the form and review the details.',
  },
  insufficient_balance: {
    zh: '可用额度不足，请检查调整金额',
    en: 'Not enough credits. Check the adjustment amount.',
  },
  internal_error: {
    zh: '服务内部错误，请稍后重试',
    en: 'The service hit an internal error. Please try again later.',
  },
  invalid_credentials: {
    zh: '用户名或密码不正确，请重新输入',
    en: 'Incorrect username or password. Please try again.',
  },
  invalid_device_code: {
    zh: '设备授权码无效、已使用或已过期，请在设备上重新发起登录',
    en: 'This device code is invalid, already used, or expired. Start sign-in again on your device.',
  },
  invalid_grant: {
    zh: '授权码无效或已使用，请在设备上重新发起登录',
    en: 'This authorization code is invalid or already used. Start sign-in again on your device.',
  },
  invalid_icon: {
    zh: '图标无效，请使用 HTTPS 图片或不超过 64 KiB 的 PNG、JPEG、WebP、安全 SVG',
    en: 'Invalid icon. Use an HTTPS image or a PNG, JPEG, WebP or safe SVG up to 64 KiB.',
  },
  invalid_json: {
    zh: 'JSON 格式不正确，请检查参数或连接配置后重试',
    en: 'Invalid JSON. Check the parameters or connection configuration and try again.',
  },
  invalid_password: {
    zh: '密码长度需在 12 到 1024 个字符之间',
    en: 'Password must be 12 to 1024 characters long.',
  },
  invalid_request: {
    zh: '提交内容不符合要求，请检查后重试',
    en: 'Check the submitted information and try again.',
  },
  invalid_settlement: {
    zh: '结算策略无效：检查 JSON、业务码路径或正则',
    en: 'Invalid settlement policy: check the JSON, business-code path, or pattern.',
  },
  invalid_state: {
    zh: '状态校验未通过，请重新发起登录',
    en: 'The state check failed. Start sign-in again.',
  },
  invalid_token: {
    zh: '验证链接无效或已过期，请重新发送验证邮件',
    en: 'This verification link is invalid or expired. Request another verification email.',
  },
  invalid_username: {
    zh: '用户名需为 3–32 位字母、数字、下划线或横线',
    en: 'Username must be 3–32 letters, digits, underscores, or hyphens.',
  },
  invite_expired: {
    zh: '邀请码已过期，请联系团队所有者生成新邀请',
    en: 'This invitation has expired. Ask the team owner for a new invitation.',
  },
  invite_used: {
    zh: '邀请码已经使用，请联系团队所有者生成新邀请',
    en: 'This invitation has already been used. Ask the team owner for a new invitation.',
  },
  key_taken: {
    zh: '标识已被使用，请换一个',
    en: 'This identifier is already taken. Choose another one.',
  },
  last_admin: {
    zh: '需要保留至少一位启用的管理员',
    en: 'At least one administrator must remain active.',
  },
  last_owner: {
    zh: '最后一位团队所有者不能退出',
    en: 'The last team owner cannot leave the team.',
  },
  marketplace_unavailable: {
    zh: '插件市场暂不可用，请稍后重试',
    en: 'The marketplace is temporarily unavailable. Please try again later.',
  },
  method_not_allowed: {
    zh: '不支持的请求方法',
    en: 'This request method is not allowed.',
  },
  not_found: {
    zh: '记录已不存在，请刷新后重试',
    en: 'This record no longer exists. Refresh and try again.',
  },
  not_installed: { zh: '该插件未安装', en: 'This plugin is not installed.' },
  organization_required: {
    zh: '请使用指定组织的 GitHub 账号登录',
    en: 'Sign in with a GitHub account from the allowed organization.',
  },
  payment_unavailable: {
    zh: '在线支付暂未配置，请使用兑换码或联系管理员补充额度',
    en: 'Online payments are not configured. Use a redemption code or contact an administrator to add credits.',
  },
  rate_limited: {
    zh: '请求过于频繁，请稍后再试',
    en: 'Too many requests. Please try again later.',
  },
  rate_limit_unavailable: {
    zh: '限流服务暂不可用，请稍后重试',
    en: 'The rate limiter is temporarily unavailable. Please try again later.',
  },
  registration_unavailable: {
    zh: '注册服务暂不可用，请稍后重试',
    en: 'Registration is temporarily unavailable. Please try again later.',
  },
  seats_in_use: {
    zh: '席位数量不能少于现有成员人数',
    en: 'The seat limit cannot be lower than the current number of members.',
  },
  settings_conflict: {
    zh: '其他管理员已更新配置，请重新加载后再编辑',
    en: 'Another administrator updated the settings. Reload before editing again.',
  },
  settings_encryption_unavailable: {
    zh: '部署未配置加密主密钥，暂不能保存新的集成密钥',
    en: 'The deployment encryption key is not configured. New integration secrets cannot be saved.',
  },
  settings_unavailable: {
    zh: '系统配置暂不可用，请检查服务部署后重试',
    en: 'System settings are unavailable. Check the deployment and retry.',
  },
  slow_down: {
    zh: '查询过于频繁，请降低轮询频率',
    en: 'You are polling too fast. Wait longer between attempts.',
  },
  team_full: {
    zh: '团队席位已满，请联系团队所有者调整席位',
    en: 'All team seats are occupied. Ask the team owner to add seats.',
  },
  temporarily_unavailable: {
    zh: '服务暂时不可用，请稍后重试',
    en: 'The service is temporarily unavailable. Please try again later.',
  },
  tool_disabled: { zh: '工具已被停用', en: 'This tool has been disabled.' },
  transport_not_supported: {
    zh: '不支持的传输类型',
    en: 'This transport type is not supported.',
  },
  transport_required: {
    zh: '缺少上游传输配置',
    en: 'An upstream transport is required.',
  },
  unauthorized: {
    zh: '登录已过期，请重新登录',
    en: 'Your session has expired. Sign in again.',
  },
  unknown: {
    zh: '暂时无法完成请求，请检查连接后重试',
    en: 'Unable to complete the request. Check your connection and try again.',
  },
  upstream_unavailable: {
    zh: '上游服务暂不可用，请稍后重试',
    en: 'The upstream service is temporarily unavailable. Please try again later.',
  },
  username_taken: {
    zh: '用户名已被使用，请换一个用户名',
    en: 'This username is taken. Choose another username.',
  },
};

export function errorText(code: string, locale: Locale): string {
  const entry = errorMessages[code];
  if (!entry)
    return locale === 'en'
      ? errorMessages.unknown.en
      : errorMessages.unknown.zh;
  return locale === 'en' ? entry.en : entry.zh;
}
