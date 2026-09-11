import { readFileSync } from 'node:fs';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { ApiError } from './features/account/api';
import { chooseOption } from './test/select';

const items = [
  {
    slug: 'deepwiki',
    name: 'DeepWiki',
    description: 'Repository docs',
    kind: 'mcp',
    version: '1.0.0',
    gateway: false,
  },
  {
    slug: 'commit-style',
    name: 'Commit style',
    description: 'Commit conventions',
    kind: 'skill',
    version: '1.0.0',
    gateway: false,
  },
];
function mount(path = '/plugins') {
  window.history.replaceState({}, '', path);
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
beforeEach(() => localStorage.clear());

function luminance(hex: string) {
  const channels = hex
    .trim()
    .slice(1)
    .match(/.{2}/g)
    ?.map((part) => {
      const value = Number.parseInt(part, 16) / 255;
      return value <= 0.04045
        ? value / 12.92
        : ((value + 0.055) / 1.055) ** 2.4;
    });
  if (channels?.length !== 3) throw new Error(`Invalid color: ${hex}`);
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}

it.each([false, true])(
  'adapts catalog colors when switching themes (authenticated: %s)',
  async (authenticated) => {
    const style = document.createElement('style');
    style.textContent = readFileSync('src/features/PluginsPage.css', 'utf8');
    document.head.append(style);
    vi.stubGlobal('fetch', (url: string) =>
      Promise.resolve(
        url.endsWith('/account/me')
          ? authenticated
            ? Response.json({
                user: {
                  id: 7,
                  username: 'market-user',
                  role: 'user',
                  balance: 100,
                  enabled: true,
                },
                summary: { today_calls: 0, month_cost: 0, token_count: 0 },
              })
            : Response.json({ error: 'signed_out' }, { status: 401 })
          : Response.json({ items, origin: '' }),
      ),
    );
    try {
      const { container } = mount();
      await screen.findByRole('link', { name: 'DeepWiki' });
      const surface = container.querySelector(
        authenticated ? '.workspace-catalog' : '.public-catalog',
      );
      if (!surface) throw new Error('Catalog not mounted');
      const user = userEvent.setup();
      for (const [label, dark] of [
        ['深色', true],
        ['浅色', false],
      ] as const) {
        await chooseOption(
          user,
          screen.getByRole('combobox', { name: '外观' }),
          label,
        );
        const currentSurface = container.querySelector(
          authenticated ? '.workspace-catalog' : '.public-catalog',
        );
        if (!currentSurface) throw new Error('Catalog not mounted');
        const computed = getComputedStyle(currentSurface);
        const artwork = currentSurface.querySelector('.catalog-hero-art');
        if (!artwork) throw new Error('Catalog artwork not mounted');
        expect(['', 'none']).toContain(getComputedStyle(artwork).filter);
        const background = luminance(
          computed.getPropertyValue('--catalog-showcase'),
        );
        if (dark) expect(background).toBeLessThan(0.1);
        else expect(background).toBeGreaterThan(0.7);
        for (const token of ['--catalog-ink', '--catalog-secondary']) {
          const foreground = luminance(computed.getPropertyValue(token));
          expect(
            (Math.max(background, foreground) + 0.05) /
              (Math.min(background, foreground) + 0.05),
          ).toBeGreaterThanOrEqual(4.5);
        }
        expect(
          screen.getByRole('searchbox', { name: '搜索插件' }),
        ).toBeEnabled();
      }
    } finally {
      style.remove();
    }
  },
);

it.each(['/plugins', '/plugins/deepwiki'])(
  'keeps the workspace sidebar on authenticated market route %s',
  async (path) => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) =>
        Promise.resolve(
          Response.json(
            url.endsWith('/account/me')
              ? {
                  user: {
                    id: 7,
                    username: 'market-user',
                    role: 'user',
                    balance: 100,
                    enabled: true,
                  },
                  summary: { today_calls: 0, month_cost: 0, token_count: 0 },
                }
              : url.endsWith('/plugins/deepwiki')
                ? { item: items[0], origin: '' }
                : { items, origin: '' },
          ),
        ),
      ),
    );
    mount(path);
    expect(await screen.findByRole('link', { name: '网关令牌' })).toBeVisible();
    expect(screen.getByRole('button', { name: '账号菜单' })).toBeVisible();
    expect(
      screen.queryByRole('link', { name: '创建账号' }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(
      await screen.findByRole('heading', {
        name: path === '/plugins' ? '插件市场' : 'DeepWiki',
        level: 1,
      }),
    ).toBeVisible();
  },
);

it('keeps the anonymous catalog available without a workspace sidebar', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        url.endsWith('/account/me')
          ? Response.json({ error: 'signed_out' }, { status: 401 })
          : Response.json({ items, origin: '' }),
      ),
    ),
  );
  mount();
  expect(await screen.findByRole('link', { name: 'DeepWiki' })).toBeVisible();
  expect(screen.getByRole('link', { name: '创建账号' })).toBeVisible();
  expect(
    screen.queryByRole('link', { name: '网关令牌' }),
  ).not.toBeInTheDocument();
  expect(window.location.pathname).toBe('/plugins');
});

const manyItems = Array.from({ length: 29 }, (_, index) => ({
  ...items[0],
  slug: `plugin-${index + 1}`,
  name: `Plugin ${index + 1}`,
  kind: index < 25 ? 'mcp' : 'skill',
}));

function mockCatalog() {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(Response.json({ items: manyItems, origin: '' })),
    ),
  );
}

it('paginates the catalog with keyboard controls and shareable page URLs', async () => {
  mockCatalog();
  mount();
  expect(await screen.findByRole('link', { name: 'Plugin 1' })).toBeVisible();
  expect(
    screen.queryByRole('link', { name: 'Plugin 13' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled();
  const next = screen.getByRole('button', { name: '下一页' });
  next.focus();
  await userEvent.setup().keyboard('{Enter}');
  expect(screen.getByRole('link', { name: 'Plugin 13' })).toBeVisible();
  expect(window.location.search).toBe('?page=2');
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: '第 3 页' }));
  expect(screen.getByRole('link', { name: 'Plugin 29' })).toBeVisible();
  expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
  expect(screen.getByRole('button', { name: '第 3 页' })).toHaveAttribute(
    'aria-current',
    'page',
  );
});

it('resets pagination when searching or changing plugin type', async () => {
  mockCatalog();
  mount('/plugins?page=3');
  expect(await screen.findByRole('link', { name: 'Plugin 29' })).toBeVisible();
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '技能' }));
  expect(window.location.search).toBe('?kind=skill');
  expect(screen.getByRole('link', { name: 'Plugin 26' })).toBeVisible();
  expect(
    screen.queryByRole('navigation', { name: '插件分页' }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '全部' }));
  await user.click(screen.getByRole('button', { name: '下一页' }));
  await user.type(screen.getByRole('searchbox'), 'Plugin 29');
  expect(new URLSearchParams(window.location.search).has('page')).toBe(false);
  expect(screen.getByRole('link', { name: 'Plugin 29' })).toBeVisible();
});

it.each(['-1', 'abc', '1.5', '999'])(
  'handles an invalid or out-of-range page %s',
  async (page) => {
    mockCatalog();
    mount(`/plugins?page=${page}`);
    const expected = page === '999' ? 'Plugin 29' : 'Plugin 1';
    expect(await screen.findByRole('link', { name: expected })).toBeVisible();
    expect(
      screen.queryByRole('link', { name: 'Plugin 13' }),
    ).not.toBeInTheDocument();
  },
);

it('keeps the public catalog open when an earlier private session has expired', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  await client
    .fetchQuery({
      queryKey: ['expired-account'],
      queryFn: async () => {
        throw new ApiError(401, 'unauthorized');
      },
    })
    .catch(() => undefined);
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(Response.json({ items, origin: '' }))),
  );
  window.history.replaceState({}, '', '/plugins');
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole('link', { name: 'DeepWiki' })).toBeVisible();
  expect(window.location.pathname).toBe('/plugins');
});

it('browses and filters the public marketplace with an optional session check', async () => {
  const fetcher = vi.fn((_url: string) =>
    Promise.resolve(
      Response.json({ items, origin: 'https://pluginpocket.example' }),
    ),
  );
  vi.stubGlobal('fetch', fetcher);
  mount();
  expect(
    await screen.findByRole('heading', { name: '插件市场', level: 1 }),
  ).toBeVisible();
  expect(await screen.findByRole('link', { name: 'DeepWiki' })).toHaveAttribute(
    'href',
    '/plugins/deepwiki',
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '技能' }));
  expect(
    screen.queryByRole('link', { name: 'DeepWiki' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Commit style' })).toBeVisible();
  await user.type(
    screen.getByRole('searchbox', { name: '搜索插件' }),
    'missing',
  );
  expect(screen.getByText('没有匹配的插件')).toBeVisible();
  expect(
    fetcher.mock.calls.every(
      ([url]) =>
        String(url).startsWith('/api/v1/plugins') ||
        String(url) === '/api/v1/account/me',
    ),
  ).toBe(true);
});

it('opens anonymous plugin details and exposes installation commands', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        Response.json({
          item: items[0],
          origin: 'https://pluginpocket.example',
        }),
      ),
    ),
  );
  mount('/plugins/deepwiki');
  expect(
    await screen.findByRole('heading', { name: 'DeepWiki', level: 1 }),
  ).toBeVisible();
  expect(
    screen.getByText('codex plugin add deepwiki --marketplace pluginpocket', {
      exact: false,
    }),
  ).toBeVisible();
  expect(screen.getByRole('link', { name: '返回插件市场' })).toHaveAttribute(
    'href',
    '/plugins',
  );
});

it('recovers a public catalog failure without redirecting to login', async () => {
  let failing = true;
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        failing
          ? Response.json({ error: 'unavailable' }, { status: 503 })
          : Response.json({ items, origin: '' }),
      ),
    ),
  );
  mount();
  expect(await screen.findByRole('alert')).toBeVisible();
  failing = false;
  await userEvent.setup().click(screen.getByRole('button', { name: '重试' }));
  expect(await screen.findByRole('link', { name: 'DeepWiki' })).toBeVisible();
  expect(window.location.pathname).toBe('/plugins');
});
