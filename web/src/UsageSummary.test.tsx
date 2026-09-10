import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { UsageSummary } from './features/operations/UsageSummary';
import { PreferencesProvider } from './i18n';
import { chooseOption } from './test/select';

it.each([false, true])(
  'loads summary pages and retries without losing rows (team=%s)',
  async (team) => {
    const path = team ? '/account/teams/1/usage' : '/admin/usage';
    const item = (name: string, id: number) => ({
      ...(team ? { username: name, user_id: id } : { tool: name }),
      calls: 1,
      cost: 3,
      errors: 0,
    });
    let failed = false;
    vi.stubGlobal(
      'fetch',
      vi.fn((raw: string) => {
        const url = new URL(raw, 'http://localhost');
        if (url.searchParams.get('days') === '1') {
          return Response.json({
            items: [item('today-only', 3)],
            next_cursor: '',
          });
        }
        if (url.searchParams.get('cursor') === '2') {
          if (!failed) {
            failed = true;
            return Response.json({ error: 'internal_error' }, { status: 503 });
          }
          return Response.json({
            items: [item('last-group', 2)],
            next_cursor: '',
          });
        }
        return Response.json({
          items: [item('first-group', 1)],
          next_cursor: '2',
        });
      }),
    );
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    render(
      <QueryClientProvider client={client}>
        <PreferencesProvider>
          <UsageSummary path={path} team={team} />
        </PreferencesProvider>
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    expect(await screen.findByText('first-group')).toBeVisible();
    const more = await screen.findByRole('button', { name: '加载更多' });
    more.focus();
    await user.keyboard('{Enter}');
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByText('first-group')).toBeVisible();
    const retry = screen.getByRole('button', { name: '重试' });
    retry.focus();
    await user.keyboard('{Enter}');
    expect(await screen.findByText('last-group')).toBeVisible();
    expect(screen.getByText('first-group')).toBeVisible();
    expect(
      screen.queryByRole('button', { name: '加载更多' }),
    ).not.toBeInTheDocument();
    const table = screen.getByRole('region', { name: '调用汇总表格' });
    expect(within(table).getAllByRole('row')).toHaveLength(3);
    table.focus();
    expect(table).toHaveFocus();
    await chooseOption(
      user,
      screen.getByLabelText('汇总时间范围'),
      '今天（UTC）',
    );
    expect(await screen.findByText('today-only')).toBeVisible();
    expect(screen.queryByText('first-group')).not.toBeInTheDocument();
    expect(screen.queryByText('last-group')).not.toBeInTheDocument();
  },
);
