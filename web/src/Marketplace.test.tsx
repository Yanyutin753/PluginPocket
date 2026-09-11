import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { editCode } from './test/edit-code';

beforeEach(() => localStorage.clear());

it('keeps skill and bundle rows focused on editing without install tutorials', async () => {
  const { user } = mount((url) => {
    if (url === '/api/v1/admin/marketplace')
      return Response.json({
        items: ['skill', 'bundle'].map((kind, index) => ({
          ...market.items[0],
          id: index + 3,
          kind,
          slug: `review-${kind}`,
          name: `Review ${kind}`,
          repo_url: '',
          spec:
            kind === 'skill'
              ? { source: 'inline', files: { 'SKILL.md': 'Review' } }
              : { includes: ['deepwiki'] },
        })),
      });
  }, '/admin/marketplace');
  await screen.findByText('Review skill');
  for (const kind of ['skill', 'bundle']) {
    const row = screen
      .getByRole('heading', { name: `Review ${kind}` })
      .closest('li');
    if (!row) throw new Error(`Missing marketplace row for ${kind}`);
    expect(row).not.toHaveTextContent(/loadout install|组合安装/);
    expect(row).toHaveTextContent(`review-${kind}`);
    const edit = within(row).getByRole('button', {
      name: `编辑 Review ${kind}`,
    });
    edit.focus();
    await user.keyboard('{Enter}');
    expect(await screen.findByRole('dialog')).toBeVisible();
    await user.keyboard('{Escape}');
    expect(edit).toHaveFocus();
  }
  expect(screen.queryByText(/技能保存工作方法/)).not.toBeInTheDocument();
});

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
  extra?: (
    url: string,
    init: RequestInit,
  ) => Response | Promise<Response> | undefined,
  path = '/admin/tools',
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
  window.history.replaceState({}, '', path);
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
  await user.click(await screen.findByRole('button', { name: '插件市场' }));
  expect(await screen.findByText('DeepWiki')).toBeInTheDocument();
  expect(screen.getByText('github-mcp-server')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '同步 GitHub' }));
  expect(await screen.findByText(/同步了 1/)).toBeInTheDocument();
  expect(synced).toBe(true);
  await user.click(
    screen.getByRole('button', { name: '安装到工具池 DeepWiki' }),
  );
  const config = await screen.findByLabelText('连接配置（JSON）');
  expect(config).toHaveValue('{"url":"https://mcp.deepwiki.com/mcp"}');
  await user.click(await screen.findByRole('button', { name: '安装到工具池' }));
  expect(await screen.findByText('已加入工具池')).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: '移出工具池 DeepWiki' }),
  ).toBeVisible();
  expect(
    calls.some((call) => call.url === '/api/v1/admin/marketplace/install'),
  ).toBe(true);
});

it('opens market management from navigation and publishes a custom skill', async () => {
  const { user, calls } = mount((url, init) => {
    if (url.endsWith('/marketplace/skills') && init.method === 'POST')
      return Response.json({
        item: {
          ...market.items[0],
          kind: 'skill',
          slug: 'review',
          name: 'Review',
        },
      });
  });
  await user.click(await screen.findByRole('link', { name: '市场管理' }));
  await user.click(await screen.findByRole('button', { name: '创建技能' }));
  await user.type(screen.getByLabelText('名称'), 'Review');
  await user.type(screen.getByLabelText('标识'), 'review');
  await editCode(user, 'SKILL.md 内容', '# Review code');
  await user.click(screen.getByRole('button', { name: '保存技能' }));
  expect(await screen.findByRole('status')).toHaveTextContent('已保存');
  const call = calls.find((c) => c.url.endsWith('/marketplace/skills'));
  expect(JSON.parse(String(call?.init.body))).toMatchObject({
    slug: 'review',
    source: 'inline',
    files: { 'SKILL.md': '# Review code' },
  });
});

it('creates a bundle by selecting existing components', async () => {
  const { user, calls } = mount((url, init) => {
    if (url.endsWith('/marketplace/bundles') && init.method === 'POST')
      return Response.json({
        item: { ...market.items[0], kind: 'bundle', slug: 'starter' },
      });
  }, '/admin/marketplace');
  await user.click(await screen.findByRole('button', { name: '创建装备组' }));
  await user.type(screen.getByLabelText('名称'), 'Starter');
  await user.type(screen.getByLabelText('标识'), 'starter');
  expect(
    screen.getByRole('checkbox', { name: /github-mcp-server/ }),
  ).toBeDisabled();
  await user.click(screen.getByRole('checkbox', { name: /DeepWiki/ }));
  await user.click(screen.getByRole('button', { name: '保存装备组' }));
  expect(await screen.findByRole('status')).toHaveTextContent('已保存');
  expect(
    JSON.parse(
      String(
        calls.find((c) => c.url.endsWith('/marketplace/bundles'))?.init.body,
      ),
    ),
  ).toMatchObject({ includes: ['deepwiki'] });
});

it('imports a recommended GitHub skill and retries after failure', async () => {
  let attempts = 0;
  const { user, calls } = mount((url, init) => {
    if (url.endsWith('/marketplace/skills') && init.method === 'POST') {
      attempts++;
      return attempts === 1
        ? Response.json({ error: 'upstream_unavailable' }, { status: 502 })
        : Response.json({ item: { ...market.items[0], kind: 'skill' } });
    }
  }, '/admin/marketplace');
  await user.click(
    await screen.findByRole('button', { name: '精选 GitHub 技能' }),
  );
  await user.click(
    screen.getByRole('button', { name: '同步 Systematic Debugging' }),
  );
  expect(await screen.findByRole('alert')).toBeVisible();
  await user.click(
    screen.getByRole('button', { name: '同步 Systematic Debugging' }),
  );
  expect(await screen.findByRole('status')).toHaveTextContent('已同步');
  expect(
    JSON.parse(
      String(
        calls.find((c) => c.url.endsWith('/marketplace/skills'))?.init.body,
      ),
    ),
  ).toMatchObject({
    source: 'github',
    repo: 'obra/superpowers',
    path: 'skills/systematic-debugging',
  });
});

it('syncs all selected skills and reports individual failures', async () => {
  const { user, calls } = mount((url, init) => {
    if (url.endsWith('/marketplace/skills') && init.method === 'POST') {
      const body = JSON.parse(String(init.body));
      return body.slug === 'superpowers-tdd'
        ? Response.json({ error: 'upstream_unavailable' }, { status: 502 })
        : Response.json({ item: { ...market.items[0], kind: 'skill' } });
    }
  }, '/admin/marketplace');
  await user.click(
    await screen.findByRole('button', { name: '精选 GitHub 技能' }),
  );
  await user.click(screen.getByRole('button', { name: '一键同步全部' }));
  expect(await screen.findByText('同步完成：成功 2，失败 1')).toBeVisible();
  expect(screen.getByText(/Test Driven Development.*同步失败/)).toBeVisible();
  expect(
    calls.filter((call) => call.url.endsWith('/marketplace/skills')),
  ).toHaveLength(3);
});

it('filters skills, preserves supporting files on edit, and returns keyboard focus', async () => {
  const skill = {
    ...market.items[0],
    id: 3,
    slug: 'review',
    name: 'Review',
    kind: 'skill',
    spec: {
      source: 'inline',
      files: { 'SKILL.md': 'original', 'reference.md': 'keep me' },
    },
  };
  const { user, calls } = mount((url, init) => {
    if (url === '/api/v1/admin/marketplace')
      return Response.json({ items: [...market.items, skill] });
    if (url.endsWith('/marketplace/skills') && init.method === 'POST')
      return Response.json({ item: skill });
  }, '/admin/marketplace');
  await user.click(await screen.findByRole('button', { name: '技能 (1)' }));
  expect(screen.queryByText('DeepWiki')).not.toBeInTheDocument();
  const edit = screen.getByRole('button', { name: '编辑 Review' });
  await user.click(edit);
  expect(screen.getByLabelText('SKILL.md 内容')).toHaveTextContent('original');
  await user.keyboard('{Escape}');
  expect(edit).toHaveFocus();
  await user.click(edit);
  await editCode(user, 'SKILL.md 内容', 'original revised');
  await user.click(screen.getByRole('button', { name: '保存技能' }));
  expect(await screen.findByRole('status')).toHaveTextContent('已保存');
  expect(
    JSON.parse(
      String(
        calls.find((call) => call.url.endsWith('/marketplace/skills'))?.init
          .body,
      ),
    ).files,
  ).toEqual({ 'SKILL.md': 'original revised', 'reference.md': 'keep me' });
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
  await user.click(await screen.findByRole('button', { name: '插件市场' }));
  expect(await screen.findByText('github-mcp-server')).toBeInTheDocument();
  await user.click(
    screen.getByRole('button', { name: '安装到工具池 github-mcp-server' }),
  );
  const config = await screen.findByLabelText('连接配置（JSON）');
  await user.type(config, '{{"url":"https://mcp.example.com/mcp"}');
  await user.click(await screen.findByRole('button', { name: '安装到工具池' }));
  expect(await screen.findByText('已加入工具池')).toBeInTheDocument();
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

it('keeps the marketplace open until a delayed sync failure is visible', async () => {
  let finish: (response: Response) => void = () => {};
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  mount((url) => (url.endsWith('/marketplace/sync') ? pending : undefined));
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '插件市场' }));
  await user.click(screen.getByRole('button', { name: '同步 GitHub' }));
  await user.keyboard('{Escape}');
  expect(screen.getByRole('dialog', { name: '插件市场' })).toBeVisible();
  finish(Response.json({ error: 'temporarily_unavailable' }, { status: 503 }));
  expect(await screen.findByRole('alert')).toBeVisible();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

it('keeps tool details open until a delayed metadata failure is visible', async () => {
  let finish: (response: Response) => void = () => {};
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  mount((url, init) => {
    if (url.endsWith('/upstream'))
      return Response.json({
        tools: [
          {
            name: 'ping',
            description: 'Ping',
            input_schema: { type: 'object' },
          },
        ],
        overrides: [],
      });
    if (init.method === 'PUT') return pending;
  });
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole('button', { name: '查看参数 Remote tools' }),
  );
  await user.click(
    await screen.findByRole('button', { name: '覆盖描述 ping' }),
  );
  await user.click(screen.getByRole('button', { name: '保存覆盖' }));
  await user.keyboard('{Escape}');
  expect(screen.getByRole('dialog', { name: 'Remote tools' })).toBeVisible();
  finish(Response.json({ error: 'temporarily_unavailable' }, { status: 503 }));
  expect(await screen.findByRole('alert')).toBeVisible();
});

it('syncs GitHub skills with MCP, preserves custom skills and retries partial failures', async () => {
  let fail = true;
  const entries = [
    {
      ...market.items[0],
      id: 3,
      kind: 'skill',
      slug: 'superpowers-debugging',
      name: 'Custom debugging',
      spec: { source: 'inline', files: { 'SKILL.md': 'custom' } },
    },
    {
      ...market.items[0],
      id: 4,
      kind: 'skill',
      slug: 'team-review',
      name: 'Team review',
      spec: { source: 'github', repo: 'team/skills', path: 'review' },
    },
  ];
  const { user, calls } = mount((url, init) => {
    if (url === '/api/v1/admin/marketplace')
      return Response.json({ items: [...market.items, ...entries] });
    if (url.endsWith('/marketplace/sync'))
      return fail
        ? Response.json({ error: 'upstream_unavailable' }, { status: 502 })
        : Response.json({ synced: 8 });
    if (url.endsWith('/marketplace/skills')) {
      const body = JSON.parse(String(init.body));
      if (fail && body.slug === 'superpowers-tdd')
        return Response.json(
          { error: 'upstream_unavailable' },
          { status: 502 },
        );
      return Response.json({
        item: { ...market.items[0], ...body, kind: 'skill' },
      });
    }
  }, '/admin/marketplace');
  await screen.findByText('Team review');
  const button = screen.getByRole('button', { name: '同步 GitHub' });
  button.focus();
  await user.keyboard('{Enter}');
  expect(
    await screen.findByText('同步了 0 个 MCP、2 个技能；失败 2 项'),
  ).toBeVisible();
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Test Driven Development',
  );
  const posts = calls
    .filter((c) => c.url.endsWith('/marketplace/skills'))
    .map((c) => JSON.parse(String(c.init.body)));
  expect(posts.map((p) => p.slug)).toEqual([
    'superpowers-tdd',
    'anthropic-frontend-design',
    'team-review',
  ]);
  expect(posts[2]).toMatchObject({
    source: 'github',
    repo: 'team/skills',
    path: 'review',
    name: 'Team review',
  });
  expect(
    calls.filter((c) => c.url === '/api/v1/admin/marketplace').length,
  ).toBeGreaterThan(1);
  fail = false;
  await user.click(button);
  expect(
    await screen.findByText('同步了 8 个 MCP、3 个技能；失败 0 项'),
  ).toBeVisible();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
