import {
  createContext,
  type ReactNode,
  useContext,
  useMemo,
  useState,
} from 'react';
export type Language = 'zh' | 'en';
const en: Record<string, string> = {
  '已登录 选择客户端完成接入': 'Signed in · select clients to connect',
  '登录失败，请检查服务地址和令牌后重试':
    'Sign-in failed. Check the server address and token, then retry.',
  '已退出登录 已配置客户端的 bridge 将停止使用凭证':
    'Signed out · configured clients stop using credentials',
  '退出登录失败，请重试': 'Sign-out failed, please retry.',
  '已移除所选客户端的 PluginPocket 配置':
    'PluginPocket configuration removed from selected clients.',
  '配置已写入 重启客户端后即可使用':
    'Configuration saved · restart clients to apply',
  '配置未完成 请检查客户端配置是否有手写冲突或无效内容，处理后重试':
    'Configuration incomplete · check client configs for manual conflicts or invalid content, then retry.',
  '服务可达 · 凭证有效': 'Server reachable · credentials valid',
  '服务可达 · 尚未登录': 'Server reachable · not signed in',
  '连接检查失败，请检查服务地址、网络和凭证后重试':
    'Connection check failed. Verify the server address, network and credentials, then retry.',
  '账号、工具与这台电脑的接入状态':
    'Account, tools and connection status of this computer',
  '一处接入，随时开工。': 'One connection, ready to work.',
  概况信息: 'Overview information',
  当前余额: 'Current balance',
  'credits · 服务端实时额度': 'credits · live server balance',
  可用工具: 'Available tools',
  账号当前可用: 'Available for this account',
  客户端接入: 'Client connections',
  '已配置 bridge': 'Bridges configured',
  正在读取本地状态: 'Reading local state',
  正在检测客户端: 'Detecting clients',
  账号与连接: 'Account and connection',
  服务地址: 'Server address',
  请在桌面应用中登录: 'Sign in from the desktop app',
  '{account.tools.length} 个可用工具': '{account.tools.length} tools available',
  '从网页控制台创建令牌，然后在这里登录':
    'Create a token in the web console, then sign in here.',
  '尚未登录或凭证不可用，请登录或刷新重试':
    'Not signed in or credentials unavailable. Sign in or refresh to retry.',
  'PluginPocket 令牌': 'PluginPocket token',
  '登录中…': 'Signing in…',
  登录: 'Sign in',
  这台电脑的客户端: 'Clients on this computer',
  等待本机检测: 'Waiting for local detection',
  选择客户端: 'Select clients',
  '已配置 PluginPocket': 'PluginPocket configured',
  '已检测到 · 尚未配置': 'Detected · not configured',
  '未检测到 · 可创建配置': 'Not detected · config can be created',
  已卸载: 'Uninstalled',
  已更新: 'Updated',
  '卸载未完成，请检查本地文件是否有手动改动，然后重试。详情见运行日志。':
    'Uninstall incomplete. Check local files for manual changes, then retry. See the run log for details.',
  '更新未完成，请检查连接、凭证与文件冲突，然后重试。详情见运行日志。':
    'Update incomplete. Check connectivity, credentials and file conflicts, then retry. See the run log for details.',
  搜索已安装装备: 'Search installed equipment',
  '搜索装备名称…': 'Search equipment by name…',
  安装目标: 'Install target',
  全部客户端: 'All clients',
  装备类型: 'Equipment type',
  全部装备: 'All equipment',
  'MCP 插件': 'MCP plugins',
  本地托管清单: 'Locally managed manifest',
  '无法读取已安装装备，请检查本地清单后重试':
    'Could not read installed equipment. Check the local manifest and retry.',
  '你的装备，在本机就位': 'Your equipment, ready on this machine',
  没有匹配的装备: 'No matching equipment',
  还没有本地安装的装备: 'No locally installed equipment yet',
  '在桌面应用中查看已安装的 MCP 和 Skills，以及它们接入的客户端。':
    'View installed MCP servers and Skills in the desktop app, along with their connected clients.',
  '试试其他关键词或清除筛选。': 'Try other keywords or clear the filters.',
  '通过 PluginPocket CLI 安装后，装备会出现在这里。':
    'Equipment appears here after installing via the PluginPocket CLI.',
  '支持查看安装目标、更新与安全卸载':
    'Inspect install targets, update and uninstall safely',
  版本未记录: 'Version not recorded',
  '更新中…': 'Updating…',
  更新: 'Update',
  '卸载会删除已安装的技能文件，包括你对这些文件内容的修改。请先备份需要保留的内容；发现新增文件或路径冲突时将停止。':
    'Uninstalling deletes installed skill files, including your edits. Back up anything you want to keep; it stops on unexpected files or path conflicts.',
  '配置条目有手动改动时会停止并保留配置。':
    'Stops and keeps the configuration if entries were edited manually.',
  '卸载中…': 'Uninstalling…',
  确认卸载: 'Confirm uninstall',
  检查服务地址: 'Check server address',
  留空使用本机已保存的服务: 'Leave empty to use the saved server',
  检查项目: 'Checks',
  '诊断未完成，请检查本机环境后重新运行':
    'Diagnostics incomplete. Check the local environment and run again.',
  '导出失败，请刷新日志后重试，或复制日志':
    'Export failed. Refresh the log and retry, or copy it instead.',
  '复制失败，请重试或导出日志': 'Copy failed. Retry or export the log instead.',
  搜索日志: 'Search logs',
  '搜索操作或日志内容…': 'Search actions or log content…',
  日志级别: 'Log level',
  全部级别: 'All levels',
  信息: 'Info',
  错误: 'Error',
  本机运行记录: 'Local run records',
  '登录 · 配置 · 装备 · Bridge': 'Sign-in · Config · Equipment · Bridge',
  '导出中…': 'Exporting…',
  导出日志: 'Export logs',
  '无法读取运行日志，请重试': 'Could not read the run log, please retry.',
  '运行记录，随时可查': 'Run records, always available',
  没有匹配的日志: 'No matching logs',
  还没有运行日志: 'No run logs yet',
  '在桌面应用中读取这台电脑的真实操作记录。关闭窗口或重启后，记录依然保留。':
    'Read real operation records of this computer in the desktop app. Records persist after closing or restarting.',
  '更换关键词或日志级别，再试一次。': 'Try different keywords or log levels.',
  '登录、配置或管理装备后，可以在这里查看结果。':
    'Results appear here after signing in, configuring or managing equipment.',
  筛选后的本机运行日志: 'Filtered local run log',
  级别: 'Level',
  操作: 'Action',
  记录: 'Record',
  概览: 'Overview',
  我的装备: 'My equipment',
  运行日志: 'Run log',
  连接诊断: 'Diagnostics',
  '你的本机 AI 装备工作台': 'Your local AI workbench',
  '管理这台电脑上由 PluginPocket 安装的工具与技能':
    'Manage PluginPocket tools and skills on this computer',
  '每一步操作，有迹可循': 'Every operation, accounted for',
  '找到连接问题，知道下一步怎么做':
    'Find connection issues and know what to do next',
  工作台: 'Workbench',
  '插件口袋 · AI 装备工坊': 'PluginPocket · AI workbench',
  '工具随身，密钥留在本机': 'Tools nearby, keys stay local',
  '一个入口，连接你的 AI 客户端。': 'One entry point for your AI clients.',
  桌面应用: 'Desktop app',
  浏览器预览: 'Browser preview',
  界面实时预览: 'Live UI preview',
  本机工作台: 'Local workbench',
  外观: 'Appearance',
  浅色: 'Light',
  深色: 'Dark',
  跟随系统: 'System',
  语言: 'Language',
  中文: '中文',
  英文: 'English',
  本地操作: 'Local operations',
  用量明细: 'Usage details',
  账变记录: 'Ledger',
  刷新账单: 'Refresh billing',
  重试读取账单: 'Retry billing',
  '账单暂不可用，请在概览登录或重试读取。':
    'Billing is unavailable. Sign in from Overview or retry.',
  '请在桌面应用中登录后查看真实账单。':
    'Sign in from the desktop app to view live billing.',
  暂无记录: 'No records yet',
  正在读取账单: 'Reading billing',
  时间: 'Time',
  工具: 'Tool',
  状态: 'Status',
  耗时: 'Duration',
  费用: 'Cost',
  类型: 'Type',
  说明: 'Note',
  变动: 'Change',
  余额后: 'Balance after',
  处理中: 'Pending',
  成功: 'Success',
  失败: 'Failed',
  已拒绝: 'Denied',
  已恢复: 'Recovered',
  注册赠送: 'Registration',
  额度调整: 'Adjustment',
  兑换入账: 'Redemption',
  团队转账: 'Team transfer',
  调用预扣: 'Reservation',
  退款: 'Refund',
  额度回收: 'Recovery',
  '从服务端读取，不保存到本地操作日志。':
    'Read live from the server; not stored in local operation logs.',
  '这里显示当前账号的真实账单记录。':
    'Real billing records for the current account.',
  '当前显示最近一页记录。': 'Showing the most recent page of records.',
  运行记录分类: 'Run record categories',
  '可切换页面与外观。本机日志、装备和诊断需在桌面应用中读取。':
    'Switch pages and appearance. Local logs, equipment and diagnostics require the desktop app.',
};
const I18nContext = createContext<{
  language: Language;
  setLanguage: (language: Language) => void;
  t: (key: string) => string;
} | null>(null);
export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<Language>(() =>
    navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en',
  );
  const value = useMemo(
    () => ({
      language,
      setLanguage,
      t: (key: string) => (language === 'en' ? (en[key] ?? key) : key),
    }),
    [language],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error('I18nProvider is missing');
  return value;
}
