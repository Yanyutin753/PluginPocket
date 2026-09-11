import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { expect, it } from 'vitest';
import { PreferencesProvider } from '../i18n';
import LandingPage from './LandingPage';

it('introduces PluginPocket publicly with honest setup steps and account navigation', () => {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <PreferencesProvider>
        <MemoryRouter>
          <LandingPage />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
  expect(screen.getByRole('link', { name: 'PluginPocket 首页' })).toBeVisible();
  expect(
    screen.getByRole('heading', { level: 1, name: '把 AI 的超能力，装进口袋' }),
  ).toBeVisible();
  expect(
    screen
      .getAllByRole('link', { name: /创建账号/ })
      .every((link) => link.getAttribute('href') === '/register'),
  ).toBe(true);
  expect(
    screen
      .getAllByRole('link', { name: '登录' })
      .every((link) => link.getAttribute('href') === '/login'),
  ).toBe(true);
  expect(screen.getByRole('link', { name: '进入工作空间' })).toHaveAttribute(
    'href',
    '/overview',
  );
  expect(screen.getByRole('list', { name: '连接步骤' }).children).toHaveLength(
    3,
  );
  expect(
    screen.getByText(
      `pluginpocket login --server ${window.location.origin}\npluginpocket apply`,
      { normalizer: (text) => text },
    ),
  ).toBeVisible();
  expect(screen.getByRole('combobox', { name: '外观' })).toBeInTheDocument();
  expect(
    screen.queryByText(/今日调用|余额|月度消耗|已有.*用户/),
  ).not.toBeInTheDocument();
});

it('switches public copy to English while preserving account destinations', async () => {
  const user = userEvent.setup();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <PreferencesProvider>
        <MemoryRouter>
          <LandingPage />
        </MemoryRouter>
      </PreferencesProvider>
    </QueryClientProvider>,
  );
  await user.click(screen.getByRole('combobox', { name: '语言' }));
  await user.click(screen.getByRole('option', { name: 'English' }));
  expect(
    screen.getByRole('heading', {
      level: 1,
      name: 'Your AI superpowers, in your pocket.',
    }),
  ).toBeVisible();
  expect(
    screen.getByRole('link', { name: 'Create an account and get equipped' }),
  ).toHaveAttribute('href', '/register');
  expect(screen.getByRole('link', { name: 'Log in' })).toHaveAttribute(
    'href',
    '/login',
  );
});
