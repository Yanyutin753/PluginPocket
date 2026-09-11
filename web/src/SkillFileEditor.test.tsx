import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { marketplaceItemSchema } from './features/operations/api';
import { MarketplaceEditor } from './features/operations/MarketplaceEditor';
import { editCode } from './test/edit-code';

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
    source: 'github',
    repo: 'owner/repo',
    path: 'skills/review',
    files: { 'SKILL.md': '# Review', 'reference.md': 'Reference' },
    file_manifest: {
      'SKILL.md': { sha256: 'c'.repeat(64), size: 8, executable: true },
      'scripts/check.sh': { sha256: 'a'.repeat(64), size: 7, executable: true },
      'image.bin': { sha256: 'b'.repeat(64), size: 3, executable: false },
    },
  },
});

function mount(failFirst = false) {
  let attempts = 0;
  const fetch = vi.fn(async (_url: string, init?: RequestInit) => {
    if (init?.method === 'POST') return Response.json({ item });
    if (failFirst && attempts++ === 0)
      return Response.json({ error: 'upstream_unavailable' }, { status: 503 });
    return Response.json({
      files: {
        'scripts/check.sh': {
          encoding: 'base64',
          content: 'ZWNobyBoaQ==',
          sha256: 'a'.repeat(64),
          size: 7,
          executable: true,
        },
        'image.bin': {
          encoding: 'base64',
          content: 'AP+A',
          sha256: 'b'.repeat(64),
          size: 3,
          executable: false,
        },
      },
    });
  });
  vi.stubGlobal('fetch', fetch);
  const close = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <MarketplaceEditor
        kind="skill"
        item={item}
        items={[]}
        saved={vi.fn()}
        close={close}
      />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), fetch, close };
}

it('toggles fullscreen without losing the selected draft and Escape restores before closing', async () => {
  const { user, close } = mount();
  await user.click(screen.getByRole('button', { name: 'reference.md' }));
  await editCode(user, 'reference.md 内容', 'Reference draft');
  const toggle = screen.getByRole('button', { name: '全屏' });
  toggle.focus();
  await user.keyboard('{Enter}');
  expect(screen.getByRole('dialog')).toHaveAttribute('data-fullscreen', 'true');
  expect(screen.getByRole('button', { name: '退出全屏' })).toHaveFocus();
  await user.keyboard('{Escape}');
  expect(screen.getByRole('dialog')).toHaveAttribute(
    'data-fullscreen',
    'false',
  );
  expect(screen.getByLabelText('reference.md 内容')).toHaveTextContent(
    'Reference draft',
  );
  expect(close).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: '全屏' }));
  await user.click(screen.getByRole('button', { name: '退出全屏' }));
  expect(screen.getByRole('dialog')).toHaveAttribute(
    'data-fullscreen',
    'false',
  );
  await user.keyboard('{Escape}');
  expect(close).toHaveBeenCalledOnce();
});

it('keeps an empty SKILL.md from being saved after switching to another file', async () => {
  const { user } = mount();
  await editCode(user, 'SKILL.md 内容', '');
  await user.click(screen.getByRole('button', { name: 'reference.md' }));
  expect(screen.getByRole('button', { name: '保存技能' })).toBeDisabled();
});

it('retains uploaded bytes and removals when switching to GitHub sync and back', async () => {
  const { user, fetch } = mount();
  await user.click(screen.getByText('文件管理：上传与移除'));
  await user.upload(
    screen.getByLabelText('添加附件'),
    new File(['hello'], 'hello.txt'),
  );
  await screen.findByText(/hello.txt ·/);
  await user.click(screen.getByRole('button', { name: '移除 image.bin' }));
  await user.click(screen.getByText('GitHub 导入与同步'));
  await user.click(screen.getByRole('combobox', { name: '技能来源' }));
  await user.click(screen.getByRole('option', { name: '从 GitHub 重新同步' }));
  await user.click(screen.getByRole('combobox', { name: '技能来源' }));
  await user.click(screen.getByRole('option', { name: '编辑技能文件' }));
  await user.click(screen.getByRole('button', { name: '保存技能' }));
  await waitFor(() =>
    expect(fetch.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(
      true,
    ),
  );
  const body = JSON.parse(
    String(
      fetch.mock.calls.find(([, init]) => init?.method === 'POST')?.[1]?.body,
    ),
  );
  expect(body.files_v2['hello.txt'].content).toBe('aGVsbG8=');
  expect(body.delete_files).toEqual(['image.bin']);
});

it('edits a published GitHub snapshot, retains drafts across file switches and preserves executable bits', async () => {
  const { user, fetch } = mount();
  expect(screen.getByLabelText('SKILL.md 内容')).toHaveTextContent('# Review');
  await user.click(screen.getByRole('button', { name: 'reference.md' }));
  await editCode(user, 'reference.md 内容', 'Reference revised');
  await user.click(screen.getByRole('button', { name: 'scripts/check.sh' }));
  await screen.findByLabelText('scripts/check.sh 内容');
  await editCode(user, 'scripts/check.sh 内容', 'echo hi!');
  await user.click(screen.getByRole('button', { name: 'reference.md' }));
  expect(screen.getByLabelText('reference.md 内容')).toHaveTextContent(
    'Reference revised',
  );
  await user.click(screen.getByRole('button', { name: '保存技能' }));
  await waitFor(() =>
    expect(fetch.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(
      true,
    ),
  );
  const body = JSON.parse(
    String(
      fetch.mock.calls.find(([, init]) => init?.method === 'POST')?.[1]?.body,
    ),
  );
  expect(body.source).toBe('inline');
  expect(body.files_v2['SKILL.md']).toMatchObject({
    content: btoa('# Review'),
    executable: true,
  });
  expect(body.files_v2['reference.md'].content).toBe(btoa('Reference revised'));
  expect(body.files_v2['scripts/check.sh']).toMatchObject({
    content: btoa('echo hi!'),
    executable: true,
  });
  expect(body.delete_files).toEqual([]);
});

it('retries failed file loading and keeps binary files out of the text editor', async () => {
  const { user } = mount(true);
  await user.click(screen.getByRole('button', { name: 'image.bin' }));
  expect(await screen.findByRole('alert')).toBeVisible();
  expect(screen.queryByLabelText('image.bin 内容')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '重试' }));
  expect(
    await screen.findByText('此文件不支持文本编辑，可在文件管理中上传替换'),
  ).toBeVisible();
  expect(screen.queryByLabelText('image.bin 内容')).not.toBeInTheDocument();
});

it('creates a text file using the keyboard and rejects paths outside the skill', async () => {
  const { user } = mount();
  await user.click(screen.getByRole('button', { name: '新建文本文件' }));
  await user.type(screen.getByLabelText('新文件路径'), '../bad.md');
  await user.click(screen.getByRole('button', { name: '创建文件' }));
  expect(await screen.findByRole('alert')).toBeVisible();
  await user.clear(screen.getByLabelText('新文件路径'));
  await user.type(screen.getByLabelText('新文件路径'), 'notes/todo.md');
  await user.tab();
  await user.keyboard('{Enter}');
  await waitFor(() =>
    expect(screen.getByLabelText('notes/todo.md 内容')).toHaveFocus(),
  );
  await user.paste('Todo');
  await user.click(screen.getByRole('button', { name: 'SKILL.md' }));
  await user.click(screen.getByRole('button', { name: 'notes/todo.md' }));
  expect(screen.getByLabelText('notes/todo.md 内容')).toHaveTextContent('Todo');
});

it('rejects case aliases and file-directory collisions before creating a draft', async () => {
  const { user } = mount();
  await user.click(screen.getByRole('button', { name: '新建文本文件' }));
  for (const path of ['Reference.md', 'scripts', 'reference.md/nested.txt']) {
    await user.clear(screen.getByLabelText('新文件路径'));
    await user.type(screen.getByLabelText('新文件路径'), path);
    await user.click(screen.getByRole('button', { name: '创建文件' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(
      screen.queryByRole('button', { name: path }),
    ).not.toBeInTheDocument();
  }
});
