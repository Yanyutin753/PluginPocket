import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { expect, it, vi } from 'vitest';
import UsagePage from './features/account/UsagePage';
import { PreferencesProvider } from './i18n';

function mount(exportResponse: () => Response | Promise<Response>) {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((raw: string) => {
      const url = new URL(raw, 'http://localhost');
      if (url.pathname.endsWith('/export')) return exportResponse();
      return Response.json({ items: [], next_cursor: '' });
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PreferencesProvider>
        <MemoryRouter initialEntries={['/admin/usage?tool=filtered']}>
          <UsagePage admin />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
}

const csv = () =>
  new Response('tool,cost\nfiltered,1', {
    headers: { 'Content-Type': 'text/csv', 'X-Next-Cursor': '50' },
  });

it('clears the previous filtered CSV download and cursor when filters are cleared', async () => {
  mount(csv);
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '导出 CSV' }));
  expect(await screen.findByRole('link', { name: '下载 CSV' })).toBeVisible();
  expect(screen.getByRole('button', { name: '导出下一页' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '清除筛选' }));
  expect(screen.getByLabelText('工具筛选')).toHaveValue('');
  expect(
    screen.queryByRole('link', { name: '下载 CSV' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: '导出下一页' }),
  ).not.toBeInTheDocument();
});

it('does not restore a stale CSV download when an export settles after filters are cleared', async () => {
  let complete: (response: Response) => void = () => {
    throw new Error('Export has not started');
  };
  const response = new Promise<Response>((resolve) => {
    complete = resolve;
  });
  mount(() => response);
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '导出 CSV' }));
  expect(screen.getByRole('button', { name: '正在导出…' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: '清除筛选' }));
  await act(async () => {
    complete(csv());
    await response;
  });
  await screen.findByRole('button', { name: '导出 CSV' });
  expect(
    screen.queryByRole('link', { name: '下载 CSV' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: '导出下一页' }),
  ).not.toBeInTheDocument();
});
