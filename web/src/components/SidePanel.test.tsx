import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import { SidePanel } from './SidePanel';
import { Button } from './ui/button';
import { Select } from './ui/select';

it('keeps focus inside the panel and lets a nested selector close before the panel', async () => {
  const user = userEvent.setup();
  render(
    <>
      <Button>Background</Button>
      <SidePanel title="Edit" trigger="Open">
        <label htmlFor="choice">Choice</label>
        <Select
          id="choice"
          options={[
            { value: 'a', label: 'Alpha' },
            { value: 'b', label: 'Beta' },
          ]}
        />
        <Button>Save</Button>
      </SidePanel>
    </>,
  );
  await user.click(screen.getByRole('button', { name: 'Open' }));
  const dialog = screen.getByRole('dialog');
  for (let i = 0; i < 5; i++) {
    await user.tab();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  }
  await user.click(screen.getByRole('combobox', { name: 'Choice' }));
  await user.keyboard('{Escape}');
  expect(dialog).toBeVisible();
  expect(screen.getByRole('combobox')).toHaveFocus();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

it('prevents dismissal while locked, then allows it after the operation finishes', async () => {
  const user = userEvent.setup();
  const panel = (locked: boolean) => (
    <SidePanel title="Save" trigger="Open" locked={locked}>
      <p role="alert">Retry stays here</p>
    </SidePanel>
  );
  const { rerender } = render(panel(true));
  await user.click(screen.getByRole('button', { name: 'Open' }));
  expect(screen.getByRole('button', { name: '关闭面板' })).toBeDisabled();
  await user.keyboard('{Escape}');
  expect(within(screen.getByRole('dialog')).getByRole('alert')).toBeVisible();
  rerender(panel(false));
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Open' })).toHaveFocus();
});
