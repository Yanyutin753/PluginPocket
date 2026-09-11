import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { marketplaceItemSchema } from './features/operations/api';
import { MarketplaceEditor } from './features/operations/MarketplaceEditor';
import { SkillFilePreview } from './features/operations/SkillFilePreview';
import { editCode } from './test/edit-code';

const png =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jf1sAAAAASUVORK5CYII=';

it('shows an accessible embedded image error and recovers after replacement', () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {});
  const props = {
    path: 'SKILL.md',
    text: '![Diagram](pixel.png)',
    paths: ['pixel.png'],
    openFile: vi.fn(),
  };
  const { rerender } = render(
    <SkillFilePreview
      {...props}
      files={{ 'pixel.png': { content: 'broken', size: 6 } }}
    />,
  );
  fireEvent.error(screen.getByRole('img', { name: 'Diagram' }));
  expect(screen.getByRole('status')).toHaveTextContent('图片无法预览');
  expect(error).not.toHaveBeenCalled();
  rerender(
    <SkillFilePreview
      {...props}
      files={{ 'pixel.png': { content: png, size: 68 } }}
    />,
  );
  expect(screen.getByRole('img', { name: 'Diagram' })).toHaveAttribute(
    'src',
    `data:image/png;base64,${png}`,
  );
  expect(screen.queryByRole('status')).not.toBeInTheDocument();
  error.mockRestore();
});
const item = marketplaceItemSchema.parse({
  id: 1,
  slug: 'review',
  name: 'Review',
  description: '',
  kind: 'skill',
  source: 'curated',
  repo_url: '',
  homepage: '',
  transport: 'unknown',
  endpoint: '',
  package: '',
  stars: 0,
  synced_at: null,
  installed: false,
  spec: {
    source: 'inline',
    files: {
      'SKILL.md':
        '# Hello\n\n**Important**\n\n<script>bad()</script>\n\n[unsafe](javascript:alert%281%29)\n\n![Diagram](assets/pixel.png)\n\n[Guide](references/guide.md)',
      'references/guide.md': '# Guide',
    },
    file_manifest: {
      'assets/pixel.png': {
        sha256: 'a'.repeat(64),
        size: 68,
        executable: false,
      },
    },
  },
});

function mount(initialContent?: string) {
  const fetch = vi.fn(async (_url: string, init?: RequestInit) =>
    init?.method === 'POST'
      ? Response.json({ item })
      : Response.json({
          files: {
            'SKILL.md': {
              encoding: 'base64',
              content: btoa('# Published'),
              size: 11,
              executable: false,
            },
            'assets/pixel.png': {
              encoding: 'base64',
              content: png,
              size: 68,
              executable: false,
            },
          },
        }),
  );
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MarketplaceEditor
        kind="skill"
        item={
          initialContent === undefined
            ? item
            : {
                ...item,
                spec: {
                  ...item.spec,
                  files: { ...item.spec?.files, 'SKILL.md': initialContent },
                },
              }
        }
        items={[]}
        saved={vi.fn()}
        close={vi.fn()}
      />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

it('renders split-view drafts and downloads the current text after loading published attachments', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: '分栏' }));
  await screen.findByRole('img', { name: 'Diagram' });
  await editCode(user, 'SKILL.md 内容', '# Updated');
  expect(await screen.findByRole('heading', { name: 'Updated' })).toBeVisible();
  expect(screen.getByRole('link', { name: '下载文件' })).toHaveAttribute(
    'href',
    `data:application/octet-stream;base64,${btoa('# Updated')}`,
  );
});

it('does not preview a removed attachment', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: '预览' }));
  await screen.findByRole('img', { name: 'Diagram' });
  await user.click(screen.getByText('文件管理：上传与移除'));
  await user.click(
    screen.getByRole('button', { name: '移除 assets/pixel.png' }),
  );
  expect(
    screen.queryByRole('img', { name: 'Diagram' }),
  ).not.toBeInTheDocument();
});

it('separates skill metadata from the rendered Markdown body', async () => {
  const user = mount(
    '---\nname: review\ndescription: Review code\n---\n\n# Instructions\n\nDo the work.',
  );
  await user.click(screen.getByRole('button', { name: '预览' }));
  expect(
    await screen.findByRole('heading', { name: 'Instructions' }),
  ).toBeVisible();
  await user.click(screen.getByText('技能元数据'));
  expect(
    within(screen.getByRole('region', { name: '文件预览' })).getByText(
      /name: review/,
    ),
  ).toBeVisible();
});

it('keeps undo history per file across an image preview', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: 'references/guide.md' }));
  await editCode(user, 'references/guide.md 内容', '# Draft');
  await user.click(screen.getByRole('button', { name: 'assets/pixel.png' }));
  await screen.findByRole('img', { name: 'assets/pixel.png' });
  await user.click(screen.getByRole('button', { name: 'references/guide.md' }));
  await user.click(
    await screen.findByRole('textbox', { name: 'references/guide.md 内容' }),
  );
  await user.keyboard('{Control>}z{/Control}');
  expect(
    screen.getByRole('textbox', { name: 'references/guide.md 内容' }),
  ).toHaveTextContent('# Guide');
});

it('lets Escape dismiss editor search before fullscreen or the surrounding panel', async () => {
  const user = mount();
  await editCode(user, 'SKILL.md 内容', '# Keep this draft');
  await user.click(screen.getByRole('button', { name: '工作区全屏' }));
  await user.click(screen.getByRole('textbox', { name: 'SKILL.md 内容' }));
  await user.keyboard('{Control>}f{/Control}');
  expect(await screen.findByPlaceholderText('Find')).toBeVisible();
  await user.keyboard('{Escape}');
  expect(screen.queryByPlaceholderText('Find')).not.toBeInTheDocument();
  expect(screen.getByRole('dialog')).toHaveAttribute('data-fullscreen', 'true');
  await user.keyboard('{Escape}');
  expect(screen.getByRole('dialog')).toHaveAttribute(
    'data-fullscreen',
    'false',
  );
  await user.keyboard('{Control>}f{/Control}');
  await screen.findByPlaceholderText('Find');
  await user.keyboard('{Escape}');
  expect(
    screen.getByRole('textbox', { name: 'SKILL.md 内容' }),
  ).toHaveTextContent('# Keep this draft');
});

it('previews safe Markdown and package images, then opens a relative document link', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: '预览' }));
  const preview = await screen.findByRole('region', { name: '文件预览' });
  expect(within(preview).getByRole('heading', { name: 'Hello' })).toBeVisible();
  expect(within(preview).getByText('Important').tagName).toBe('STRONG');
  expect(preview.querySelector('script')).toBeNull();
  expect(
    within(preview).queryByRole('link', { name: 'unsafe' }),
  ).not.toBeInTheDocument();
  await waitFor(() =>
    expect(
      within(preview).getByRole('img', { name: 'Diagram' }),
    ).toHaveAttribute('src', `data:image/png;base64,${png}`),
  );
  await user.click(within(preview).getByRole('button', { name: 'Guide' }));
  expect(await screen.findByRole('heading', { name: 'Guide' })).toBeVisible();
});

it('keeps edits when closing and reopening a file tab and offers a workspace fullscreen control', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: 'references/guide.md' }));
  const editor = screen.getByRole('textbox', {
    name: 'references/guide.md 内容',
  });
  await user.click(editor);
  await user.keyboard('{Control>}a{/Control}');
  await user.paste('# Draft');
  await user.click(
    screen.getByRole('button', { name: '关闭标签 references/guide.md' }),
  );
  await user.click(screen.getByRole('button', { name: 'references/guide.md' }));
  expect(
    screen.getByRole('textbox', { name: 'references/guide.md 内容' }),
  ).toHaveTextContent('# Draft');
  await user.click(screen.getByRole('button', { name: '工作区全屏' }));
  expect(screen.getByRole('dialog')).toHaveAttribute('data-fullscreen', 'true');
  await user.click(screen.getByRole('button', { name: '还原工作区' }));
  expect(screen.getByRole('dialog')).toHaveAttribute(
    'data-fullscreen',
    'false',
  );
});

it('opens an image as a preview, supports download, and folds its folder', async () => {
  const user = mount();
  await user.click(screen.getByRole('button', { name: 'assets/pixel.png' }));
  expect(
    await screen.findByRole('img', { name: 'assets/pixel.png' }),
  ).toHaveAttribute('src', `data:image/png;base64,${png}`);
  expect(screen.getByRole('link', { name: '下载文件' })).toHaveAttribute(
    'download',
    'pixel.png',
  );
  await user.click(screen.getByRole('button', { name: '文件夹 assets' }));
  expect(
    screen.queryByRole('button', { name: 'assets/pixel.png' }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '文件夹 assets' }));
  expect(
    screen.getByRole('button', { name: 'assets/pixel.png' }),
  ).toBeVisible();
});
