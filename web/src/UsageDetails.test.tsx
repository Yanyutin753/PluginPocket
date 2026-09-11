import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { expect, it, vi } from 'vitest';
import UsagePage from './features/account/UsagePage';
import { PreferencesProvider } from './i18n';

const item = {
  id: 7,
  tool: 'echo',
  cost: 1,
  status: 'ok',
  duration_ms: 15,
  created_at: '2026-09-11T08:00:00Z',
};
function mount(detail: () => Response) {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string) =>
      raw.split('?')[0].endsWith('/7')
        ? detail()
        : Response.json({ items: [item], next_cursor: '' }),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <PreferencesProvider>
        <MemoryRouter>
          <UsagePage />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
}

it('opens original input/output on demand and returns keyboard focus after Escape', async () => {
  let loaded = false;
  mount(() => {
    loaded = true;
    return Response.json({
      item: {
        ...item,
        input_data: '{"token":"original-secret","message":"你好"}',
        output_data: '{"content":[{"type":"text","text":"你好"}]}',
        input_truncated: false,
        output_truncated: false,
      },
    });
  });
  const user = userEvent.setup();
  const button = await screen.findByRole('button', { name: '查看详情' });
  expect(loaded).toBe(false);
  button.focus();
  await user.keyboard('{Enter}');
  const panel = await screen.findByRole('dialog', { name: '调用详情' });
  expect(await within(panel).findByText(/original-secret/)).toBeVisible();
  expect(
    within(panel).getByRole('heading', { name: '传入数据' }),
  ).toBeVisible();
  expect(
    within(panel).getByRole('heading', { name: '传出数据' }),
  ).toBeVisible();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(button).toHaveFocus();
});

it('retries detail errors and distinguishes missing data from truncated output', async () => {
  let failed = true;
  mount(() =>
    failed
      ? Response.json({ error: 'internal_error' }, { status: 500 })
      : Response.json({
          item: {
            ...item,
            input_data: null,
            output_data: 'partial output',
            input_truncated: false,
            output_truncated: true,
          },
        }),
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '查看详情' }));
  const panel = await screen.findByRole('dialog');
  expect(await within(panel).findByRole('alert')).toBeVisible();
  failed = false;
  await user.click(within(panel).getByRole('button', { name: '重试' }));
  expect(await within(panel).findByText('未记录')).toBeVisible();
  expect(within(panel).getByText('partial output')).toBeVisible();
  expect(within(panel).getByText(/内容较大，仅保留前 64 KiB/)).toBeVisible();
});

it('preserves large integers and literal markup in recorded payloads', async () => {
  mount(() =>
    Response.json({
      item: {
        ...item,
        input_data: '{"id":9007199254740993}',
        output_data: '<script>alert(1)</script>',
        input_truncated: false,
        output_truncated: false,
      },
    }),
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '查看详情' }));
  expect(await screen.findByText('{"id":9007199254740993}')).toBeVisible();
  expect(screen.getByText('<script>alert(1)</script>')).toBeVisible();
});
