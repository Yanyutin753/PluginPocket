export const maxFileBytes = 8 * 1024 * 1024;
export const maxPackageBytes = 32 * 1024 * 1024;

export function imageType(path: string) {
  const extension = path.split('.').pop()?.toLowerCase();
  return (
    {
      png: 'image/png',
      jpg: 'image/jpeg',
      jpeg: 'image/jpeg',
      gif: 'image/gif',
      webp: 'image/webp',
      avif: 'image/avif',
      svg: 'image/svg+xml',
      ico: 'image/x-icon',
    } as Record<string, string>
  )[extension ?? ''];
}

export function safeFilePath(path: string) {
  return (
    path.length > 0 &&
    path.length <= 128 &&
    !path.includes('..') &&
    !/[\\\p{Cc}]/u.test(path) &&
    path
      .split('/')
      .every(
        (part) =>
          part !== '' &&
          part !== '.' &&
          !/[. ]$/.test(part) &&
          !/^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(part),
      ) &&
    !/[<>:"|?*]/.test(path)
  );
}

export function readFileBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error('read'));
    reader.onload = () => resolve(String(reader.result).split(',')[1] ?? '');
    reader.readAsDataURL(file);
  });
}

export type FileUpload = {
  encoding: 'base64';
  content: string;
  executable: boolean;
  size: number;
};
export type FileInfo = { size: number; executable: boolean };
