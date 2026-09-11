import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import App from '../App';
import { networkFetch, origin, sessionCookie } from './setup';

const password = 'correct horse battery staple';
const clients: QueryClient[] = [];
afterEach(() => {
  for (const client of clients.splice(0)) client.clear();
});

function mount(path = '/register') {
  cleanup();
  window.history.replaceState({}, '', path);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  clients.push(client);
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}
type User = ReturnType<typeof userEvent.setup>;
async function fill(user: User, name: string, value: string) {
  const input = await screen.findByLabelText(name, { exact: true });
  await user.clear(input);
  await user.type(input, value);
}
async function click(user: User, name: string) {
  await user.click(await screen.findByRole('button', { name }));
}
async function nav(user: User, name: string) {
  await user.click(await screen.findByRole('link', { name }));
}
async function auth(
  user: User,
  username: string,
  register = false,
  credential = password,
) {
  await fill(user, '用户名', username);
  await fill(user, '密码', credential);
  await click(user, register ? '创建账号' : '登录');
  await screen.findByRole('button', { name: '账号菜单' });
}
async function logout(user: User) {
  await click(user, '账号菜单');
  await user.click(await screen.findByRole('menuitem', { name: '退出登录' }));
  await screen.findByRole('heading', { name: '登录 Loadout' });
}
async function api(path: string, init?: RequestInit) {
  const response = await fetch(`/api/v1${path}`, init);
  expect(
    response.ok,
    `${init?.method ?? 'GET'} ${path}: ${response.status}`,
  ).toBe(true);
  return response.status === 204 ? null : response.json();
}
async function bearer(token: string) {
  return networkFetch(`${origin}/api/v1/account/verify`, {
    headers: { Authorization: `Bearer ${token}` },
    signal: AbortSignal.timeout(5_000),
  });
}
function secret(region: string) {
  const value = screen
    .getByRole('region', { name: region })
    .querySelector('code')?.textContent;
  if (!value) throw new Error(`Missing one-time ${region}`);
  return value;
}

test('registers with a real cookie, creates and revokes a token, logs out and restores a protected route', async () => {
  const user = mount();
  const username = `web_${crypto.randomUUID().slice(0, 8)}`;
  await auth(user, username, true);
  const account = await api('/account/me');
  expect(account.user.username).toBe(username);
  expect(document.cookie).toBe('');
  expect(sessionCookie()).not.toBe('');
  await nav(user, '网关令牌');
  await click(user, '创建令牌');
  await fill(user, '令牌名称', 'E2E laptop');
  await user.tab();
  expect(screen.getByRole('combobox', { name: '扣费钱包' })).toHaveFocus();
  await user.tab();
  // The submit control remains reachable and operable using the keyboard.
  const create = await screen.findByRole('button', { name: '创建令牌' });
  expect(create).toHaveFocus();
  await user.keyboard('{Enter}');
  await screen.findByRole('region', { name: '新令牌' });
  const token = secret('新令牌');
  const verified = await bearer(token);
  expect(verified.status).toBe(200);
  expect(await verified.json()).toMatchObject({ username });
  await click(user, '我已保存，隐藏令牌');
  await user.keyboard('{Escape}');
  expect(screen.queryByText(token)).not.toBeInTheDocument();
  expect(JSON.stringify(await api('/account/tokens'))).not.toContain(token);
  await click(user, '撤销 E2E laptop');
  await click(user, '确认撤销');
  await screen.findByText('已撤销');
  expect((await bearer(token)).status).toBe(401);
  const cookie = sessionCookie();
  await logout(user);
  const expired = await networkFetch(`${origin}/api/v1/account/me`, {
    headers: { Cookie: cookie },
    signal: AbortSignal.timeout(5_000),
  });
  expect(expired.status).toBe(401);
  mount('/tokens');
  await screen.findByRole('heading', { name: '登录 Loadout' });
  await fill(user, '用户名', username);
  await fill(user, '密码', 'wrong password');
  await click(user, '登录');
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '用户名或密码不正确',
  );
  await auth(user, username);
  await screen.findByRole('heading', { name: '网关令牌', level: 1 });
  await screen.findByText('已撤销');
  mount('/admin/plans');
  await screen.findByRole('heading', { name: '没有管理员权限' });
  expect(
    screen.queryByRole('link', { name: '用户管理' }),
  ).not.toBeInTheDocument();
  const forbidden = await fetch('/api/v1/admin/plans');
  expect(forbidden.status).toBe(403);
});

test('admin adjusts real credits, manages tools and plans, then a user redeems the issued code once', async () => {
  const user = mount();
  const username = `bill_${crypto.randomUUID().slice(0, 8)}`;
  await auth(user, username, true);
  const before = await api('/account/me');
  await logout(user);
  await auth(
    user,
    process.env.LOADOUT_E2E_ADMIN_USERNAME ?? 'operator',
    false,
    process.env.LOADOUT_E2E_ADMIN_PASSWORD ?? password,
  );
  await nav(user, '用户管理');
  const row = await screen.findByRole('row', { name: new RegExp(username) });
  await user.click(within(row).getByRole('button', { name: '调整余额' }));
  await fill(user, '调整额度', '137');
  await fill(user, '备注', 'Web E2E credit');
  await click(user, '确认调账');
  await screen.findByText(
    new RegExp(
      `余额已调整，当前额度 ${(before.user.balance + 137).toLocaleString('zh-CN')}`,
    ),
  );
  await click(user, '完成');
  await click(user, `停用 ${username}`);
  await click(user, '确认停用');
  await screen.findByRole('button', { name: `启用 ${username}` });
  const disabled = await networkFetch(`${origin}/api/v1/auth/login`, {
    method: 'POST',
    headers: { Origin: origin, 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
    signal: AbortSignal.timeout(5_000),
  });
  expect(disabled.status).toBe(401);
  await click(user, `启用 ${username}`);
  await click(user, '确认启用');
  await screen.findByRole('button', { name: `停用 ${username}` });

  await nav(user, '工具管理');
  const tools = await api('/admin/tools');
  const echo = tools.items.find((item: { key: string }) => item.key === 'echo');
  expect(echo).toBeDefined();
  await click(user, `编辑 ${echo.name}`);
  await fill(user, '每次调用额度', '3');
  await click(user, '保存工具');
  await waitFor(() =>
    expect(screen.queryByLabelText('每次调用额度')).not.toBeInTheDocument(),
  );
  expect(
    (await api('/admin/tools')).items.find(
      (item: { key: string }) => item.key === 'echo',
    ).units_per_call,
  ).toBe(3);
  await click(user, `停用 ${echo.name}`);
  await screen.findByRole('button', { name: `启用 ${echo.name}` });
  expect(
    (await api('/tools')).items.some(
      (item: { key: string }) => item.key === 'echo',
    ),
  ).toBe(false);
  await click(user, `启用 ${echo.name}`);
  await screen.findByRole('button', { name: `停用 ${echo.name}` });

  await nav(user, '套餐管理');
  await click(user, '添加套餐');
  await fill(user, '套餐名称', 'E2E 额度包');
  await fill(user, '额度数量', '211');
  await fill(user, '价格（分）', '500');
  await click(user, '保存套餐');
  await screen.findByRole('heading', { name: 'E2E 额度包' });
  await click(user, '停用 E2E 额度包');
  await screen.findByRole('button', { name: '启用 E2E 额度包' });
  expect(
    (await api('/plans')).items.some(
      (item: { name: string }) => item.name === 'E2E 额度包',
    ),
  ).toBe(false);
  await click(user, '启用 E2E 额度包');
  await screen.findByRole('button', { name: '停用 E2E 额度包' });
  await nav(user, '兑换码管理');
  await click(user, '生成兑换码');
  await fill(user, '兑换额度', '211');
  await fill(user, '备注', 'Web E2E redemption');
  await click(user, '生成兑换码');
  await screen.findByRole('region', { name: '兑换码' });
  const code = secret('兑换码');
  await click(user, '我已保存，隐藏兑换码');
  await user.keyboard('{Escape}');
  expect(screen.queryByText(code)).not.toBeInTheDocument();
  await logout(user);
  await auth(user, username);
  expect(
    screen.queryByRole('link', { name: '用户管理' }),
  ).not.toBeInTheDocument();
  await nav(user, '额度与账单');
  await screen.findByRole('heading', { name: 'E2E 额度包' });
  expect(
    screen.getByRole('button', { name: '购买 E2E 额度包' }),
  ).toBeDisabled();
  await click(user, '兑换额度');
  await fill(user, '兑换码', 'invalid-redemption');
  await click(user, '兑换额度');
  await screen.findByRole('alert');
  await fill(user, '兑换码', code);
  await click(user, '兑换额度');
  await screen.findByText(
    new RegExp(
      `已兑换 211 额度，当前余额 ${(before.user.balance + 348).toLocaleString('zh-CN')}`,
    ),
  );
  expect((await api('/account/me')).user.balance).toBe(
    before.user.balance + 348,
  );
  await screen.findByText('Web E2E credit');
  await fill(user, '兑换码', code);
  await click(user, '兑换额度');
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '此兑换码已经使用',
  );
  expect((await api('/account/me')).user.balance).toBe(
    before.user.balance + 348,
  );
  const ledger = await api('/account/ledger');
  expect(
    ledger.items.filter((item: { delta: number }) => item.delta === 211),
  ).toHaveLength(1);
  await user.keyboard('{Escape}');
  await nav(user, '个人设置');
  await screen.findByText('邮件服务尚未配置');
  await click(user, '邮箱验证');
  expect(screen.getByRole('button', { name: '发送验证邮件' })).toBeDisabled();
});

test('shares a funded team through an invitation and explicitly authorizes a device after session expiry', async () => {
  const user = mount();
  const suffix = crypto.randomUUID().slice(0, 8);
  const owner = `owner_${suffix}`;
  const member = `member_${suffix}`;
  await auth(user, owner, true);
  const before = await api('/account/me');
  await nav(user, '我的团队');
  await click(user, '新建团队');
  await fill(user, '团队名称', 'E2E 开发团队');
  await click(user, '创建团队');
  await waitFor(() =>
    expect(screen.getByLabelText('团队名称')).toHaveValue(''),
  );
  await user.keyboard('{Escape}');
  await nav(user, '管理 E2E 开发团队');
  await click(user, '转入团队额度');
  await fill(user, '转入额度', '23');
  await click(user, '确认转入');
  try {
    await screen.findByText('已转入 23 额度');
  } catch (error) {
    const balances = await Promise.allSettled([
      api('/account/me').then((value) => value.user.balance),
      api('/account/teams').then((value) =>
        value.items.map((team: { balance: number }) => team.balance),
      ),
    ]);
    // Only the funding panel and numeric balances: never log cookies, tokens,
    // invitation codes or complete account/API responses on failure.
    console.error('Team funding diagnostic', {
      panel: screen.queryByRole('heading', { name: '转入团队额度' })
        ?.parentElement?.textContent,
      balances: balances.map((result) =>
        result.status === 'fulfilled' ? result.value : 'request failed',
      ),
    });
    throw error;
  }
  expect((await api('/account/me')).user.balance).toBe(
    before.user.balance - 23,
  );
  await screen.findByText(/共享额度 23/);
  await user.keyboard('{Escape}');
  await click(user, '退出团队');
  await click(user, '确认退出');
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '最后一位团队所有者不能退出',
  );
  await click(user, '取消');
  await click(user, '邀请成员');
  await click(user, '生成邀请');
  await screen.findByRole('region', { name: '邀请码' });
  const invite = secret('邀请码');
  const team = (await api('/account/teams')).items[0];
  await click(user, '我已保存，隐藏邀请码');
  await user.keyboard('{Escape}');
  await logout(user);
  await nav(user, '创建账号');
  await auth(user, member, true);
  await nav(user, '我的团队');
  await click(user, '接受邀请');
  await fill(user, '团队邀请码', invite);
  await click(user, '加入团队');
  await waitFor(() =>
    expect(screen.getByLabelText('团队邀请码')).toHaveValue(''),
  );
  await user.keyboard('{Escape}');
  await nav(user, '管理 E2E 开发团队');
  await screen.findByRole('heading', { name: `${member}（你）` });
  expect(
    screen.queryByRole('button', { name: '生成邀请' }),
  ).not.toBeInTheDocument();
  await nav(user, '创建团队令牌');
  await click(user, '创建令牌');
  await fill(user, '令牌名称', 'E2E team token');
  await click(user, '创建令牌');
  await screen.findByRole('region', { name: '新令牌' });
  const token = secret('新令牌');
  const shared = await bearer(token);
  expect(shared.status).toBe(200);
  expect(await shared.json()).toMatchObject({ username: member, balance: 23 });
  expect((await api(`/account/teams/${team.id}`)).item.balance).toBe(23);
  await click(user, '我已保存，隐藏令牌');
  await user.keyboard('{Escape}');
  await nav(user, '我的团队');
  await user.keyboard('{Escape}');
  await nav(user, '管理 E2E 开发团队');
  await user.keyboard('{Escape}');
  await click(user, '退出团队');
  await click(user, '确认退出');
  await screen.findByText('还没有加入团队');
  expect((await bearer(token)).status).toBe(401);

  const codes = await api('/device/authorize', { method: 'POST', body: '{}' });
  // Expire the real server session; a protected navigation must recover through login.
  await api('/auth/logout', { method: 'POST' });
  mount(`/device?user_code=${encodeURIComponent(codes.user_code)}`);
  await screen.findByRole('heading', { name: '登录 Loadout' });
  await auth(user, member);
  await screen.findByRole('heading', { name: '设备授权', level: 1 });
  expect(screen.getByLabelText('设备授权码')).toHaveValue(codes.user_code);
  await fill(user, '设备授权码', '0000000000');
  await click(user, '批准此设备');
  expect(await screen.findByRole('alert')).toHaveTextContent('设备授权码无效');
  await fill(user, '设备授权码', codes.user_code);
  await click(user, '批准此设备');
  await screen.findByText('设备已获授权，请回到本地设备完成登录');
  const issued = await api('/device/token', {
    method: 'POST',
    body: JSON.stringify({ device_code: codes.device_code }),
  });
  const device = await bearer(issued.token);
  expect(device.status).toBe(200);
  expect(await device.json()).toMatchObject({
    username: member,
    balance: before.user.balance,
  });
  const replay = await fetch('/api/v1/device/token', {
    method: 'POST',
    body: JSON.stringify({ device_code: codes.device_code }),
  });
  expect(replay.status).toBe(400);
  expect(await replay.json()).toEqual({ error: 'invalid_grant' });
});
