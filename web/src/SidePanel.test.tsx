import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { SidePanel } from './components/SidePanel';

it('provides fullscreen in any panel and restores before applying the close lock', async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  const { rerender } = render(
    <SidePanel title="详情" onClose={close} locked>
      <p>内容</p>
    </SidePanel>,
  );
  await user.click(screen.getByRole('button', { name: '全屏' }));
  expect(screen.getByRole('dialog')).toHaveAttribute('data-fullscreen', 'true');
  await user.keyboard('{Escape}');
  expect(screen.getByRole('dialog')).toHaveAttribute(
    'data-fullscreen',
    'false',
  );
  await user.keyboard('{Escape}');
  expect(close).not.toHaveBeenCalled();
  rerender(
    <SidePanel title="详情" onClose={close}>
      <p>内容</p>
    </SidePanel>,
  );
  await user.click(screen.getByRole('button', { name: '全屏' }));
  await user.click(screen.getByRole('button', { name: '退出全屏' }));
  await user.keyboard('{Escape}');
  expect(close).toHaveBeenCalledOnce();
});
