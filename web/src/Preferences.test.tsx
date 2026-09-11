import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import App from './App';
import { chooseOption } from './test/select';

let dark = false;
const listeners = new Set<() => void>();
beforeEach(() => {
  localStorage.clear();
  dark = false;
  listeners.clear();
  window.history.replaceState({}, '', '/login');
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({
      get matches() {
        return dark;
      },
      addEventListener: (_: string, listener: () => void) =>
        listeners.add(listener),
      removeEventListener: (_: string, listener: () => void) =>
        listeners.delete(listener),
    })),
  );
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        url.endsWith('/account/me') || url.endsWith('/auth/refresh')
          ? Response.json({ error: 'unauthorized' }, { status: 401 })
          : Response.json({ github: false, email: false, payments: false }),
      ),
    ),
  );
});
afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});
function mount() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <App />
    </QueryClientProvider>,
  );
}
it('switches the entire login form to English without losing entered values and restores the preference', async () => {
  const view = mount();
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText('用户名'), 'alice');
  await chooseOption(
    user,
    screen.getByRole('combobox', { name: '语言' }),
    'English',
  );
  expect(
    await screen.findByRole('heading', { name: 'Log in to PluginPocket' }),
  ).toBeVisible();
  expect(screen.getByLabelText('Username')).toHaveValue('alice');
  expect(document.documentElement.lang).toBe('en');
  expect(localStorage.getItem('pluginpocket.locale')).toBe('en');
  view.unmount();
  mount();
  expect(await screen.findByLabelText('Username')).toBeVisible();
});
it('persists explicit themes and responds to system changes only in system mode', async () => {
  mount();
  const user = userEvent.setup();
  const theme = await screen.findByRole('combobox', { name: '外观' });
  await chooseOption(user, theme, '深色');
  expect(document.documentElement.dataset.theme).toBe('dark');
  expect(localStorage.getItem('pluginpocket.theme')).toBe('dark');
  await chooseOption(user, theme, '浅色');
  expect(document.documentElement.dataset.theme).toBe('light');
  await chooseOption(user, theme, '跟随系统');
  dark = true;
  for (const listener of listeners) listener();
  await waitFor(() =>
    expect(document.documentElement.dataset.theme).toBe('dark'),
  );
  dark = false;
  for (const listener of listeners) listener();
  await waitFor(() =>
    expect(document.documentElement.dataset.theme).toBe('light'),
  );
});
it('ignores invalid stored values and remains usable when storage is blocked', async () => {
  localStorage.setItem('pluginpocket.locale', 'invalid');
  localStorage.setItem('pluginpocket.theme', 'invalid');
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('blocked');
  });
  mount();
  const user = userEvent.setup();
  await chooseOption(
    user,
    await screen.findByRole('combobox', { name: '语言' }),
    'English',
  );
  expect(await screen.findByLabelText('Password')).toBeVisible();
  await chooseOption(
    user,
    screen.getByRole('combobox', { name: 'Appearance' }),
    'Dark',
  );
  expect(document.documentElement.dataset.theme).toBe('dark');
});

it('opens appearance options when clicking its visible icon', async () => {
  mount();
  const user = userEvent.setup();
  const trigger = await screen.findByRole('combobox', { name: '外观' });
  const icon = trigger.closest('.preference-control')?.querySelector('svg');
  if (!icon) throw new Error('Missing appearance icon');
  await user.click(icon);
  expect(await screen.findByRole('listbox')).toBeVisible();
  await user.keyboard('{Escape}');
  expect(trigger).toHaveFocus();
});
