import { useEffect, useRef, useState } from 'react';
import { useI18n } from '@/i18n';
import {
  type FileInfo,
  type FileUpload,
  maxFileBytes,
  maxPackageBytes,
  readFileBase64,
  safeFilePath,
} from '@/lib/files';
import { Button } from './ui/button';
import { Field, FieldLabel } from './ui/field';
import { Input } from './ui/input';

export function FileAttachments({
  initial,
  documentBytes,
  disabled,
  onChange,
  onBlocked,
  files,
  deleted,
}: {
  initial: Record<string, FileInfo>;
  documentBytes: number;
  disabled: boolean;
  onChange: (files: Record<string, FileUpload>, deleted: string[]) => void;
  onBlocked: (blocked: boolean) => void;
  files: Record<string, FileUpload>;
  deleted: string[];
}) {
  const { t } = useI18n();
  const [directory, setDirectory] = useState('');
  const [reading, setReading] = useState(false);
  const [error, setError] = useState('');
  const revision = useRef(0);
  useEffect(
    () => () => {
      revision.current++;
    },
    [],
  );
  const entries = Object.entries({ ...initial, ...files }).filter(
    ([path]) => path !== 'SKILL.md' && !deleted.includes(path),
  );
  function publish(next: Record<string, FileUpload>, removed: string[]) {
    onChange(next, removed);
  }
  return (
    <Field>
      <FieldLabel htmlFor="attachment-directory">
        {t('附件目录（可选）')}
      </FieldLabel>
      <Input
        id="attachment-directory"
        placeholder="assets"
        value={directory}
        disabled={disabled || reading}
        onChange={(e) => setDirectory(e.target.value)}
      />
      <FieldLabel htmlFor="attachment-files">{t('添加附件')}</FieldLabel>
      <Input
        id="attachment-files"
        type="file"
        multiple
        disabled={disabled || reading}
        onChange={async (event) => {
          const selected = Array.from(event.target.files ?? []);
          event.target.value = '';
          if (!selected.length) return;
          const current = ++revision.current;
          setError('');
          setReading(true);
          onBlocked(true);
          try {
            const next = { ...files };
            let removed = [...deleted];
            for (const file of selected) {
              const path = directory ? `${directory}/${file.name}` : file.name;
              if (!safeFilePath(path) || path === 'SKILL.md')
                throw new Error(
                  '附件路径无效；使用相对目录，SKILL.md 请在上方编辑。',
                );
              if (file.size > maxFileBytes)
                throw new Error('单个附件最多 8 MiB。');
              next[path] = {
                encoding: 'base64',
                content: '',
                size: file.size,
                executable:
                  files[path]?.executable ?? initial[path]?.executable ?? false,
              };
              removed = removed.filter((name) => name !== path);
            }
            const all = Object.entries({ ...initial, ...next }).filter(
              ([path]) => path !== 'SKILL.md' && !removed.includes(path),
            );
            if (
              all.length + 1 > 32 ||
              all.reduce((sum, [, file]) => sum + file.size, documentBytes) >
                maxPackageBytes
            )
              throw new Error('技能最多 32 个文件，总大小最多 32 MiB。');
            for (const file of selected) {
              const path = directory ? `${directory}/${file.name}` : file.name;
              next[path].content = await readFileBase64(file);
            }
            if (revision.current !== current) return;
            publish(next, removed);
            onBlocked(false);
          } catch (cause) {
            if (revision.current !== current) return;
            setError(
              cause instanceof Error && cause.message !== 'read'
                ? cause.message
                : '附件读取失败，请重试。',
            );
          } finally {
            if (revision.current === current) setReading(false);
          }
        }}
      />
      <p className="hint-text">
        {t(
          '附件保留原始字节，不做图片压缩。每个文件最多 8 MiB，含 SKILL.md 共 32 个文件、32 MiB；较大附件需配置对象存储。',
        )}
      </p>
      {reading && <p role="status">{t('正在读取附件…')}</p>}
      {error && (
        <div role="alert">
          <p>{t(error)}</p>
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setError('');
              onBlocked(false);
            }}
          >
            {t('清除附件错误')}
          </Button>
        </div>
      )}
      <ul className="record-list">
        {entries.map(([path, info]) => (
          <li key={path} className="action-row">
            <span style={{ overflowWrap: 'anywhere', minWidth: 0 }}>
              {path} · {(info.size / 1024).toFixed(1)} KiB
              {!files[path] && info.executable ? ` · ${t('可执行')}` : ''}
            </span>
            {files[path] && (
              <label>
                <input
                  type="checkbox"
                  aria-label={t('可执行 {path}', { path })}
                  checked={files[path].executable}
                  disabled={disabled || reading}
                  onChange={(e) =>
                    publish(
                      {
                        ...files,
                        [path]: {
                          ...files[path],
                          executable: e.target.checked,
                        },
                      },
                      deleted,
                    )
                  }
                />
                {t('可执行')}
              </label>
            )}
            <Button
              type="button"
              variant="ghost"
              aria-label={t('移除 {path}', { path })}
              disabled={disabled || reading}
              onClick={() => {
                const next = { ...files };
                delete next[path];
                publish(next, [...new Set([...deleted, path])]);
              }}
            >
              {t('移除')}
            </Button>
          </li>
        ))}
      </ul>
    </Field>
  );
}
