import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
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
  it('allows removal of a malformed stored image without crashing', async () => {
    render(<Editor initial="data:invalid" />);
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: '移除图标' }));
    expect(screen.getByLabelText('图标地址或 SVG')).toBeValid();
  });
  afterEach(() => vi.unstubAllGlobals());
  it('keeps saving blocked during compression and ignores a result after removal', async () => {
    let finish!: (value: unknown) => void;
    vi.stubGlobal(
      'createImageBitmap',
      vi.fn(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      ),
    );
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      drawImage: vi.fn(),
    } as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(
      'data:image/webp;base64,eA==',
    );
    const close = vi.fn();
    render(<Editor initial={url} />);
    const user = userEvent.setup();
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File(['image'], 'photo.png', { type: 'image/png' }),
    );
    expect(screen.getByLabelText('图标地址或 SVG')).toBeInvalid();
    await user.click(screen.getByRole('button', { name: '移除图标' }));
    finish({ width: 100, height: 100, close });
    await waitFor(() => expect(close).toHaveBeenCalled());
    expect(
      screen.queryByRole('img', { name: '图标预览' }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText('图标地址或 SVG')).toBeValid();
  });
  it('reduces dimensions again when the encoded output is still too large', async () => {
    vi.stubGlobal(
      'createImageBitmap',
      vi.fn().mockResolvedValue({ width: 800, height: 800, close: vi.fn() }),
    );
    const draw = vi.fn();
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      drawImage: draw,
    } as unknown as CanvasRenderingContext2D);
    const small = 'data:image/png;base64,eA==';
    vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL')
      .mockReturnValueOnce(`data:image/png;base64,${btoa('x'.repeat(70000))}`)
      .mockReturnValue(small);
    render(<Editor />);
    await userEvent
      .setup()
      .upload(
        screen.getByLabelText('上传图标'),
        new File(['image'], 'photo.png', { type: 'image/png' }),
      );
    await waitFor(() =>
      expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
        'src',
        small,
      ),
    );
    expect(draw).toHaveBeenLastCalledWith(expect.anything(), 0, 0, 128, 128);
  });
  it('compresses a large photo before previewing and saving it', async () => {
    const close = vi.fn();
    vi.stubGlobal(
      'createImageBitmap',
      vi.fn().mockResolvedValue({ width: 1200, height: 600, close }),
    );
    const draw = vi.fn();
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      drawImage: draw,
    } as unknown as CanvasRenderingContext2D);
    const small = `data:image/webp;base64,${btoa('x'.repeat(1024))}`;
    vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(small);
    render(<Editor initial={url} />);
    await userEvent
      .setup()
      .upload(
        screen.getByLabelText('上传图标'),
        new File(['x'.repeat(100000)], 'photo.png', { type: 'image/png' }),
      );
    await waitFor(() =>
      expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
        'src',
        small,
      ),
    );
    expect(draw).toHaveBeenCalledWith(expect.anything(), 0, 0, 256, 128);
    expect(close).toHaveBeenCalled();
    expect(screen.getByText('保存大小：1.0 KiB')).toBeVisible();
    expect(screen.getByLabelText('图标地址或 SVG')).toBeValid();
  });
  it('keeps the old icon when decoding fails and can recover with SVG', async () => {
    vi.stubGlobal(
      'createImageBitmap',
      vi.fn().mockRejectedValue(new Error('bad image')),
    );
    render(<Editor initial={url} />);
    const user = userEvent.setup();
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File(['broken'], 'bad.png', { type: 'image/png' }),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent('图片处理失败');
    expect(screen.getByRole('img', { name: '图标预览' })).toHaveAttribute(
      'src',
      url,
    );
    await user.upload(
      screen.getByLabelText('上传图标'),
      new File([svg], 'safe.svg', { type: 'image/svg+xml' }),
    );
    await waitFor(() =>
      expect(screen.getByLabelText('图标地址或 SVG')).toBeValid(),
    );
  });
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
    new File(['x'.repeat(5 * 1024 * 1024 + 1)], 'icon.png', {
      type: 'image/png',
    }),
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
