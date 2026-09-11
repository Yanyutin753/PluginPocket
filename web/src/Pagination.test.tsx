import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { chooseOption } from './test/select';

const record = (id: number, tool = `tool_${id}`) => ({
  id,
  tool,
  cost: 1,
  status: 'ok',
  duration_ms: 12,
  created_at: '2026-09-11T00:00:00Z',
});
function mount() {
  window.history.replaceState({}, '', '/usage');
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false, gcTime: 0 } },
        })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
function network() {
  const calls: URL[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string) => {
      const url = new URL(raw, 'http://localhost');
      if (url.pathname.endsWith('/account/me'))
        return Response.json({
          user: {
            id: 1,
            username: 'alice',
            role: 'user',
            balance: 100,
            enabled: true,
          },
          summary: { today_calls: 1, month_cost: 1, token_count: 1 },
        });
      if (url.pathname.endsWith('/account/usage')) {
        calls.push(url);
        if (url.searchParams.get('tool') === 'empty')
          return Response.json({ items: [], next_cursor: '' });
        if (url.searchParams.get('tool'))
          return Response.json({
            items: [record(99, 'filtered')],
            next_cursor: '',
          });
        const start = Number(url.searchParams.get('cursor') || 0),
          limit = Number(url.searchParams.get('limit'));
        const end = Math.min(start + limit, 25);
        return Response.json({
          items: Array.from({ length: end - start }, (_, i) =>
            record(start + i + 1),
          ),
          next_cursor: end < 25 ? String(end) : '',
        });
      }
      return Response.json({ items: [], next_cursor: '' });
    }),
  );
  return calls;
}
beforeEach(() => localStorage.clear());
it('changes page size, uses actual cursors, and keeps only one page visible', async () => {
  const calls = network();
  mount();
  const user = userEvent.setup();
  await screen.findByText('tool_1');
  let nav = screen.getByRole('navigation', { name: '调用明细分页' });
  expect(within(nav).getByText('第 1–10 条')).toBeVisible();
  expect(calls[0].searchParams.get('limit')).toBe('10');
  await user.click(within(nav).getByRole('button', { name: '下一页' }));
  await screen.findByText('tool_11');
  expect(screen.getByRole('region', { name: '调用明细表格' })).toHaveFocus();
  expect(screen.queryByText('tool_1')).not.toBeInTheDocument();
  expect(within(nav).getByText('第 11–20 条')).toBeVisible();
  await user.click(within(nav).getByRole('button', { name: '上一页' }));
  await screen.findByText('tool_1');
  expect(calls).toHaveLength(2);
  await chooseOption(
    user,
    within(nav).getByLabelText('每页条数'),
    '20 条 / 页',
  );
  await screen.findByText('tool_20');
  nav = screen.getByRole('navigation', { name: '调用明细分页' });
  expect(within(nav).getByText('第 1 页')).toBeVisible();
  expect(calls.at(-1)?.searchParams.get('limit')).toBe('20');
  expect(calls.at(-1)?.searchParams.get('cursor')).toBe('');
  await user.click(within(nav).getByRole('button', { name: '下一页' }));
  await screen.findByText('tool_25');
  expect(within(nav).getByText('第 21–25 条')).toBeVisible();
  expect(within(nav).getByRole('button', { name: '下一页' })).toBeDisabled();
});
it('resets to page one when filters change and when returning to an earlier filter', async () => {
  network();
  mount();
  const user = userEvent.setup();
  await screen.findByText('tool_1');
  await user.click(screen.getByRole('button', { name: '下一页' }));
  await screen.findByText('tool_11');
  await user.type(screen.getByLabelText('工具筛选'), 'filtered');
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  await screen.findByText('filtered');
  expect(screen.getByText('第 1 页')).toBeVisible();
  await user.clear(screen.getByLabelText('工具筛选'));
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  await waitFor(() => expect(screen.getByText('tool_1')).toBeVisible());
  expect(screen.getByText('第 1 页')).toBeVisible();
  expect(screen.queryByText('tool_11')).not.toBeInTheDocument();
});

it('clears applied filters and restores the input and first page', async () => {
  network();
  mount();
  const user = userEvent.setup();
  await screen.findByText('tool_1');
  await user.type(screen.getByLabelText('工具筛选'), 'filtered');
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  await screen.findByText('filtered');
  await user.click(screen.getByRole('button', { name: '清除筛选' }));
  await screen.findByText('tool_1');
  expect(screen.getByLabelText('工具筛选')).toHaveValue('');
  expect(screen.getByText('第 1 页')).toBeVisible();
});

it('does not apply a late next-page response after changing filters', async () => {
  network();
  const original = globalThis.fetch;
  let resolveNext: ((value: Response) => void) | undefined;
  let nextCalls = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string, init?: RequestInit) => {
      const url = new URL(raw, 'http://localhost');
      if (
        url.searchParams.get('cursor') === '10' &&
        !url.searchParams.get('tool')
      ) {
        nextCalls++;
        return new Promise<Response>((resolve) => {
          resolveNext = resolve;
        });
      }
      return original(raw, init);
    }),
  );
  mount();
  const user = userEvent.setup();
  await screen.findByText('tool_1');
  await user.click(screen.getByRole('button', { name: '下一页' }));
  expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
  expect(nextCalls).toBe(1);
  expect(screen.getByText('tool_1')).toBeVisible();
  await user.type(screen.getByLabelText('工具筛选'), 'filtered');
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  await screen.findByText('filtered');
  resolveNext?.(Response.json({ items: [record(11)], next_cursor: '' }));
  await waitFor(() => expect(screen.getByText('第 1 页')).toBeVisible());
  expect(screen.getByText('filtered')).toBeVisible();
  expect(screen.queryByText('tool_11')).not.toBeInTheDocument();
});

it('omits inactive pagination controls for an empty filtered result', async () => {
  network();
  mount();
  const user = userEvent.setup();
  await screen.findByText('tool_1');
  await user.type(screen.getByLabelText('工具筛选'), 'empty');
  await user.click(screen.getByRole('button', { name: '应用筛选' }));
  await screen.findByRole('heading', { name: '没有符合筛选条件的记录' });
  expect(
    screen.queryByRole('navigation', { name: '调用明细分页' }),
  ).not.toBeInTheDocument();
});
