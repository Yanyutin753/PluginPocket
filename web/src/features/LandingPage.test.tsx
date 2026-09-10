import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import { PreferencesProvider } from '../i18n';
import LandingPage from './LandingPage';

it('introduces Loadout publicly with honest setup steps and account navigation', () => {
  render(
    <PreferencesProvider>
      <LandingPage />
    </PreferencesProvider>,
  );
  expect(
    screen.getByRole('heading', { level: 1, name: '给你的 AI，装上超能力' }),
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
      `loadout login --server ${window.location.origin}\nloadout apply`,
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
    <PreferencesProvider>
      <LandingPage />
    </PreferencesProvider>,
  );
  await user.click(screen.getByRole('combobox', { name: '语言' }));
  await user.click(screen.getByRole('option', { name: 'English' }));
  expect(
    screen.getByRole('heading', {
      level: 1,
      name: 'Give your AI superpowers.',
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
