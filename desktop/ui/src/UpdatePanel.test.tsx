import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UpdatePanel } from './UpdatePanel';
import { type UpdateProgress, updater } from './updater';

vi.mock('./updater', () => ({
  updater: {
    currentVersion: vi.fn(),
    check: vi.fn(),
    downloadAndInstall: vi.fn(),
    relaunch: vi.fn(),
  },
}));

const available = {
  available: true as const,
  info: {
    currentVersion: '0.3.0',
    version: '0.4.0',
    notes: '修复连接诊断与市场同步',
  },
};
const uptodate = { available: false as const, currentVersion: '0.3.0' };
type CheckResult =
  | { available: true; info: typeof available.info }
  | { available: false; currentVersion: string }
  | { reject: Error };

// 首次 check 为启动静默检查；队列控制后续手动调用的返回。
let queue: CheckResult[] = [];
let progressHandler: ((progress: UpdateProgress) => void) | null = null;
let resolveInstall: (() => void) | null = null;

beforeEach(() => {
  vi.clearAllMocks();
  queue = [];
  progressHandler = null;
  resolveInstall = null;
  vi.mocked(updater.currentVersion).mockResolvedValue('0.3.0');
  vi.mocked(updater.check).mockImplementation(async () => {
    const next = queue.shift();
    if (!next) return uptodate;
    if ('reject' in next) throw next.reject;
    return next;
  });
  vi.mocked(updater.relaunch).mockResolvedValue(undefined);
  vi.mocked(updater.downloadAndInstall).mockImplementation(
    async (onProgress: (progress: UpdateProgress) => void) => {
      progressHandler = onProgress;
      return new Promise<void>((resolve) => {
        resolveInstall = resolve;
      });
    },
  );
});

describe('UpdatePanel', () => {
  it('shows the current version and offers a manual check', async () => {
    const user = userEvent.setup();
    queue = [uptodate, available];
    render(<UpdatePanel />);
    expect(
      await screen.findByText('当前版本 v0.3.0', { exact: false }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '检查更新' }));
    expect(await screen.findByText(/发现新版本 v0\.4\.0/)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: '下载并安装' }),
    ).toBeInTheDocument();
  });

  it('surfaces an available update found by the silent startup check', async () => {
    queue = [available];
    render(<UpdatePanel />);
    expect(await screen.findByText(/发现新版本 v0\.4\.0/)).toBeInTheDocument();
  });

  it('stays quiet when the startup check fails', async () => {
    queue = [{ reject: new Error('offline') }];
    render(<UpdatePanel />);
    await waitFor(() => expect(updater.check).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: '检查更新' }),
    ).toBeInTheDocument();
  });

  it('reports up-to-date after a manual check', async () => {
    const user = userEvent.setup();
    render(<UpdatePanel />);
    await user.click(screen.getByRole('button', { name: '检查更新' }));
    expect(await screen.findByText(/已是最新版本/)).toBeInTheDocument();
  });

  it('shows a recoverable error when a manual check fails', async () => {
    const user = userEvent.setup();
    queue = [uptodate, { reject: new Error('offline') }, available];
    render(<UpdatePanel />);
    await user.click(screen.getByRole('button', { name: '检查更新' }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/检查更新失败/);
    await user.click(screen.getByRole('button', { name: '重试' }));
    expect(await screen.findByText(/发现新版本/)).toBeInTheDocument();
  });

  it('downloads with progress, then offers relaunch and restarts the app', async () => {
    const user = userEvent.setup();
    queue = [available];
    render(<UpdatePanel />);
    await user.click(await screen.findByRole('button', { name: '下载并安装' }));
    expect(screen.getByText(/正在下载/)).toBeInTheDocument();
    expect(progressHandler).not.toBeNull();
    await act(async () => {
      progressHandler?.({ received: 512, total: 1024 });
    });
    expect(await screen.findByText(/50%/)).toBeInTheDocument();
    await act(async () => {
      progressHandler?.({ received: 1024, total: 1024 });
    });
    expect(await screen.findByText(/100%/)).toBeInTheDocument();
    await act(async () => {
      resolveInstall?.();
    });
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: '立即重启' }),
      ).toBeInTheDocument(),
    );
    await user.click(screen.getByRole('button', { name: '立即重启' }));
    await waitFor(() => expect(updater.relaunch).toHaveBeenCalled());
  });

  it('keeps the old version usable when the download fails', async () => {
    const user = userEvent.setup();
    queue = [available];
    vi.mocked(updater.downloadAndInstall).mockRejectedValueOnce(
      new Error('network reset'),
    );
    render(<UpdatePanel />);
    await user.click(await screen.findByRole('button', { name: '下载并安装' }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/更新未完成/);
    expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument();
  });

  it('is fully keyboard operable from the manual check to install', async () => {
    const user = userEvent.setup();
    queue = [uptodate, available];
    render(<UpdatePanel />);
    const check = await screen.findByRole('button', { name: '检查更新' });
    check.focus();
    await user.keyboard('{Enter}');
    const install = await screen.findByRole('button', { name: '下载并安装' });
    expect(install).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(screen.getByText(/正在下载/)).toBeInTheDocument();
  });
});
