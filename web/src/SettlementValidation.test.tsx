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

function mount() {
  const calls: { url: string; init: RequestInit }[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit = {}) => {
      calls.push({ url, init });
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
      if (url === '/api/v1/admin/marketplace')
        return Response.json({ items: [] });
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

async function openEditor(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '添加工具' }));
}

async function fillBasics(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('工具标识'), 'bizjs');
  await user.type(screen.getByLabelText('显示名称'), 'BizJS');
  await user.click(screen.getByLabelText('连接方式'));
  await user.click(await screen.findByRole('option', { name: 'HTTP 服务' }));
  const config = screen.getByLabelText('连接配置（JSON）');
  await user.type(config, '{{"url":"https://mcp.example.com/mcp"}');
}

it('blocks submit on settlement script syntax errors without calling the server', async () => {
  const { user, calls } = mount();
  await openEditor(user);
  await fillBasics(user);
  const settlement = screen.getByLabelText('结算策略（JSON，可选）');
  await user.type(settlement, '{{"script":"return ???"}');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  expect(await screen.findByText(/结算脚本语法有误/)).toBeInTheDocument();
  expect(
    calls.some(
      (call) =>
        call.url === '/api/v1/admin/tools' && call.init.method === 'POST',
    ),
  ).toBe(false);
});

it('submits a valid settlement script', async () => {
  const { user, calls } = mount();
  await openEditor(user);
  await fillBasics(user);
  const settlement = screen.getByLabelText('结算策略（JSON，可选）');
  await user.type(
    settlement,
    '{{"script":"const body = JSON.parse(result.text); return body.code === 0;"}',
  );
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await new Promise((resolve) => setTimeout(resolve, 50));
  const saved = calls.find(
    (call) => call.url === '/api/v1/admin/tools' && call.init.method === 'POST',
  );
  expect(saved).toBeTruthy();
  const body = JSON.parse(String(saved ? saved.init.body : ''));
  expect(body.settlement.script).toContain('JSON.parse');
});
