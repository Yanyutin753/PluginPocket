import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeAll, expect, it, vi } from 'vitest';
import { Select } from './select';

beforeAll(() => {
  // jsdom lacks geometry APIs used by Radix's managed focus and popper.
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
    configurable: true,
    value: vi.fn(),
  });
});
const options = [
  { value: '', label: '个人钱包' },
  { value: '7', label: '设计团队' },
  { value: '8', label: '已停用团队', disabled: true },
  { value: '42', label: '开发团队' },
];

it('uses managed keyboard navigation, skips disabled options, and Escape keeps the selected value', async () => {
  const user = userEvent.setup();
  render(<Select aria-label="钱包" defaultValue="7" options={options} />);
  await user.tab();
  await user.keyboard('{Enter}');
  await screen.findByRole('listbox');
  await waitFor(() =>
    expect(screen.getByRole('option', { name: '设计团队' })).toHaveFocus(),
  );
  await user.keyboard('{ArrowDown}{Enter}');
  expect(screen.getByRole('combobox', { name: '钱包' })).toHaveTextContent(
    '开发团队',
  );
  await user.keyboard('{Enter}');
  await screen.findByRole('listbox');
  await user.keyboard('{Home}{Escape}');
  expect(screen.getByRole('combobox', { name: '钱包' })).toHaveTextContent(
    '开发团队',
  );
  expect(screen.getByRole('combobox', { name: '钱包' })).toHaveFocus();
});

it('selects an empty option, serializes actual numeric and empty values, and resets an uncontrolled form', async () => {
  const user = userEvent.setup();
  const { container } = render(
    <form>
      <Select
        aria-label="钱包"
        name="wallet"
        defaultValue="42"
        options={options}
      />
      <button type="reset">重置</button>
    </form>,
  );
  const form = container.querySelector('form');
  if (!form) throw new Error('Missing form');
  expect(new FormData(form).get('wallet')).toBe('42');
  await user.tab();
  await user.keyboard('{Enter}');
  await screen.findByRole('listbox');
  await user.keyboard('{Home}');
  await waitFor(() =>
    expect(screen.getByRole('option', { name: '个人钱包' })).toHaveFocus(),
  );
  await user.keyboard('{Enter}');
  expect(screen.getByRole('combobox', { name: '钱包' })).toHaveTextContent(
    '个人钱包',
  );
  expect(new FormData(form).getAll('wallet')).toEqual(['']);
  await user.click(screen.getByRole('button', { name: '重置' }));
  expect(screen.getByRole('combobox', { name: '钱包' })).toHaveTextContent(
    '开发团队',
  );
  expect(new FormData(form).get('wallet')).toBe('42');
});

it('shows a placeholder, supports controlled selection, and portals the list outside its container', async () => {
  const user = userEvent.setup();
  const change = vi.fn();
  const { container, rerender } = render(
    <Select
      aria-label="团队"
      value=""
      placeholder="选择团队"
      onValueChange={change}
      options={options.slice(1)}
    />,
  );
  expect(screen.getByRole('combobox', { name: '团队' })).toHaveTextContent(
    '选择团队',
  );
  await user.tab();
  await user.keyboard('{Enter}');
  const list = await screen.findByRole('listbox');
  expect(container).not.toContainElement(list);
  expect(screen.getByRole('option', { name: '已停用团队' })).toHaveAttribute(
    'aria-disabled',
    'true',
  );
  await user.keyboard('{End}');
  await waitFor(() =>
    expect(screen.getByRole('option', { name: '开发团队' })).toHaveFocus(),
  );
  await user.keyboard('{Enter}');
  expect(change).toHaveBeenCalledWith('42');
  rerender(
    <Select
      aria-label="团队"
      value="42"
      onValueChange={change}
      options={options.slice(1)}
    />,
  );
  expect(screen.getByRole('combobox', { name: '团队' })).toHaveTextContent(
    '开发团队',
  );
});

it('disables the trigger and omits disabled values from form submission', async () => {
  const { container } = render(
    <form>
      <Select
        aria-label="钱包"
        name="wallet"
        defaultValue="42"
        disabled
        options={options}
      />
    </form>,
  );
  expect(screen.getByRole('combobox', { name: '钱包' })).toBeDisabled();
  const form = container.querySelector('form');
  if (!form) throw new Error('Missing form');
  expect(new FormData(form).has('wallet')).toBe(false);
});
