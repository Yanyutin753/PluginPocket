import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import App from './App';
import { ApiError } from './features/account/api';
import { chooseOption } from './test/select';

const account = {
  user: {
    id: 1,
    username: 'alice',
    role: 'admin',
    balance: 500,
    enabled: true,
  },
  summary: { today_calls: 0, month_cost: 0, token_count: 0 },
};
const tool = {
  id: 1,
  key: 'echo',
  name: '回声',
  description: '返回提供的消息',
  kind: 'builtin',
  enabled: true,
  units_per_call: 1,
  input_schema: { type: 'object' },
  configured: true,
};
const plan = {
  id: 1,
  name: '开发额度',
  credits: 1000,
  price_cents: 1200,
  currency: 'CNY',
  enabled: true,
  created_at: '2026-09-10T10:00:00Z',
};
const team = {
  id: 1,
  name: '研发组',
  role: 'owner',
  balance: 100,
  seat_limit: 5,
};
function setup(
  path: string,
  handler: (
    url: string,
    init: RequestInit,
  ) => Response | Promise<Response> | undefined,
  role = 'admin',
) {
  const fetch = vi.fn(
    (url: string, init: RequestInit = {}) =>
      handler(url, init) ??
      (url === '/api/v1/account/me'
        ? Response.json({ ...account, user: { ...account.user, role } })
        : url === '/api/v1/meta'
          ? Response.json({ github: false, email: false, payments: false })
          : Response.json({ items: [], next_cursor: '' })),
  );
  vi.stubGlobal('fetch', fetch);
  window.history.replaceState({}, '', path);
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), client, fetch };
}
it('shows the real tool catalog with cost and schema details', async () => {
  const { user } = setup(
    '/tools',
    (url) =>
      url.startsWith('/api/v1/tools')
        ? Response.json({ items: [tool], next_cursor: '' })
        : undefined,
    'user',
  );
  expect(await screen.findByText('回声')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '查看参数 回声' }));
  expect(screen.getByText(/"type": "object"/)).toBeVisible();
  expect(
    screen.queryByRole('button', { name: '添加工具' }),
  ).not.toBeInTheDocument();
});
it('creates a tool and changes its price and enabled state through real forms', async () => {
  let current = { ...tool };
  let exists = false;
  const { user } = setup('/admin/tools', (url, init) => {
    if (!url.startsWith('/api/v1/admin/tools')) return;
    if (init.method === 'POST' || init.method === 'PATCH') {
      if (
        Object.keys(JSON.parse(String(init.body))).some(
          (key) =>
            ![
              'key',
              'name',
              'description',
              'kind',
              'enabled',
              'units_per_call',
              'input_schema',
              'config',
            ].includes(key),
        )
      )
        return Response.json({ error: 'invalid_request' }, { status: 400 });
      current = { ...current, ...JSON.parse(String(init.body)) };
      exists = true;
      return Response.json({ item: current });
    }
    return Response.json({ items: exists ? [current] : [], next_cursor: '' });
  });
  await user.click(await screen.findByRole('button', { name: '添加工具' }));
  await user.type(screen.getByLabelText('工具标识'), 'echo');
  await user.type(screen.getByLabelText('显示名称'), '回声');
  await user.type(screen.getByLabelText('工具说明'), '返回提供的消息');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  expect(await screen.findByText('回声')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '编辑 回声' }));
  await user.clear(screen.getByLabelText('每次调用额度'));
  await user.type(screen.getByLabelText('每次调用额度'), '3');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await user.click(await screen.findByRole('button', { name: '停用 回声' }));
  expect(await screen.findByText('已停用')).toBeVisible();
});
it('disables a user only after explicit confirmation', async () => {
  let enabled = true;
  const { user } = setup('/admin/users', (url, init) => {
    if (!url.startsWith('/api/v1/admin/users')) return;
    if (init.method === 'PATCH') {
      enabled = JSON.parse(String(init.body)).enabled;
      return Response.json({
        user: {
          ...account.user,
          id: 2,
          username: 'bob',
          role: 'user',
          enabled,
        },
      });
    }
    return Response.json({
      items: [
        { ...account.user, id: 2, username: 'bob', role: 'user', enabled },
      ],
      next_cursor: '',
    });
  });
  await user.click(await screen.findByRole('button', { name: '停用 bob' }));
  expect(screen.getByRole('button', { name: '确认停用' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '确认停用' }));
  expect(await screen.findByText('已停用')).toBeVisible();
});
it('redeems credits and refreshes the actual ledger without pretending payment is available', async () => {
  let redeemed = false;
  const { user } = setup(
    '/billing',
    (url, init) => {
      // Even an advertised provider can become unavailable at purchase time.
      if (url === '/api/v1/meta')
        return Response.json({ github: false, email: false, payments: true });
      if (url === '/api/v1/account/redeem') {
        redeemed = true;
        return Response.json({ credits: 100, balance: 600 });
      }
      if (url.startsWith('/api/v1/account/ledger'))
        return Response.json({
          items: redeemed
            ? [
                {
                  id: 2,
                  delta: 100,
                  kind: 'redeem',
                  note: '兑换额度',
                  balance_after: 600,
                  created_at: '2026-09-10T10:00:00Z',
                },
              ]
            : [],
          next_cursor: '',
        });
      if (url.startsWith('/api/v1/plans'))
        return Response.json({ items: [plan], next_cursor: '' });
      if (url.startsWith('/api/v1/account/orders') && init.method === 'POST')
        return Response.json({ error: 'payment_unavailable' }, { status: 503 });
    },
    'user',
  );
  await user.type(await screen.findByLabelText('兑换码'), 'redeem_test');
  await user.click(screen.getByRole('button', { name: '兑换额度' }));
  expect(await screen.findByRole('status')).toHaveTextContent(
    '已兑换 100 额度',
  );
  expect(await screen.findByRole('cell', { name: '兑换额度' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '购买 开发额度' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '在线支付暂未配置',
  );
  expect(screen.queryByText('支付成功')).not.toBeInTheDocument();
});
it('creates and edits an actual plan', async () => {
  let exists = false;
  let current = { ...plan };
  const { user } = setup('/admin/plans', (url, init) => {
    if (!url.startsWith('/api/v1/admin/plans')) return;
    if (init.method === 'POST' || init.method === 'PATCH') {
      if (
        Object.keys(JSON.parse(String(init.body))).some(
          (key) =>
            !['name', 'credits', 'price_cents', 'currency', 'enabled'].includes(
              key,
            ),
        )
      )
        return Response.json({ error: 'invalid_request' }, { status: 400 });
      exists = true;
      current = { ...current, ...JSON.parse(String(init.body)) };
      return Response.json({ item: current });
    }
    return Response.json({ items: exists ? [current] : [], next_cursor: '' });
  });
  await user.click(await screen.findByRole('button', { name: '添加套餐' }));
  await user.type(screen.getByLabelText('套餐名称'), '开发额度');
  await user.type(screen.getByLabelText('额度数量'), '1000');
  await user.type(screen.getByLabelText('价格（分）'), '1200');
  await user.click(screen.getByRole('button', { name: '保存套餐' }));
  expect(await screen.findByText('开发额度')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '停用 开发额度' }));
  expect(await screen.findByText('已停用')).toBeVisible();
});
it('creates a redemption code whose plaintext is shown once and kept out of query caches', async () => {
  const item = {
    id: 1,
    credits: 100,
    note: '活动',
    created_at: '2026-09-10T10:00:00Z',
    redeemed_at: null,
  };
  const { user, client } = setup('/admin/codes', (url, init) =>
    url.startsWith('/api/v1/admin/redemption-codes')
      ? Response.json(
          init.method === 'POST'
            ? { code: 'red_once_secret', item }
            : { items: [item], next_cursor: '' },
        )
      : undefined,
  );
  await user.type(await screen.findByLabelText('兑换额度'), '100');
  await user.type(screen.getByLabelText('备注'), '活动');
  await user.click(screen.getByRole('button', { name: '生成兑换码' }));
  expect(await screen.findByText('red_once_secret')).toBeVisible();
  expect(
    JSON.stringify(
      client
        .getMutationCache()
        .getAll()
        .map((m) => m.state.data),
    ),
  ).not.toContain('red_once_secret');
  await user.click(
    screen.getByRole('button', { name: '我已保存，隐藏兑换码' }),
  );
  expect(screen.queryByText('red_once_secret')).not.toBeInTheDocument();
});
it('creates a team, funds it idempotently and creates an invitation', async () => {
  let exists = false;
  let attempts = 0;
  const requests: unknown[] = [];
  const { user } = setup('/teams', (url, init) => {
    if (url === '/api/v1/account/teams' && init.method === 'POST') {
      exists = true;
      return Response.json({ item: team });
    }
    if (url === '/api/v1/account/teams/1') return Response.json({ item: team });
    if (url.endsWith('/fund')) {
      requests.push(JSON.parse(String(init.body)));
      if (++attempts === 1) return Promise.reject(new Error('offline'));
      return Response.json({ item: { ...team, balance: 200 } });
    }
    if (url.endsWith('/invites'))
      return Response.json({
        code: 'invite_once',
        expires_at: '2026-09-11T10:00:00Z',
      });
    if (url.includes('/members'))
      return Response.json({
        items: [{ id: 1, user_id: 1, username: 'alice', role: 'owner' }],
        next_cursor: '',
      });
    if (url.startsWith('/api/v1/account/teams'))
      return Response.json({ items: exists ? [team] : [], next_cursor: '' });
  });
  await user.type(await screen.findByLabelText('团队名称'), '研发组');
  await user.click(screen.getByRole('button', { name: '创建团队' }));
  await user.click(await screen.findByRole('link', { name: '管理 研发组' }));
  await user.type(await screen.findByLabelText('转入额度'), '100');
  await user.click(screen.getByRole('button', { name: '确认转入' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '暂时无法完成请求',
  );
  await user.click(screen.getByRole('button', { name: '确认转入' }));
  expect(await screen.findByRole('status')).toHaveTextContent(
    '已转入 100 额度',
  );
  expect(requests[0]).toEqual(requests[1]);
  await user.click(screen.getByRole('button', { name: '生成邀请' }));
  expect(await screen.findByText('invite_once')).toBeVisible();
});
it('accepts an invitation and lets a member leave after confirmation', async () => {
  let joined = false;
  const memberTeam = { ...team, role: 'member' };
  const { user } = setup(
    '/teams',
    (url, init) => {
      if (url === '/api/v1/account/teams/1')
        return Response.json({ item: memberTeam });
      if (url.endsWith('/team-invites/accept')) {
        joined = true;
        return Response.json({ item: memberTeam });
      }
      if (url.includes('/members')) {
        if (init.method === 'DELETE') {
          joined = false;
          return new Response(null, { status: 204 });
        }
        return Response.json({
          items: [{ id: 2, user_id: 1, username: 'alice', role: 'member' }],
          next_cursor: '',
        });
      }
      if (url.startsWith('/api/v1/account/teams'))
        return Response.json({
          items: joined ? [memberTeam] : [],
          next_cursor: '',
        });
    },
    'user',
  );
  await user.type(await screen.findByLabelText('团队邀请码'), 'invite_once');
  await user.click(screen.getByRole('button', { name: '加入团队' }));
  await user.click(await screen.findByRole('link', { name: '管理 研发组' }));
  expect(
    screen.queryByRole('button', { name: '生成邀请' }),
  ).not.toBeInTheDocument();
  await user.click(await screen.findByRole('button', { name: '退出团队' }));
  await user.click(screen.getByRole('button', { name: '确认退出' }));
  expect(await screen.findByText('还没有加入团队')).toBeVisible();
});
it('creates a token charged to the selected team wallet', async () => {
  const bodies: unknown[] = [];
  const { user } = setup('/tokens', (url, init) => {
    if (url.startsWith('/api/v1/account/teams'))
      return Response.json({ items: [team], next_cursor: '' });
    if (url.startsWith('/api/v1/account/tokens') && init.method === 'POST') {
      bodies.push(JSON.parse(String(init.body)));
      return Response.json({
        token: 'ldt_team_secret',
        item: {
          id: 1,
          name: '团队电脑',
          prefix: 'ldt_team',
          wallet_id: 2,
          created_at: '2026-09-10T10:00:00Z',
          revoked_at: null,
          last_used_at: null,
        },
      });
    }
  });
  await user.type(await screen.findByLabelText('令牌名称'), '团队电脑');
  await chooseOption(user, screen.getByLabelText('扣费钱包'), '研发组');
  await user.click(screen.getByRole('button', { name: '创建令牌' }));
  expect(await screen.findByText('ldt_team_secret')).toBeVisible();
  expect(bodies).toEqual([{ name: '团队电脑', team_id: 1 }]);
});
it('requires explicit approval before authorizing a local device', async () => {
  let approved = false;
  const { user } = setup(
    '/device?user_code=ABCD-EFGH',
    (url) => {
      if (url.endsWith('/devices/approve')) {
        approved = true;
        return new Response(null, { status: 204 });
      }
    },
    'user',
  );
  expect(await screen.findByLabelText('设备授权码')).toHaveValue('ABCD-EFGH');
  expect(approved).toBe(false);
  await user.click(screen.getByRole('button', { name: '批准此设备' }));
  expect(await screen.findByRole('status')).toHaveTextContent('设备已获授权');
});
it('filters global usage and exports one server-provided CSV page at a time', async () => {
  const { user } = setup('/admin/usage', (url) => {
    if (url.includes('/usage/summary'))
      return Response.json({ items: [], next_cursor: '' });
    if (url.includes('/usage/export'))
      return new Response('工具,额度\necho,1\n', {
        headers: { 'Content-Type': 'text/csv', 'X-Next-Cursor': '8' },
      });
    if (url.includes('/admin/usage'))
      return Response.json({
        items: [
          {
            id: 8,
            user_id: 1,
            tool: 'echo',
            cost: 1,
            status: 'ok',
            duration_ms: 3,
            created_at: '2026-09-10T10:00:00Z',
          },
        ],
        next_cursor: '',
      });
  });
  await user.type(await screen.findByLabelText('工具筛选'), 'echo');
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  expect(await screen.findByText('echo')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '导出 CSV' }));
  const download = await screen.findByRole('link', { name: '下载 CSV' });
  expect(download).toHaveAttribute('download', 'loadout-usage.csv');
  expect(decodeURIComponent(download.getAttribute('href') ?? '')).toContain(
    'echo,1',
  );
  expect(screen.getByRole('button', { name: '导出下一页' })).toBeVisible();
});

it('updates team seats and transfers ownership only after confirmation', async () => {
  let current = { ...team };
  const { user } = setup('/teams/1', (url, init) => {
    if (url === '/api/v1/account/teams/1') {
      if (init.method === 'PATCH') {
        const body = JSON.parse(String(init.body));
        current = {
          ...current,
          ...body,
          role: body.owner_user_id ? 'member' : 'owner',
        };
      }
      return Response.json({ item: current });
    }
    if (url.includes('/members'))
      return Response.json({
        items: [
          { id: 1, user_id: 1, username: 'alice', role: 'owner' },
          { id: 2, user_id: 2, username: 'bob', role: 'member' },
        ],
        next_cursor: '',
      });
  });
  await user.clear(await screen.findByLabelText('团队席位'));
  await user.type(screen.getByLabelText('团队席位'), '8');
  await user.click(screen.getByRole('button', { name: '保存团队设置' }));
  expect(await screen.findByText(/8 个席位/)).toBeVisible();
  await chooseOption(user, screen.getByLabelText('新的所有者'), 'bob');
  await user.click(screen.getByRole('button', { name: '交接所有权' }));
  await user.click(screen.getByRole('button', { name: '确认交接' }));
  expect(await screen.findByText('团队所有权已交接')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: '生成邀请' }),
  ).not.toBeInTheDocument();
});
it('shows actual global usage totals by tool and changes the date range', async () => {
  const { user } = setup('/admin/usage', (url) =>
    url.includes('/usage/summary')
      ? Response.json({
          items: [
            {
              tool: 'echo',
              calls: url.includes('days=1') ? 2 : 9,
              cost: 18,
              errors: 1,
            },
          ],
          next_cursor: '',
        })
      : undefined,
  );
  expect(
    await screen.findByRole('heading', { name: '调用汇总' }),
  ).toBeVisible();
  expect(await screen.findByText('9')).toBeVisible();
  await chooseOption(
    user,
    screen.getByLabelText('汇总时间范围'),
    '今天（UTC）',
  );
  expect(await screen.findByText('2')).toBeVisible();
});
it('preserves tool configuration fields after invalid JSON and never shows a fake saved result', async () => {
  const { user } = setup('/admin/tools', () => undefined);
  await user.click(await screen.findByRole('button', { name: '添加工具' }));
  await user.type(screen.getByLabelText('工具标识'), 'remote');
  await user.type(screen.getByLabelText('显示名称'), '远程文档');
  await chooseOption(user, screen.getByLabelText('连接方式'), 'HTTP 服务');
  await user.type(screen.getByLabelText('连接配置（JSON）'), 'not-json');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('JSON 格式不正确');
  expect(screen.getByLabelText('连接配置（JSON）')).toHaveValue('not-json');
});
it('shows GitHub login only when metadata confirms the provider is configured', async () => {
  setup('/login', (url) =>
    url === '/api/v1/meta'
      ? Response.json({ github: true, email: false, payments: false })
      : undefined,
  );
  expect(
    await screen.findByRole('link', { name: '通过 GitHub 登录' }),
  ).toHaveAttribute('href', '/api/v1/auth/github/start');
});
it('shows an unavailable email configuration without allowing a fake send', async () => {
  setup(
    '/settings',
    (url) =>
      url === '/api/v1/account/email'
        ? Response.json({ email: null, verified_at: null, configured: false })
        : undefined,
    'user',
  );
  expect(await screen.findByText('邮件服务尚未配置')).toBeVisible();
  expect(screen.getByRole('button', { name: '发送验证邮件' })).toBeDisabled();
});
it('recovers from an email rate limit and reports only confirmed delivery', async () => {
  let attempts = 0;
  const { user } = setup(
    '/settings',
    (url) => {
      if (url === '/api/v1/account/email')
        return Response.json({
          email: null,
          verified_at: null,
          configured: true,
        });
      if (url.endsWith('/email/request'))
        return ++attempts === 1
          ? Response.json({ error: 'rate_limited' }, { status: 429 })
          : Response.json({ status: 'sent' }, { status: 202 });
    },
    'user',
  );
  await user.type(
    await screen.findByLabelText('邮箱地址'),
    'alice@example.com',
  );
  await user.click(screen.getByRole('button', { name: '发送验证邮件' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('请求过于频繁');
  await user.click(screen.getByRole('button', { name: '发送验证邮件' }));
  expect(await screen.findByRole('status')).toHaveTextContent('验证邮件已发送');
});
it('verifies email only on confirmation and removes the secret from the URL', async () => {
  let verified = false;
  const { user } = setup('/verify-email?token=verify_once_secret', (url) => {
    if (url.endsWith('/auth/email/verify')) {
      verified = true;
      return Response.json({ status: 'verified' });
    }
  });
  const button = await screen.findByRole('button', { name: '确认验证邮箱' });
  expect(verified).toBe(false);
  await user.click(button);
  expect(await screen.findByRole('status')).toHaveTextContent('邮箱验证成功');
  expect(window.location.search).toBe('');
});
it('returns to a requested device approval after logging in without approving automatically', async () => {
  let signedIn = false;
  let approved = false;
  const { user, client } = setup('/device?user_code=ABCD-EFGH', (url) => {
    if (url.endsWith('/account/me'))
      return signedIn
        ? Response.json(account)
        : Response.json({ error: 'unauthorized' }, { status: 401 });
    if (url.endsWith('/auth/login')) {
      signedIn = true;
      return Response.json({ user: account.user });
    }
    if (url.endsWith('/devices/approve')) {
      approved = true;
      return new Response(null, { status: 204 });
    }
  });
  const username = await screen.findByLabelText('用户名');
  // A second in-flight request can fail after the first redirected to login.
  await expect(
    client.fetchQuery({
      queryKey: ['late-auth-check'],
      queryFn: async () => {
        throw new ApiError(401, 'unauthorized');
      },
    }),
  ).rejects.toThrow();
  await user.type(username, 'alice');
  await user.type(screen.getByLabelText('密码'), 'a-secure-password');
  await user.click(screen.getByRole('button', { name: '登录' }));
  expect(await screen.findByLabelText('设备授权码')).toHaveValue('ABCD-EFGH');
  expect(approved).toBe(false);
});

it('opens the verification URI returned by device authorization', async () => {
  setup('/devices?user_code=ABCD-EFGH', () => undefined, 'user');
  expect(await screen.findByLabelText('设备授权码')).toHaveValue('ABCD-EFGH');
});
it('keeps a team chosen by URL visible even when it is outside the first team page', async () => {
  const { user } = setup('/tokens?team_id=99', (url) =>
    url === '/api/v1/account/teams/99'
      ? Response.json({ item: { ...team, id: 99, name: '远端团队' } })
      : undefined,
  );
  expect(
    await within(await screen.findByLabelText('扣费钱包')).findByText(
      '远端团队',
    ),
  ).toBeVisible();
  screen.getByLabelText('扣费钱包').focus();
  await user.keyboard('{Enter}');
  expect(
    await screen.findByRole('option', { name: '远端团队' }),
  ).toHaveAttribute('aria-selected', 'true');
  await user.keyboard('{Escape}');
  expect(screen.getByLabelText('扣费钱包')).toHaveTextContent('远端团队');
});
it('allows keyboard scrolling of ledger and summary tables', async () => {
  setup('/billing', (url) =>
    url.startsWith('/api/v1/account/ledger')
      ? Response.json({
          items: [
            {
              id: 1,
              delta: 100,
              kind: 'redeem',
              note: '活动额度',
              created_at: '2026-09-10T10:00:00Z',
              balance_after: 100,
            },
          ],
          next_cursor: '',
        })
      : undefined,
  );
  const table = await screen.findByRole('region', { name: '额度流水表格' });
  table.focus();
  expect(table).toHaveFocus();
});
it('displays an actionable expired invitation error without joining the team', async () => {
  const { user } = setup('/teams', (url) =>
    url.endsWith('/team-invites/accept')
      ? Response.json({ error: 'invite_expired' }, { status: 410 })
      : undefined,
  );
  await user.type(await screen.findByLabelText('团队邀请码'), 'expired_code');
  await user.click(screen.getByRole('button', { name: '加入团队' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('邀请码已过期');
  expect(screen.getByText('还没有加入团队')).toBeVisible();
});

it('disables online purchases when server capabilities say payments are unavailable', async () => {
  setup(
    '/billing',
    (url) =>
      url === '/api/v1/meta'
        ? Response.json({ github: false, email: false, payments: false })
        : url.startsWith('/api/v1/plans')
          ? Response.json({ items: [plan], next_cursor: '' })
          : undefined,
    'user',
  );
  expect(
    await screen.findByRole('button', { name: '购买 开发额度' }),
  ).toBeDisabled();
  expect(
    screen.getByText('在线支付暂未配置，可使用兑换码或联系管理员补充额度'),
  ).toBeVisible();
});
it('keeps purchases disabled while capabilities load or fail and allows retry', async () => {
  let complete!: (response: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    complete = resolve;
  });
  let attempts = 0;
  const { user } = setup('/billing', (url) => {
    if (url === '/api/v1/meta') {
      attempts++;
      return attempts === 1
        ? pending
        : Response.json({ github: false, email: false, payments: false });
    }
    if (url.startsWith('/api/v1/plans'))
      return Response.json({ items: [plan], next_cursor: '' });
  });
  const purchase = await screen.findByRole('button', { name: '购买 开发额度' });
  expect(purchase).toBeDisabled();
  complete(
    Response.json({ error: 'temporarily_unavailable' }, { status: 503 }),
  );
  expect(await screen.findByRole('alert')).toBeVisible();
  expect(purchase).toBeDisabled();
  await user.click(screen.getByRole('button', { name: '重试' }));
  expect(
    await screen.findByText(
      '在线支付暂未配置，可使用兑换码或联系管理员补充额度',
    ),
  ).toBeVisible();
  expect(purchase).toBeDisabled();
});
it('filters the administrator ledger and shows actual actor and wallet references', async () => {
  const { user } = setup('/admin/ledger', (url) =>
    url.startsWith('/api/v1/admin/ledger')
      ? Response.json({
          items: [
            {
              id: 7,
              user_id: 2,
              actor_id: 1,
              wallet_id: 2,
              delta: 100,
              kind: 'adjustment',
              note: '人工补充',
              balance_after: 500,
              created_at: '2026-09-10T10:00:00Z',
            },
          ],
          next_cursor: '',
        })
      : undefined,
  );
  await user.type(await screen.findByLabelText('用户编号'), '2');
  await user.click(screen.getByRole('button', { name: '筛选流水' }));
  expect(await screen.findByRole('cell', { name: '人工补充' })).toBeVisible();
  expect(screen.getByRole('columnheader', { name: '操作人' })).toBeVisible();
});
