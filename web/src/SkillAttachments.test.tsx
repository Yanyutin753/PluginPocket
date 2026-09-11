import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { marketplaceItemSchema } from './features/operations/api';
import { MarketplaceEditor } from './features/operations/MarketplaceEditor';
import { safeFilePath } from './lib/files';

const item = marketplaceItemSchema.parse({
  id: 1,
  slug: 'binary-skill',
  name: 'Binary skill',
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
    files: { 'SKILL.md': '# Binary' },
    file_manifest: {
      'assets/old.bin': { sha256: 'a'.repeat(64), size: 3, executable: false },
    },
  },
});

function mount() {
  const fetch = vi.fn().mockResolvedValue(Response.json({ item }));
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MarketplaceEditor
        kind="skill"
        item={item}
        items={[]}
        saved={vi.fn()}
        close={vi.fn()}
      />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), fetch };
}

it('uploads binary bytes with a path and executable bit, while explicitly removing an old attachment', async () => {
  const { user, fetch } = mount();
  await user.type(screen.getByLabelText('附件目录（可选）'), 'bin');
  await user.upload(
    screen.getByLabelText('添加附件'),
    new File([new Uint8Array([0, 255, 128, 65])], 'helper.bin', {
      type: 'application/octet-stream',
    }),
  );
  await screen.findByText(/bin\/helper.bin ·/);
  await user.click(
    screen.getByRole('checkbox', { name: '可执行 bin/helper.bin' }),
  );
  await user.click(screen.getByRole('button', { name: '移除 assets/old.bin' }));
  await user.click(screen.getByRole('button', { name: '保存技能' }));
  await waitFor(() => expect(fetch).toHaveBeenCalled());
  const payload = JSON.parse(fetch.mock.calls[0][1].body);
  expect(payload).toMatchObject({
    files: { 'SKILL.md': '# Binary' },
    files_v2: {
      'bin/helper.bin': {
        encoding: 'base64',
        content: 'AP+AQQ==',
        executable: true,
      },
    },
    delete_files: ['assets/old.bin'],
  });
});

it('rejects path traversal and oversized files, then allows a valid retry', async () => {
  const { user, fetch } = mount();
  await user.type(screen.getByLabelText('附件目录（可选）'), '../escape');
  await user.upload(
    screen.getByLabelText('添加附件'),
    new File(['hello'], 'file.bin'),
  );
  expect(await screen.findByRole('alert')).toHaveTextContent('附件路径');
  expect(screen.getByRole('button', { name: '保存技能' })).toBeDisabled();
  await user.clear(screen.getByLabelText('附件目录（可选）'));
  await user.upload(
    screen.getByLabelText('添加附件'),
    new File([new Uint8Array(8 * 1024 * 1024 + 1)], 'large.bin'),
  );
  expect(await screen.findByRole('alert')).toHaveTextContent('8 MiB');
  await user.upload(
    screen.getByLabelText('添加附件'),
    new File(['ok'], 'ok.bin'),
  );
  await screen.findByText(/ok.bin ·/);
  expect(screen.getByRole('button', { name: '保存技能' })).toBeEnabled();
  expect(fetch).not.toHaveBeenCalled();
});

it('rejects an oversized file selection before reading its contents', async () => {
  const { user } = mount();
  const read = vi.spyOn(FileReader.prototype, 'readAsDataURL');
  await user.upload(
    screen.getByLabelText('添加附件'),
    Array.from({ length: 33 }, (_, i) => new File(['x'], `file-${i}.bin`)),
  );
  expect(await screen.findByRole('alert')).toHaveTextContent('32 个文件');
  expect(read).not.toHaveBeenCalled();
});

it('rejects file paths that alias reserved Windows names', () => {
  for (const path of [
    'assets/CON.txt',
    'aux',
    'file.',
    'file ',
    'bin/a?.dat',
    'LPT1.exe',
  ]) {
    expect(safeFilePath(path), path).toBe(false);
  }
});
