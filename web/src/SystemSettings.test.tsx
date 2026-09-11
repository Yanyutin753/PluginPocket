import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { chooseOption } from './test/select';

beforeEach(() => localStorage.clear());

it('edits access and refresh lifetimes and saves them through the runtime settings API', async () => {
  let body: Record<string, unknown> = {};
  const user = mount((init) => {
    if (init.method === 'PATCH') body = JSON.parse(String(init.body));
    return Response.json({
      item: {
        ...defaults,
        access_token_seconds: 900,
        refresh_token_seconds: 604800,
        ...body,
      },
      secret_writes_available: true,
    });
  });
  await user.click(await screen.findByRole('button', { name: '编辑系统配置' }));
  const access = await screen.findByLabelText('AT 有效期（秒）');
  const refresh = screen.getByLabelText('RT 有效期（秒）');
  expect(access).toHaveValue(900);
  expect(refresh).toHaveValue(604800);
  await user.clear(access);
  await user.type(access, '120');
  await user.clear(refresh);
  await user.type(refresh, '3600');
  screen.getByRole('button', { name: '保存系统配置' }).focus();
  await user.keyboard('{Enter}');
  expect(await screen.findByRole('status')).toHaveTextContent('配置已保存');
  expect(body).toMatchObject({
    access_token_seconds: 120,
    refresh_token_seconds: 3600,
  });
});

const defaults = {
  revision: 0,
  initial_credits: 1000,
  github_enabled: true,
  github_client_id: 'client-id',
  github_org: 'example-org',
  github_client_secret_set: true,
  smtp_enabled: true,
  smtp_address: 'smtp.example.com:587',
  smtp_from: 'Loadout <mail@example.com>',
  smtp_username: 'mailer',
  smtp_password_set: true,
};

function mount(handler: (init: RequestInit) => Response, role = 'admin') {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit = {}) => {
      if (url === '/api/v1/account/me')
        return Response.json({
          user: {
            id: 1,
            username: 'operator',
            role,
            balance: 0,
            enabled: true,
          },
          summary: { today_calls: 0, month_cost: 0, token_count: 0 },
        });
      if (url === '/api/v1/admin/settings') return handler(init);
      return Response.json({ github: true, email: true, payments: false });
    }),
  );
  window.history.replaceState({}, '', '/admin/settings');
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

it('saves runtime settings with keyboard, preserves blank secrets and retries without losing edits', async () => {
  let item = { ...defaults };
  let attempts = 0;
  const user = mount((init) => {
    if (init.method === 'PATCH') {
      const patch = JSON.parse(String(init.body));
      if (
        patch.github_client_secret !== undefined ||
        patch.smtp_password !== undefined
      ) {
        return Response.json({ error: 'invalid_request' }, { status: 400 });
      }
      if (++attempts === 1)
        return Response.json(
          { error: 'temporarily_unavailable' },
          { status: 503 },
        );
      item = { ...item, initial_credits: patch.initial_credits, revision: 1 };
    }
    return Response.json({ item, secret_writes_available: true });
  });
  await user.click(await screen.findByRole('button', { name: '编辑系统配置' }));
  const credits = await screen.findByLabelText('注册赠送额度');
  expect(credits).toHaveValue(1000);
  expect(screen.getByLabelText('GitHub Client Secret')).toHaveValue('');
  expect(screen.getByLabelText('SMTP 密码')).toHaveValue('');
  await user.clear(credits);
  await user.type(credits, '42');
  screen.getByRole('button', { name: '保存系统配置' }).focus();
  await user.keyboard('{Enter}');
  expect(await screen.findByRole('alert')).toBeVisible();
  expect(credits).toHaveValue(42);
  await user.click(screen.getByRole('button', { name: '保存系统配置' }));
  expect(await screen.findByRole('status')).toHaveTextContent('配置已保存');
  expect(screen.getByLabelText('注册赠送额度')).toHaveValue(42);
});

it('reports settings conflicts and explicitly reloads the current revision', async () => {
  let conflict = false;
  const user = mount((init) => {
    if (init.method === 'PATCH') {
      conflict = true;
      return Response.json({ error: 'settings_conflict' }, { status: 409 });
    }
    return Response.json({
      item: {
        ...defaults,
        revision: conflict ? 2 : 0,
        initial_credits: conflict ? 55 : 1000,
      },
      secret_writes_available: true,
    });
  });
  await user.click(await screen.findByRole('button', { name: '编辑系统配置' }));
  const credits = await screen.findByLabelText('注册赠送额度');
  await user.clear(credits);
  await user.type(credits, '42');
  await user.click(screen.getByRole('button', { name: '保存系统配置' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('其他管理员');
  expect(credits).toHaveValue(42);
  await user.click(screen.getByRole('button', { name: '重新加载配置' }));
  expect(await screen.findByLabelText('注册赠送额度')).toHaveValue(55);
});

it('keeps system settings behind the administrator route', async () => {
  mount(
    () => Response.json({ item: defaults, secret_writes_available: true }),
    'user',
  );
  expect(
    await screen.findByRole('heading', { name: '没有管理员权限' }),
  ).toBeVisible();
  expect(
    screen.queryByRole('link', { name: '系统配置' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByLabelText('GitHub Client Secret'),
  ).not.toBeInTheDocument();
});

it('localizes system settings and recovers from a failed load', async () => {
  localStorage.setItem('loadout.locale', 'en');
  let failed = false;
  const user = mount(() => {
    if (!failed) {
      failed = true;
      return Response.json(
        { error: 'temporarily_unavailable' },
        { status: 503 },
      );
    }
    return Response.json({ item: defaults, secret_writes_available: false });
  });
  expect(
    await screen.findByRole('heading', { name: 'System settings' }),
  ).toBeVisible();
  expect(await screen.findByRole('alert')).toBeVisible();
  await user.click(screen.getByRole('button', { name: 'Retry' }));
  await user.click(
    await screen.findByRole('button', { name: 'Edit system settings' }),
  );
  expect(await screen.findByLabelText('Registration credits')).toHaveValue(
    1000,
  );
  expect(screen.getByLabelText('GitHub Client Secret')).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Save system settings' }),
  ).toBeVisible();
});

it('replaces a secret and explicitly clears a disabled integration credential', async () => {
  const user = mount((init) => {
    if (init.method === 'PATCH') {
      const body = JSON.parse(String(init.body));
      if (
        body.github_client_secret !== 'replacement-secret' ||
        body.smtp_password !== '' ||
        body.smtp_enabled !== false
      ) {
        return Response.json({ error: 'invalid_request' }, { status: 400 });
      }
      return Response.json({
        item: {
          ...defaults,
          revision: 1,
          smtp_enabled: false,
          smtp_password_set: false,
        },
        secret_writes_available: true,
      });
    }
    return Response.json({ item: defaults, secret_writes_available: true });
  });
  await user.click(await screen.findByRole('button', { name: '编辑系统配置' }));
  await user.type(
    await screen.findByLabelText('GitHub Client Secret'),
    'replacement-secret',
  );
  await chooseOption(user, screen.getByLabelText('启用邮件服务'), '停用');
  await user.click(
    screen.getByRole('checkbox', { name: '清除已保存的密钥 (SMTP 密码)' }),
  );
  await user.click(screen.getByRole('button', { name: '保存系统配置' }));
  expect(await screen.findByRole('status')).toHaveTextContent('配置已保存');
  expect(screen.getByLabelText('GitHub Client Secret')).toHaveValue('');
  expect(
    screen.queryByRole('checkbox', { name: '清除已保存的密钥 (SMTP 密码)' }),
  ).not.toBeInTheDocument();
});
