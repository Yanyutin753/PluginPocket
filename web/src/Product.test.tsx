import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';

const account = {
  user: {
    id: 1,
    username: 'alice',
    role: 'user',
    balance: 4200,
    enabled: true,
  },
  summary: { today_calls: 3, month_cost: 80, token_count: 1 },
};
const token = {
  id: 7,
  name: '工作电脑',
  prefix: 'ldt_demo',
  wallet_id: 1,
  created_at: '2026-09-10T10:00:00Z',
  revoked_at: null,
  last_used_at: null,
};
function mount(path = '/overview') {
  window.history.replaceState({}, '', path);
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const result = render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return { ...result, client };
}
function network(
  handler?: (
    url: string,
    init: RequestInit,
  ) => Response | Promise<Response> | undefined,
) {
  const mock = vi.fn(
    (url: string, init: RequestInit = {}) =>
      handler?.(url, init) ??
      (url === '/api/v1/account/me'
        ? Response.json(account)
        : Response.json({ error: 'not_found' }, { status: 404 })),
  );
  vi.stubGlobal('fetch', mock);
  return mock;
}
beforeEach(() => {
  window.history.replaceState({}, '', '/overview');
});
describe('account console', () => {
  it('logs in with keyboard and shows account values; logout removes account data', async () => {
    let signedIn = false;
    network((url) => {
      if (url.endsWith('/account/me'))
        return signedIn
          ? Response.json(account)
          : Response.json({ error: 'unauthorized' }, { status: 401 });
      if (url.endsWith('/auth/login')) {
        signedIn = true;
        return Response.json({ user: account.user });
      }
      if (url.endsWith('/auth/logout')) {
        signedIn = false;
        return new Response(null, { status: 204 });
      }
    });
    const { client } = mount();
    const user = userEvent.setup();
    await screen.findByRole('heading', { name: '登录 Loadout' });
    await user.click(screen.getByLabelText('用户名'));
    await user.type(screen.getByLabelText('用户名'), 'alice');
    await user.tab();
    expect(screen.getByLabelText('密码')).toHaveFocus();
    await user.keyboard('a-secure-password');
    await user.tab();
    expect(screen.getByRole('button', { name: '显示密码' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: /^登录$/ })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(
      await screen.findByRole('heading', { name: '账号概览' }),
    ).toBeVisible();
    expect(screen.getByText('4,200')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '账号菜单' }));
    await user.click(screen.getByRole('menuitem', { name: '退出登录' }));
    await screen.findByRole('heading', { name: '登录 Loadout' });
    expect(screen.queryByText('4,200')).not.toBeInTheDocument();
    expect(client.getQueryData(['account'])).toBeUndefined();
  });
  it('opens the account menu from the header and restores focus with Escape', async () => {
    network();
    mount();
    const user = userEvent.setup();
    const trigger = await screen.findByRole('button', { name: '账号菜单' });
    expect(
      within(screen.getByRole('banner')).getByRole('button', {
        name: '账号菜单',
      }),
    ).toBe(trigger);
    expect(
      within(screen.getByRole('complementary')).getByRole('link', {
        name: '账号设置',
      }),
    ).toHaveAttribute('href', '/settings');
    trigger.focus();
    await user.keyboard('{Enter}');
    expect(
      await screen.findByRole('menuitem', { name: '退出登录' }),
    ).toBeVisible();
    await user.keyboard('{Escape}');
    expect(
      screen.queryByRole('menuitem', { name: '退出登录' }),
    ).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
  it('registers an account and does not leak server errors before a successful retry', async () => {
    let attempts = 0;
    network((url) => {
      if (url.endsWith('/auth/register'))
        return ++attempts === 1
          ? Response.json(
              { error: 'username_taken', secret: 'private-response' },
              { status: 409 },
            )
          : Response.json({ user: account.user });
    });
    mount('/register');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('用户名'), 'alice');
    await user.type(screen.getByLabelText('密码'), 'a-secure-password');
    await user.click(screen.getByRole('button', { name: '创建账号' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '用户名已被使用',
    );
    expect(screen.queryByText('private-response')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '创建账号' }));
    expect(
      await screen.findByRole('heading', { name: '账号概览' }),
    ).toBeVisible();
  });
  it('recovers from a malformed account response without invented balances', async () => {
    let bad = true;
    network((url) =>
      url.endsWith('/account/me')
        ? Response.json(bad ? { user: { balance: 'bogus' } } : account)
        : undefined,
    );
    mount();
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法完成请求',
    );
    expect(screen.queryByText('4,200')).not.toBeInTheDocument();
    bad = false;
    await userEvent.setup().click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText('4,200')).toBeVisible();
  });
  it('shows a created token once and revokes it after explicit confirmation', async () => {
    let created = false;
    let revoked = false;
    network((url, init) => {
      if (url.startsWith('/api/v1/account/tokens')) {
        if (init.method === 'POST') {
          created = true;
          return Response.json({ token: 'ldt_only_once_secret', item: token });
        }
        if (init.method === 'DELETE') {
          revoked = true;
          return new Response(null, { status: 204 });
        }
        return Response.json({
          items: created
            ? [
                {
                  ...token,
                  revoked_at: revoked ? '2026-09-10T12:00:00Z' : null,
                },
              ]
            : [],
          next_cursor: '',
        });
      }
    });
    const { client } = mount('/tokens');
    const user = userEvent.setup();
    expect(await screen.findByText('还没有令牌')).toBeVisible();
    await user.type(screen.getByLabelText('令牌名称'), '工作电脑');
    await user.click(screen.getByRole('button', { name: '创建令牌' }));
    expect(await screen.findByText('ldt_only_once_secret')).toBeVisible();
    expect(
      JSON.stringify(
        client
          .getQueryCache()
          .getAll()
          .map((q) => q.state.data),
      ),
    ).not.toContain('ldt_only_once_secret');
    await user.click(
      screen.getByRole('button', { name: '我已保存，隐藏令牌' }),
    );
    expect(screen.queryByText('ldt_only_once_secret')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '撤销 工作电脑' }));
    expect(screen.getByRole('button', { name: '确认撤销' })).toBeVisible();
    await user.click(screen.getByRole('button', { name: '确认撤销' }));
    expect(await screen.findByText('已撤销')).toBeVisible();
  });
  it('paginates usage using the server cursor and returns to cached previous records', async () => {
    network((url) =>
      url.startsWith('/api/v1/account/usage')
        ? Response.json(
            url.includes('cursor=8')
              ? {
                  items: [
                    {
                      id: 7,
                      tool: 'echo',
                      cost: 2,
                      status: 'error',
                      duration_ms: 12,
                      created_at: '2026-09-10T10:00:00Z',
                    },
                  ],
                  next_cursor: '',
                }
              : {
                  items: [
                    {
                      id: 8,
                      tool: 'time_now',
                      cost: 1,
                      status: 'ok',
                      duration_ms: 10,
                      created_at: '2026-09-10T11:00:00Z',
                    },
                  ],
                  next_cursor: '8',
                },
          )
        : undefined,
    );
    mount('/usage');
    expect(await screen.findByText('time_now')).toBeVisible();
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: '下一页' }));
    expect(await screen.findByText('echo')).toBeVisible();
    expect(screen.queryByText('time_now')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: '上一页' }));
    expect(await screen.findByText('time_now')).toBeVisible();
    expect(screen.queryByText('echo')).not.toBeInTheDocument();
  });
  it('returns to login and removes protected content when a page returns 401', async () => {
    network((url) =>
      url.startsWith('/api/v1/account/tokens')
        ? Response.json({ error: 'unauthorized' }, { status: 401 })
        : undefined,
    );
    mount('/tokens');
    expect(
      await screen.findByRole('heading', { name: '登录 Loadout' }),
    ).toBeVisible();
    expect(
      screen.queryByRole('button', { name: '创建令牌' }),
    ).not.toBeInTheDocument();
  });
  it('hides admin operations for normal users, including a direct route', async () => {
    network();
    mount('/admin/users');
    expect(
      await screen.findByRole('heading', { name: '没有管理员权限' }),
    ).toBeVisible();
    expect(
      screen.queryByRole('button', { name: '调整余额' }),
    ).not.toBeInTheDocument();
  });
  it('keeps an adjustment idempotency key on uncertain failure and retries the same operation', async () => {
    let attempts = 0;
    const requests: Array<{
      delta: number;
      note: string;
      idempotency_key: string;
    }> = [];
    network((url, init) => {
      if (url.endsWith('/account/me'))
        return Response.json({
          ...account,
          user: { ...account.user, role: 'admin' },
        });
      if (url.endsWith('/admin/users/1/balance')) {
        requests.push(JSON.parse(String(init.body)));
        if (++attempts === 1)
          return Promise.reject(new Error('network private detail'));
        return Response.json({ user: { ...account.user, balance: 4300 } });
      }
      if (url.startsWith('/api/v1/admin/users'))
        return Response.json({ items: [account.user], next_cursor: '' });
    });
    mount('/admin/users');
    const user = userEvent.setup();
    const row = await screen.findByRole('row', { name: /alice/ });
    await user.click(within(row).getByRole('button', { name: '调整余额' }));
    await user.type(screen.getByLabelText('调整额度'), '100');
    await user.type(screen.getByLabelText('备注'), '客服补偿');
    await user.click(screen.getByRole('button', { name: '确认调账' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法完成请求',
    );
    await user.click(screen.getByRole('button', { name: '确认调账' }));
    expect(await screen.findByRole('status')).toHaveTextContent('余额已调整');
    expect(requests).toHaveLength(2);
    expect(requests[0]).toEqual(requests[1]);
    expect(requests[0].idempotency_key).toBeTruthy();
  });
});

describe('recovery and navigation', () => {
  it('rejects a malformed newly created credential instead of displaying it as a usable token', async () => {
    network((url, init) =>
      url.startsWith('/api/v1/account/tokens')
        ? Response.json(
            init.method === 'POST'
              ? { token: 'not-a-gateway-token', item: token }
              : { items: [], next_cursor: '' },
          )
        : undefined,
    );
    mount('/tokens');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('令牌名称'), '工作电脑');
    await user.click(screen.getByRole('button', { name: '创建令牌' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法完成请求',
    );
    expect(screen.queryByText('not-a-gateway-token')).not.toBeInTheDocument();
  });
  it('copies a token and clears its plaintext when navigating away and back', async () => {
    network((url, init) =>
      url.startsWith('/api/v1/account/tokens')
        ? Response.json(
            init.method === 'POST'
              ? { token: 'ldt_only_once_secret', item: token }
              : { items: [token], next_cursor: '' },
          )
        : undefined,
    );
    mount('/tokens');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('令牌名称'), '工作电脑');
    await user.click(screen.getByRole('button', { name: '创建令牌' }));
    await user.click(await screen.findByRole('button', { name: '复制令牌' }));
    expect(await navigator.clipboard.readText()).toBe('ldt_only_once_secret');
    await user.click(screen.getByRole('link', { name: '概览', hidden: true }));
    await screen.findByRole('heading', { name: '账号概览' });
    await user.click(
      screen.getByRole('link', { name: '网关令牌', hidden: true }),
    );
    await screen.findByRole('heading', { name: '网关令牌' });
    expect(screen.queryByText('ldt_only_once_secret')).not.toBeInTheDocument();
  });
  it('opens and closes compact navigation with keyboard actions', async () => {
    network((url) =>
      url.startsWith('/api/v1/account/usage')
        ? Response.json({ items: [], next_cursor: '' })
        : undefined,
    );
    mount();
    const user = userEvent.setup();
    const toggle = await screen.findByRole('button', { name: '导航' });
    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await user.click(screen.getByRole('link', { name: '用量明细' }));
    expect(await screen.findByText('还没有调用记录')).toBeVisible();
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });
  it('keeps the signed-in account after a logout failure and allows retry', async () => {
    let failures = true;
    network((url) =>
      url.endsWith('/auth/logout')
        ? failures
          ? Response.json(
              { error: 'internal_error', debug: 'secret-server-path' },
              { status: 500 },
            )
          : new Response(null, { status: 204 })
        : undefined,
    );
    mount();
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: '账号菜单' }));
    await user.click(screen.getByRole('menuitem', { name: '退出登录' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法完成请求',
    );
    expect(screen.getByText('4,200')).toBeVisible();
    expect(screen.queryByText('secret-server-path')).not.toBeInTheDocument();
    failures = false;
    await user.click(screen.getByRole('button', { name: '账号菜单' }));
    await user.click(screen.getByRole('menuitem', { name: '退出登录' }));
    expect(
      await screen.findByRole('heading', { name: '登录 Loadout' }),
    ).toBeVisible();
  });
  it('cancels an in-flight account request when the app unmounts', () => {
    let signal: AbortSignal | null | undefined;
    network((_url, init) => {
      signal = init.signal;
      return new Promise<Response>(() => {});
    });
    const { unmount } = mount();
    expect(signal?.aborted).toBe(false);
    unmount();
    expect(signal?.aborted).toBe(true);
  });
  it('preserves loaded usage records on a later-page failure and retries the same cursor', async () => {
    let failing = true;
    network((url) =>
      url.startsWith('/api/v1/account/usage')
        ? url.includes('cursor=8')
          ? failing
            ? Response.json({ error: 'internal_error' }, { status: 500 })
            : Response.json({
                items: [
                  {
                    id: 7,
                    tool: 'echo',
                    cost: 0,
                    status: 'error',
                    duration_ms: 10,
                    created_at: '2026-09-10T10:00:00Z',
                  },
                ],
                next_cursor: '',
              })
          : Response.json({
              items: [
                {
                  id: 8,
                  tool: 'time_now',
                  cost: 1,
                  status: 'ok',
                  duration_ms: 10,
                  created_at: '2026-09-10T10:00:00Z',
                },
              ],
              next_cursor: '8',
            })
        : undefined,
    );
    mount('/usage');
    const user = userEvent.setup();
    await screen.findByText('time_now');
    await user.click(screen.getByRole('button', { name: '下一页' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '暂时无法完成请求',
    );
    expect(screen.getByText('time_now')).toBeVisible();
    const table = screen.getByRole('region', { name: '调用明细表格' });
    table.focus();
    expect(table).toHaveFocus();
    failing = false;
    await user.click(screen.getByRole('button', { name: '下一页' }));
    expect(await screen.findByText('echo')).toBeVisible();
  });
});

it('handles an invalid timestamp from a token list as a recoverable API error', async () => {
  network((url) =>
    url.startsWith('/api/v1/account/tokens')
      ? Response.json({
          items: [{ ...token, created_at: 'not-a-date' }],
          next_cursor: '',
        })
      : undefined,
  );
  mount('/tokens');
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '暂时无法完成请求',
  );
  expect(screen.getByRole('button', { name: '重试' })).toBeEnabled();
});

it('collapses and expands the sidebar while keeping navigation accessible', async () => {
  network();
  mount();
  const user = userEvent.setup();
  const collapse = await screen.findByRole('button', { name: '收起侧栏' });
  expect(collapse).toHaveAttribute('aria-expanded', 'true');
  collapse.focus();
  await user.keyboard('{Enter}');
  const expand = screen.getByRole('button', { name: '展开侧栏' });
  expect(expand).toHaveAttribute('aria-expanded', 'false');
  expect(screen.getByRole('link', { name: '用量明细' })).toHaveAttribute(
    'href',
    '/usage',
  );
  await user.keyboard('{Enter}');
  expect(screen.getByRole('button', { name: '收起侧栏' })).toHaveAttribute(
    'aria-expanded',
    'true',
  );
});
