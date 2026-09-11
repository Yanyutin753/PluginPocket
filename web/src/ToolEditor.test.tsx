import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { expect, it, vi } from 'vitest';
import ToolsPage from './features/operations/ToolsPage';
import { PreferencesProvider } from './i18n';
import { chooseOption } from './test/select';

const initial = {
  id: 7,
  key: 'search',
  name: 'Search',
  description: 'Search documents',
  kind: 'http',
  enabled: true,
  units_per_call: 2,
  input_schema: { type: 'object', properties: {} },
  configured: true,
  settlement: {},
  icon: '',
};
function mount(overrides = {}, failSave = false) {
  let tool = { ...initial, ...overrides };
  const bodies: Record<string, unknown>[] = [];
  const previews: Record<string, unknown>[] = [];
  let attempts = 0;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    if (url.endsWith('/settlement-preview')) {
      previews.push(JSON.parse(String(init.body)));
      return Response.json({ charge: false });
    }
    if (init.method === 'PATCH') {
      if (failSave && attempts++ === 0)
        return Response.json({ error: 'internal_error' }, { status: 500 });
      const body = JSON.parse(String(init.body));
      bodies.push(body);
      tool = { ...tool, ...body };
      return Response.json({ item: tool });
    }
    return Response.json({ items: [tool], next_cursor: '' });
  });
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={client}>
      <PreferencesProvider>
        <MemoryRouter>
          <ToolsPage admin />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), bodies, previews };
}

it('builds a business success rule, saves it and restores fields when reopened', async () => {
  const { user, bodies } = mount();
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await chooseOption(user, screen.getByLabelText('扣费条件'), 'JSON 字段匹配');
  await user.clear(screen.getByLabelText('成功字段路径'));
  await user.type(screen.getByLabelText('成功字段路径'), 'data.code');
  await user.clear(screen.getByLabelText('成功值（JSON）'));
  await user.type(screen.getByLabelText('成功值（JSON）'), '[[0,200]');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(bodies[0].settlement).toEqual({
    content: { path: 'data.code', equals: [0, 200] },
  });
  expect(bodies[0]).not.toHaveProperty('config');
  await user.click(screen.getByRole('button', { name: '编辑 Search' }));
  expect(screen.getByLabelText('成功字段路径')).toHaveValue('data.code');
  expect(screen.getByLabelText('成功值（JSON）')).toHaveValue('[0,200]');
  await chooseOption(user, screen.getByLabelText('扣费条件'), '协议成功即扣费');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() => expect(bodies).toHaveLength(2));
  expect(bodies[1].settlement).toEqual({});
});

it('previews with the server engine and clears stale results when the sample changes', async () => {
  const { user, previews, bodies } = mount({
    settlement: { content: { path: 'code', equals: 0 } },
  });
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await user.type(screen.getByLabelText('上游返回文本'), '{{"code":500}');
  await user.click(screen.getByRole('button', { name: '试算规则' }));
  expect(await screen.findByRole('status')).toHaveTextContent(
    '不扣费，退回预留额度',
  );
  expect(previews[0]).toEqual({
    settlement: { content: { path: 'code', equals: 0 } },
    text: '{"code":500}',
    is_error: false,
  });
  expect(bodies).toHaveLength(0);
  await user.type(screen.getByLabelText('上游返回文本'), ' ');
  expect(screen.queryByText('不扣费，退回预留额度')).not.toBeInTheDocument();
});

it('keeps field errors and entered values actionable across invalid JSON and save failure', async () => {
  const { user, bodies } = mount({}, true);
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await chooseOption(user, screen.getByLabelText('扣费条件'), 'JSON 字段匹配');
  await user.clear(screen.getByLabelText('成功值（JSON）'));
  await user.type(screen.getByLabelText('成功值（JSON）'), 'oops');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('成功值');
  expect(bodies).toHaveLength(0);
  await user.clear(screen.getByLabelText('成功值（JSON）'));
  await user.type(screen.getByLabelText('成功值（JSON）'), '0');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    '服务内部错误，请稍后重试',
  );
  expect(screen.getByLabelText('成功值（JSON）')).toHaveValue('0');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(bodies).toHaveLength(1);
});

it('builds HTTP config from named fields and leaves stored credentials untouched otherwise', async () => {
  const { user, bodies } = mount();
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await user.click(screen.getByLabelText('更新连接配置'));
  await user.type(
    screen.getByLabelText('MCP 服务地址'),
    'https://mcp.example.com/mcp',
  );
  await user.type(screen.getByLabelText('Bearer 令牌（可选）'), 'sample-token');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() => expect(bodies).toHaveLength(1));
  expect(bodies[0].config).toEqual({
    url: 'https://mcp.example.com/mcp',
    headers: { Authorization: 'Bearer sample-token' },
  });
  await user.click(screen.getByRole('button', { name: '编辑 Search' }));
  expect(screen.queryByDisplayValue('sample-token')).not.toBeInTheDocument();
  await user.keyboard('{Escape}');
  expect(screen.getByRole('button', { name: '编辑 Search' })).toHaveFocus();
});

it('provides a parameter example that is saved only on explicit submission', async () => {
  const { user, bodies } = mount();
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await user.click(screen.getByRole('button', { name: '填入文本参数示例' }));
  expect(
    screen.getByLabelText<HTMLTextAreaElement>('参数 Schema（JSON）').value,
  ).toContain('"query"');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() => expect(bodies).toHaveLength(1));
  expect(bodies[0].input_schema).toMatchObject({
    type: 'object',
    properties: { query: { type: 'string' } },
    required: ['query'],
  });
});

it('keeps the current rule when switching to advanced JSON', async () => {
  const { user, bodies } = mount({
    settlement: { content: { path: 'data.code', equals: [0, 200] } },
  });
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  await user.clear(screen.getByLabelText('成功字段路径'));
  await user.type(screen.getByLabelText('成功字段路径'), 'result.code');
  await chooseOption(user, screen.getByLabelText('扣费条件'), '高级 JSON');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() => expect(bodies).toHaveLength(1));
  expect(bodies[0].settlement).toEqual({
    content: { path: 'result.code', equals: [0, 200] },
  });
});

it('does not display an old preview result after editing a pending sample', async () => {
  const { user } = mount();
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  let finish: (response: Response) => void = () => {};
  vi.stubGlobal(
    'fetch',
    () =>
      new Promise<Response>((resolve) => {
        finish = resolve;
      }),
  );
  await user.type(screen.getByLabelText('上游返回文本'), 'first');
  await user.click(screen.getByRole('button', { name: '试算规则' }));
  await user.type(screen.getByLabelText('上游返回文本'), ' changed');
  finish(Response.json({ charge: true }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '试算规则' })).toBeEnabled(),
  );
  expect(
    screen.queryByText('满足条件，将扣除每次调用额度'),
  ).not.toBeInTheDocument();
});

it('preserves an explicit null success value when editing unrelated tool information', async () => {
  const { user, bodies } = mount({
    settlement: { content: { path: 'error', equals: null } },
  });
  await user.click(await screen.findByRole('button', { name: '编辑 Search' }));
  expect(screen.getByLabelText('成功值（JSON）')).toHaveValue('null');
  await user.type(screen.getByLabelText('显示名称'), ' updated');
  await user.click(screen.getByRole('button', { name: '保存工具' }));
  await waitFor(() => expect(bodies).toHaveLength(1));
  expect(bodies[0].settlement).toEqual({
    content: { path: 'error', equals: null },
  });
});
