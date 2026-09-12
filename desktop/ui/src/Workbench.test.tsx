import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { invoke, isTauri } from '@tauri-apps/api/core';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
  isTauri: vi.fn(() => true),
}));
const equipment = [
  { slug: 'research', kind: 'skill', clients: ['codex'], version: null },
  { slug: 'search', kind: 'mcp', clients: ['claude'], version: 'v2' },
];
const logs = [
  {
    timestamp: 1789189200000,
    level: 'info',
    action: 'apply',
    message: '客户端配置已写入',
  },
  {
    timestamp: 1789189260000,
    level: 'error',
    action: 'login',
    message: '登录失败，请检查服务与凭证',
  },
];
beforeEach(() => {
  vi.mocked(isTauri).mockReturnValue(true);
  vi.mocked(invoke).mockReset();
  vi.mocked(invoke).mockImplementation(async (_name, args) => {
    const { action } = (args as { command: { action: string } }).command;
    switch (action) {
      case 'clients':
        return [];
      case 'status':
        throw new Error('not logged in');
      case 'installed':
        return equipment;
      case 'logs':
        return logs;
      case 'diagnostics':
        return {
          checks: [
            {
              id: 'network',
              label: '服务连接',
              status: 'error',
              detail: '无法连接服务，请检查地址和网络',
            },
            {
              id: 'bridge',
              label: 'Bridge 可执行文件',
              status: 'ok',
              detail: '本机文件可用；未执行 MCP 调用',
            },
          ],
        };
      default:
        throw new Error('unexpected action');
    }
  });
});
function mount() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
it('navigates the workbench by keyboard and exposes independent diagnostic failures', async () => {
  const user = userEvent.setup();
  mount();
  const nav = screen.getByRole('navigation', { name: '工作台' });
  within(nav).getByRole('button', { name: '连接诊断' }).focus();
  await user.keyboard('{Enter}');
  expect(
    screen.getByRole('heading', { level: 1, name: '连接诊断' }),
  ).toBeInTheDocument();
  expect(screen.queryByLabelText('PluginPocket 令牌')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '运行诊断' }));
  expect(
    await screen.findByText('无法连接服务，请检查地址和网络'),
  ).toBeInTheDocument();
  expect(screen.getByText('本机文件可用；未执行 MCP 调用')).toBeInTheDocument();
  expect(screen.getByText('需要处理')).toBeInTheDocument();
});
it('filters installed equipment and confirms uninstall for its recorded client only', async () => {
  const user = userEvent.setup();
  let installed = [...equipment];
  const handler = vi.mocked(invoke).getMockImplementation();
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    const command = (
      args as { command: { action: string; slug?: string; clients?: string[] } }
    ).command;
    if (command.action === 'installed') return installed;
    if (command.action === 'uninstall_installed') {
      expect(command).toEqual({
        action: 'uninstall_installed',
        slug: 'research',
        kind: 'skill',
        clients: ['codex'],
      });
      installed = installed.filter((item) => item.slug !== command.slug);
      return null;
    }
    return handler?.(name, args);
  });
  mount();
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await screen.findByText('research');
  expect(screen.getByText('版本未记录')).toBeInTheDocument();
  await user.type(
    screen.getByRole('searchbox', { name: '搜索已安装装备' }),
    'research',
  );
  expect(screen.queryByText('search', { exact: true })).not.toBeInTheDocument();
  await user.click(
    screen.getByRole('button', { name: '卸载 research · Skill' }),
  );
  expect(screen.getByText(/将从 Codex 移除 research/)).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '取消卸载' }));
  expect(screen.getByText('research')).toBeInTheDocument();
  await user.click(
    screen.getByRole('button', { name: '卸载 research · Skill' }),
  );
  await user.click(
    screen.getByRole('button', { name: '确认卸载 research · Skill' }),
  );
  await screen.findByText('已卸载 research');
  expect(
    screen.queryByRole('button', { name: '卸载 research · Skill' }),
  ).not.toBeInTheDocument();
});
it('recovers from an equipment update error without removing the installed row', async () => {
  const user = userEvent.setup();
  mount();
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await screen.findByText('research');
  vi.mocked(invoke).mockRejectedValueOnce(
    new Error('ppt_secret upstream body'),
  );
  await user.click(
    screen.getByRole('button', { name: '更新 research · Skill' }),
  );
  await screen.findByText(/更新未完成/);
  expect(screen.queryByText(/ppt_secret/)).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: '更新 research · Skill' }),
  ).toBeEnabled();
});
it('filters logs and copies only visible records, with clipboard failure recovery', async () => {
  const user = userEvent.setup();
  mount();
  await user.click(screen.getByRole('button', { name: '运行日志' }));
  await screen.findByText('客户端配置已写入');
  await user.selectOptions(screen.getByLabelText('日志级别'), 'error');
  expect(screen.queryByText('客户端配置已写入')).not.toBeInTheDocument();
  expect(screen.getByText('登录失败，请检查服务与凭证')).toBeInTheDocument();
  vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValueOnce(
    new Error('denied'),
  );
  await user.click(screen.getByRole('button', { name: '复制日志' }));
  await screen.findByText('复制失败，请重试或导出日志');
  await user.click(screen.getByRole('button', { name: '复制日志' }));
  await screen.findByText('已复制 1 条日志');
  const copied = await navigator.clipboard.readText();
  expect(copied).toContain('登录失败');
  expect(copied).not.toContain('客户端配置已写入');
  await user.type(
    screen.getByRole('searchbox', { name: '搜索日志' }),
    'missing',
  );
  expect(screen.getByText('没有匹配的日志')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '导出日志' })).toBeDisabled();
});
it('keeps invalid log responses out of the list and allows a real retry', async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByLabelText('服务地址');
  vi.mocked(invoke).mockResolvedValueOnce([
    { timestamp: 'fake', message: 'bad data' },
  ]);
  await user.click(screen.getByRole('button', { name: '运行日志' }));
  await screen.findByText('无法读取运行日志，请重试');
  expect(screen.queryByText('bad data')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '重试读取日志' }));
  await screen.findByText('客户端配置已写入');
});
it('marks browser preview honestly and never calls native commands from it', async () => {
  vi.mocked(isTauri).mockReturnValue(false);
  const user = userEvent.setup();
  mount();
  expect(screen.getByText('浏览器预览')).toBeInTheDocument();
  for (const page of ['我的装备', '运行日志', '连接诊断', '概览']) {
    await user.click(screen.getByRole('button', { name: page }));
    expect(
      screen.getByRole('heading', { level: 1, name: page }),
    ).toBeInTheDocument();
  }
  await waitFor(() => expect(invoke).not.toHaveBeenCalled());
  expect(screen.queryByText(/42 credits/)).not.toBeInTheDocument();
});

it('exports only selected real logs through native storage and shows the saved file', async () => {
  const user = userEvent.setup();
  const handler = vi.mocked(invoke).getMockImplementation();
  let saved: unknown;
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    const command = (args as { command: { action: string; entries?: unknown } })
      .command;
    if (command.action === 'export_logs') {
      saved = command.entries;
      return { path: '/home/test/.pluginpocket/exports/logs.txt', count: 1 };
    }
    return handler?.(name, args);
  });
  mount();
  await user.click(screen.getByRole('button', { name: '运行日志' }));
  await screen.findByText('客户端配置已写入');
  await user.selectOptions(screen.getByLabelText('日志级别'), 'error');
  await user.click(screen.getByRole('button', { name: '导出日志' }));
  await screen.findByText(/日志已保存到/);
  expect(
    screen.getByText('/home/test/.pluginpocket/exports/logs.txt'),
  ).toBeInTheDocument();
  expect(saved).toEqual([logs[1]]);
});

it('keeps same-named MCP and Skill confirmation independent and warns about edited skill contents', async () => {
  const user = userEvent.setup();
  const handler = vi.mocked(invoke).getMockImplementation();
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    if (
      (args as { command: { action: string } }).command.action === 'installed'
    )
      return [equipment[0], { ...equipment[1], slug: 'research' }];
    return handler?.(name, args);
  });
  mount();
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await user.click(
    await screen.findByRole('button', { name: '卸载 research · Skill' }),
  );
  expect(screen.getByText(/包括你对这些文件内容的修改/)).toBeInTheDocument();
  expect(screen.getAllByRole('button', { name: /^确认卸载/ })).toHaveLength(1);
  expect(
    screen.queryByRole('button', { name: '确认卸载 research · MCP' }),
  ).not.toBeInTheDocument();
});

it('shows gateway MCP tools alongside local skills and routes connection management to overview', async () => {
  const user = userEvent.setup();
  const handler = vi.mocked(invoke).getMockImplementation();
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    if ((args as { command: { action: string } }).command.action === 'status')
      return {
        account: {
          username: 'alice',
          balance: 42,
          tools: ['deepwiki', 'context7'],
        },
        clients: [
          { client: 'codex', detected: true, configured: true },
          { client: 'claude', detected: true, configured: false },
        ],
      };
    return handler?.(name, args);
  });
  mount();
  await screen.findByText('alice');
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await screen.findByRole('heading', { name: 'PluginPocket MCP 网关' });
  expect(screen.getByText('deepwiki')).toBeInTheDocument();
  expect(screen.getByText('context7')).toBeInTheDocument();
  expect(screen.getByText('已接入 Codex')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Skills' }));
  expect(
    screen.queryByRole('heading', { name: 'PluginPocket MCP 网关' }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'MCP 插件' }));
  await user.type(
    screen.getByRole('searchbox', { name: '搜索已安装装备' }),
    'deepwiki',
  );
  expect(screen.getByText('deepwiki')).toBeInTheDocument();
  expect(screen.queryByText('context7')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '管理网关接入' }));
  expect(
    screen.getByRole('heading', { level: 1, name: '概览' }),
  ).toBeInTheDocument();
});

it('rejects a malformed gateway directory while keeping local equipment readable', async () => {
  const user = userEvent.setup();
  const handler = vi.mocked(invoke).getMockImplementation();
  vi.mocked(invoke).mockImplementation(async (name, args) => {
    if ((args as { command: { action: string } }).command.action === 'status')
      return {
        account: {
          username: 'alice',
          balance: 42,
          tools: [{ arbitrary: 'invalid' }],
        },
        clients: [],
      };
    return handler?.(name, args);
  });
  mount();
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await screen.findByText('网关目录暂不可用，请在概览登录或重试读取。');
  expect(screen.getByText('research')).toBeInTheDocument();
  expect(
    screen.queryByText(/当前账号暂无可用网关工具/),
  ).not.toBeInTheDocument();
});

it('keeps gateway failure and retry visible when filtering by a client', async () => {
  const user = userEvent.setup();
  mount();
  await user.click(screen.getByRole('button', { name: '我的装备' }));
  await screen.findByText('网关目录暂不可用，请在概览登录或重试读取。');
  await user.selectOptions(screen.getByLabelText('安装目标'), 'codex');
  expect(screen.getByRole('button', { name: '重试读取网关' })).toBeEnabled();
  expect(screen.getByText('research')).toBeInTheDocument();
});
