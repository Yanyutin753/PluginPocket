import { screen } from '@testing-library/react';
import type userEvent from '@testing-library/user-event';
export async function chooseOption(
  user: ReturnType<typeof userEvent.setup>,
  control: HTMLElement,
  label: string,
) {
  control.focus();
  await user.keyboard('{Enter}');
  await user.click(await screen.findByRole('option', { name: label }));
}
