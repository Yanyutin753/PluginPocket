import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { ToolIcon, ToolIconEditor } from './features/operations/ToolIcon';

const url = 'https://example.com/icon.png';
const svg =
  '<svg xmlns="http://www.w3.org/2000/svg"><title>工具</title><path d="M0 0h10v10z"/></svg>';
function Editor({ initial = '', disabled = false }) {
  const [value, onChange] = useState(initial);
  return (
    <ToolIconEditor value={value} onChange={onChange} disabled={disabled} />
  );
}
describe('custom tool icons', () => {
  it('offers upload first in the keyboard flow', async () => {
    const user = userEvent.setup();
    render(<Editor />);
    await user.tab();
    expect(screen.getByLabelText('上传图标')).toHaveFocus();
  });
  it('shows the uploaded filename without putting encoded image bytes in the address field', async () => {
    const user = userEvent.setup();
    render(<Editor />);
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File([svg], 'custom-icon.svg', { type: 'image/svg+xml' }),
    );
    expect(
      await screen.findByRole('img', { name: '图标预览' }),
    ).toHaveAttribute(
      'src',
      expect.stringMatching(/^data:image\/svg\+xml;base64,/),
    );
    expect(screen.getByLabelText('图标地址或 SVG')).toHaveValue('');
    expect(screen.getByText('custom-icon.svg')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '移除图标' }));
    expect(
      screen.queryByRole('img', { name: '图标预览' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('custom-icon.svg')).not.toBeInTheDocument();
    expect(screen.getByLabelText('图标地址或 SVG')).toBeValid();
  });
  it('reopens a stored uploaded icon without exposing the encoded value', () => {
    const stored = 'data:image/svg+xml;base64,PHN2Zy8+';
    render(<Editor initial={stored} />);
    expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
      'src',
      stored,
    );
    expect(screen.getByLabelText('图标地址或 SVG')).toHaveValue('');
    expect(screen.getByText('已上传图片')).toBeVisible();
  });
  it('blocks saving until the selected image has finished reading', async () => {
    render(<Editor initial={url} />);
    fireEvent.change(screen.getByLabelText('上传图标'), {
      target: {
        files: [new File([svg], 'icon.svg', { type: 'image/svg+xml' })],
      },
    });
    expect(screen.getByLabelText('图标地址或 SVG')).toBeInvalid();
    expect(screen.getByRole('status')).toHaveTextContent('正在读取图标');
    await waitFor(() =>
      expect(screen.getByLabelText('图标地址或 SVG')).toBeValid(),
    );
    expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
      'src',
      expect.stringMatching(/^data:image\/svg\+xml;base64,/),
    );
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
  it('clears an upload error with Remove even when no icon was set', async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(<Editor />);
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File(['bad'], 'icon.html', { type: 'text/html' }),
    );
    expect(await screen.findByRole('alert')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '移除图标' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByLabelText('图标地址或 SVG')).toBeValid();
  });
  it('previews HTTPS URLs and supports keyboard removal', async () => {
    const user = userEvent.setup();
    render(<Editor />);
    await user.type(screen.getByLabelText('图标地址或 SVG'), url);
    expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
      'src',
      url,
    );
    screen.getByRole('button', { name: '移除图标' }).focus();
    await user.keyboard('{Enter}');
    expect(screen.getByLabelText('图标地址或 SVG')).toHaveValue('');
    expect(
      screen.queryByRole('img', { name: '图标预览' }),
    ).not.toBeInTheDocument();
  });
  it('encodes pasted UTF-8 SVG as an image data URL', async () => {
    const user = userEvent.setup();
    render(<Editor />);
    screen.getByLabelText('图标地址或 SVG').focus();
    await user.paste(svg);
    expect(screen.getByLabelText('图标地址或 SVG')).toHaveValue(svg);
    const src =
      screen.getByRole('img', { name: '图标预览' }).getAttribute('src') ?? '';
    expect(src).toMatch(/^data:image\/svg\+xml;base64,/);
    expect(
      new TextDecoder().decode(
        Uint8Array.from(atob(src.split(',')[1]), (c) => c.charCodeAt(0)),
      ),
    ).toBe(svg);
  });
  it.each([
    'http://example.com/a.png',
    'https://user:pass@example.com/a.png',
    '<svg><script>alert(1)</script></svg>',
    '<svg onload="alert(1)"/>',
    '<svg><foreignObject/></svg>',
    '<svg><image href="https://example.com/a.png"/></svg>',
    '<svg>',
    `https://example.com/${'a'.repeat(2048)}`,
  ])(
    'rejects unsafe input while retaining the previous preview: %s',
    async (input) => {
      const user = userEvent.setup();
      render(<Editor initial={url} />);
      const field = screen.getByLabelText('图标地址或 SVG');
      await user.click(field);
      await user.keyboard('{Control>}a{/Control}');
      await user.paste(input);
      expect(screen.getByRole('alert')).toBeVisible();
      expect(field).toBeInvalid();
      expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
        'src',
        url,
      );
    },
  );
  it('uploads SVG through a native file input', async () => {
    const user = userEvent.setup();
    render(<Editor />);
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File([svg], 'icon.svg', { type: 'image/svg+xml' }),
    );
    expect(
      await screen.findByRole('img', { name: '图标预览' }),
    ).toHaveAttribute(
      'src',
      expect.stringMatching(/^data:image\/svg\+xml;base64,/),
    );
  });
  it.each([
    new File(['bad'], 'icon.html', { type: 'text/html' }),
    new File(['x'.repeat(65537)], 'icon.png', { type: 'image/png' }),
    new File(['<svg onload="bad()"/>'], 'icon.svg', { type: 'image/svg+xml' }),
  ])(
    'rejects invalid or oversized uploads and allows recovery',
    async (file) => {
      const user = userEvent.setup({ applyAccept: false });
      render(<Editor initial={url} />);
      await user.upload(screen.getByLabelText('上传图标'), file);
      expect(await screen.findByRole('alert')).toBeVisible();
      expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
        'src',
        url,
      );
      await user.upload(
        screen.getByLabelText('上传图标'),
        new File([svg], 'safe.svg', { type: 'image/svg+xml' }),
      );
      await waitFor(() =>
        expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
      );
    },
  );
  it('falls back when an image fails and retries a changed icon', () => {
    const { rerender } = render(<ToolIcon icon={url} name="Search" />);
    fireEvent.error(screen.getByRole('img', { name: 'Search' }));
    expect(screen.getByRole('img', { name: 'Search' })).toHaveAttribute(
      'src',
      '/images/workshop-tool-icon.png',
    );
    rerender(<ToolIcon icon="https://example.com/new.png" name="Search" />);
    expect(screen.getByRole('img', { name: 'Search' })).toHaveAttribute(
      'src',
      'https://example.com/new.png',
    );
  });
  it('disables editing while saving', () => {
    render(<Editor initial={url} disabled />);
    expect(screen.getByLabelText('图标地址或 SVG')).toBeDisabled();
    expect(screen.getByLabelText('上传图标')).toBeDisabled();
    expect(screen.getByRole('button', { name: '移除图标' })).toBeDisabled();
  });
  it('shows the workshop illustration when no custom icon is configured', () => {
    render(<ToolIcon name="Echo" />);
    expect(screen.getByRole('img', { name: 'Echo' })).toHaveAttribute(
      'src',
      '/images/workshop-tool-icon.png',
    );
  });
});
