import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';

beforeEach(() => localStorage.clear());

const tools = {
  items: [
    {
      id: 9,
      key: 'remote',
      name: 'Remote tools',
      description: 'test',
      kind: 'http',
      enabled: true,
      units_per_call: 1,
      input_schema: { type: 'object' },
      configured: true,
    },
  ],
  next_cursor: '',
};
const market = {
  items: [
    {
      id: 1,
      slug: 'deepwiki',
      name: 'DeepWiki',
      description: 'Ask about repos',
      source: 'curated',
      repo_url: 'https://github.com/AsyncFuncAI/deepwiki-mcp',
      homepage: 'https://deepwiki.com',
      transport: 'http',
      endpoint: 'https://mcp.deepwiki.com/mcp',
      package: '',
      stars: 0,
      synced_at: null,
      installed: false,
    },
    {
      id: 2,
      slug: 'github-github-mcp-server',
      name: 'github-mcp-server',
      description: "GitHub's official MCP Server",
      source: 'github',
      repo_url: 'https://github.com/github/github-mcp-server',
      homepage: '',
      transport: 'unknown',
      endpoint: '',
      package: '',
      stars: 32852,
      synced_at: '2026-09-11T00:00:00Z',
      installed: false,
    },
  ],
};

function mount(
  extra?: (url: string, init: RequestInit) => Response | undefined,
) {
  const calls: { url: string; init: RequestInit }[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit = {}) => {
      calls.push({ url, init });
      const routed = extra?.(url, init);
      if (routed) return routed;
      if (url === '/api/v1/account/me')
        return Response.json({
          user: {
            id: 1,
            username: 'operator',
            role: 'admin',
            balance: 0,
            enabled: true,
          },
          summary: { today_calls: 0, month_cost: 0, token_count: 0 },
        });
      if (url.startsWith('/api/v1/admin/tools')) return Response.json(tools);
      if (url === '/api/v1/admin/marketplace') return Response.json(market);
      return Response.json({ github: true, email: true, payments: false });
    }),
  );
  window.history.replaceState({}, '', '/admin/tools');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), calls };
}

it('lists the marketplace, syncs GitHub, and installs a curated plugin', async () => {
  let synced = false;
  let installed = false;
  const { user, calls } = mount((url, init) => {
    if (url === '/api/v1/admin/marketplace' && installed)
      return Response.json({
        items: market.items.map((item) =>
          item.slug === 'deepwiki' ? { ...item, installed: true } : item,
        ),
      });
    if (url === '/api/v1/admin/marketplace/sync' && init.method === 'POST') {
      synced = true;
      return Response.json({ synced: 1 });
    }
    if (url === '/api/v1/admin/marketplace/install' && init.method === 'POST') {
      expect(JSON.parse(String(init.body)).slug).toBe('deepwiki');
      installed = true;
      return Response.json(
        { item: { ...market.items[0], installed: true }, tool_id: 42 },
        { status: 201 },
      );
    }
    return undefined;
  });
  expect(await screen.findByText('DeepWiki')).toBeInTheDocument();
  expect(screen.getByText('github-mcp-server')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '同步 GitHub' }));
  expect(await screen.findByText(/同步了 1/)).toBeInTheDocument();
  expect(synced).toBe(true);
  await user.click(screen.getByRole('button', { name: '安装 DeepWiki' }));
  const config = await screen.findByLabelText('连接配置（JSON）');
  expect(config).toHaveValue('{"url":"https://mcp.deepwiki.com/mcp"}');
  await user.click(await screen.findByRole('button', { name: '安装到工具池' }));
  expect(await screen.findByText('已安装')).toBeInTheDocument();
  expect(
    calls.some((call) => call.url === '/api/v1/admin/marketplace/install'),
  ).toBe(true);
});

it('installs a GitHub item once its HTTP endpoint is supplied', async () => {
  let installed = false;
  const { user } = mount((url, init) => {
    if (url === '/api/v1/admin/marketplace/install' && init.method === 'POST') {
      const body = JSON.parse(String(init.body));
      expect(body.transport).toBe('http');
      expect(body.config.url).toBe('https://mcp.example.com/mcp');
      installed = true;
      return Response.json(
        { item: { ...market.items[1], installed: true }, tool_id: 43 },
        { status: 201 },
      );
    }
    if (url === '/api/v1/admin/marketplace' && installed)
      return Response.json({
        items: market.items.map((item) =>
          item.slug === 'github-github-mcp-server'
            ? { ...item, installed: true }
            : item,
        ),
      });
    return undefined;
  });
  expect(await screen.findByText('github-mcp-server')).toBeInTheDocument();
  await user.click(
    screen.getByRole('button', { name: '安装 github-mcp-server' }),
  );
  const config = await screen.findByLabelText('连接配置（JSON）');
  await user.type(config, '{{"url":"https://mcp.example.com/mcp"}');
  await user.click(await screen.findByRole('button', { name: '安装到工具池' }));
  expect(await screen.findByText('已安装')).toBeInTheDocument();
});

it('edits and clears an upstream tool description override', async () => {
  let override: { remote_name: string; description: string } | null = null;
  mount((url, init) => {
    if (url === '/api/v1/admin/tools/9/upstream')
      return Response.json({
        tools: [
          {
            name: 'ping',
            description: 'original ping',
            input_schema: { type: 'object' },
          },
        ],
        overrides: override ? [override] : [],
      });
    if (url === '/api/v1/admin/tools/9/metadata' && init.method === 'PUT') {
      const body = JSON.parse(String(init.body));
      expect(body.remote_name).toBe('ping');
      expect(body.description).toBe('自定义说明');
      override = body;
      return Response.json({ item: body });
    }
    if (
      url === '/api/v1/admin/tools/9/metadata/ping' &&
      init.method === 'DELETE'
    ) {
      override = null;
      return new Response(null, { status: 204 });
    }
    return undefined;
  });
  expect(await screen.findByText('Remote tools')).toBeInTheDocument();
  await userEvent.click(
    screen.getByRole('button', { name: '查看参数 Remote tools' }),
  );
  expect(await screen.findByText('original ping')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: '覆盖描述 ping' }));
  const description = await screen.findByLabelText('覆盖描述');
  await userEvent.type(description, '自定义说明');
  await userEvent.click(screen.getByRole('button', { name: '保存覆盖' }));
  expect(await screen.findByText('自定义说明')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: '清除覆盖 ping' }));
  expect(await screen.findByText('original ping')).toBeInTheDocument();
});
