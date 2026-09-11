import { screen } from '@testing-library/react';
import type userEvent from '@testing-library/user-event';

// Exercise CodeMirror's real clipboard/keyboard handlers. jsdom has no native
// contenteditable typing/selection engine; textarea-only user.type is unsuitable.
export async function editCode(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  value: string,
) {
  await user.click(await screen.findByRole('textbox', { name: label }));
  await user.keyboard('{Control>}a{/Control}');
  if (value) await user.paste(value);
  else await user.keyboard('{Backspace}');
}
