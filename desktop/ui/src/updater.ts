import { getVersion } from '@tauri-apps/api/app';
import { relaunch } from '@tauri-apps/plugin-process';
import { check } from '@tauri-apps/plugin-updater';
import { z } from 'zod';
import pkg from '../package.json';

// latest.json 是不可信网络响应：进入 UI 前先经 zod 校验。
const updateInfo = z.object({
  currentVersion: z.string().min(1),
  version: z.string().min(1),
  notes: z.string().nullable(),
});
export type UpdateInfo = z.infer<typeof updateInfo>;

export type UpdateProgress = { received: number; total: number | null };

export const updater = {
  async currentVersion(): Promise<string> {
    try {
      return z
        .string()
        .min(1)
        .parse(await getVersion());
    } catch {
      return pkg.version;
    }
  },
  async check(): Promise<
    | { available: true; info: UpdateInfo }
    | { available: false; currentVersion: string }
  > {
    const update = await check();
    if (!update) {
      return {
        available: false,
        currentVersion: await updater.currentVersion(),
      };
    }
    return {
      available: true,
      info: updateInfo.parse({
        currentVersion: update.currentVersion,
        version: update.version,
        notes: update.body ?? null,
      }),
    };
  },
  async downloadAndInstall(
    onProgress: (progress: UpdateProgress) => void,
  ): Promise<void> {
    const update = await check();
    if (!update) throw new Error('update is no longer available');
    let total: number | null = null;
    let received = 0;
    await update.downloadAndInstall((event) => {
      switch (event.event) {
        case 'Started':
          total = event.data.contentLength ?? null;
          onProgress({ received: 0, total });
          break;
        case 'Progress':
          received += event.data.chunkLength;
          onProgress({ received, total });
          break;
        case 'Finished':
          break;
      }
    });
  },
  relaunch: () => relaunch(),
};
