import type { EditorState } from '@codemirror/state';
import { useQuery } from '@tanstack/react-query';
import {
  Columns2,
  Eye,
  FilePlus2,
  Maximize2,
  Minimize2,
  PanelLeftClose,
  PanelLeftOpen,
  Pencil,
  Upload,
  WrapText,
  X,
} from 'lucide-react';
import { lazy, Suspense, useMemo, useState } from 'react';
import { z } from 'zod';
import { usePanelFullscreen } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import {
  type FileUpload,
  imageType,
  maxFileBytes,
  safeFilePath,
} from '@/lib/files';
import { request } from '../account/api';
import { ErrorNotice } from '../account/shared';
import type { MarketplaceItem } from './api';
import { SkillFileIcon, SkillFileTree } from './SkillFileTree';

const SkillCodeEditor = lazy(() =>
  import('./SkillCodeEditor').then((module) => ({
    default: module.SkillCodeEditor,
  })),
);
const SkillFilePreview = lazy(() =>
  import('./SkillFilePreview').then((module) => ({
    default: module.SkillFilePreview,
  })),
);

const fileResponse = z.object({
  files: z.record(
    z.string(),
    z.object({
      encoding: z.literal('base64'),
      content: z.string().max(Math.ceil(maxFileBytes / 3) * 4),
      size: z.number().int().nonnegative().max(maxFileBytes),
      executable: z.boolean(),
    }),
  ),
});

function decode(content: string) {
  try {
    const bytes = Uint8Array.from(atob(content), (char) => char.charCodeAt(0));
    const text = new TextDecoder('utf-8', {
      fatal: true,
      ignoreBOM: true,
    }).decode(bytes);
    // biome-ignore lint/suspicious/noControlCharactersInRegex: Binary control bytes must not enter the text editor.
    return /[\x00-\x08\x0e-\x1f]/.test(text) ? null : text;
  } catch {
    return null;
  }
}
export function encodeSkillText(text: string, executable: boolean): FileUpload {
  const bytes = new TextEncoder().encode(text);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return {
    encoding: 'base64',
    content: btoa(binary),
    size: bytes.length,
    executable,
  };
}

export function SkillFileEditor({
  item,
  content,
  setContent,
  uploads,
  deleted,
  onChange,
  disabled,
  manageFiles,
}: {
  item?: MarketplaceItem;
  content: string;
  setContent: (value: string) => void;
  uploads: Record<string, FileUpload>;
  deleted: string[];
  onChange: (files: Record<string, FileUpload>) => void;
  disabled: boolean;
  manageFiles?: () => void;
}) {
  const { t } = useI18n();
  const { fullscreen, toggle } = usePanelFullscreen();
  const [selected, setSelected] = useState('SKILL.md');
  const [tabs, setTabs] = useState(['SKILL.md']);
  const [newPath, setNewPath] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');
  const [sidebar, setSidebar] = useState(true);
  const [wrap, setWrap] = useState(true);
  const [mode, setMode] = useState<'edit' | 'preview' | 'split'>('edit');
  const [focusEditor, setFocusEditor] = useState(false);
  const [cursor, setCursor] = useState({ line: 1, column: 1 });
  const [editorSessions] = useState(() => new Map<string, EditorState>());
  const paths = [
    ...new Set([
      'SKILL.md',
      ...Object.keys(item?.spec?.files ?? {}),
      ...Object.keys(item?.spec?.file_manifest ?? {}),
      ...Object.keys(uploads),
    ]),
  ]
    .filter((path) => !deleted.includes(path))
    .sort((a, b) =>
      a === 'SKILL.md' ? -1 : b === 'SKILL.md' ? 1 : a.localeCompare(b),
    );
  const active = paths.includes(selected) ? selected : 'SKILL.md';
  const isMarkdown = /\.(md|mdx|markdown)$/i.test(active);
  const loaded = useQuery({
    queryKey: ['skill-editor-files', item?.slug, item?.version],
    queryFn: ({ signal }) =>
      request(
        `/marketplace/${encodeURIComponent(item?.slug ?? '')}/files?format=2`,
        fileResponse,
        { signal },
      ),
    enabled:
      !!item &&
      ((!uploads[active] && item.spec?.files?.[active] === undefined) ||
        (isMarkdown && mode !== 'edit' && !!item.spec?.file_manifest)),
    retry: false,
    staleTime: Infinity,
  });
  const file = uploads[active] ?? loaded.data?.files[active];
  const text = useMemo(
    () =>
      active === 'SKILL.md'
        ? content
        : file
          ? decode(file.content)
          : item?.spec?.files?.[active],
    [active, content, file, item],
  );
  const executable =
    file?.executable ??
    item?.spec?.file_manifest?.[active]?.executable ??
    false;
  const viewFile = useMemo(
    () => (typeof text === 'string' ? encodeSkillText(text, executable) : file),
    [text, executable, file],
  );
  const previewFiles = useMemo(
    () =>
      Object.fromEntries(
        Object.entries({
          ...Object.fromEntries(
            Object.entries(item?.spec?.files ?? {}).map(([path, value]) => [
              path,
              encodeSkillText(value, false),
            ]),
          ),
          ...loaded.data?.files,
          ...uploads,
        }).filter(([path]) => !deleted.includes(path)),
      ),
    [item, loaded.data, uploads, deleted],
  );
  const modified = new Set(Object.keys(uploads));
  if (content !== (item?.spec?.files?.['SKILL.md'] ?? ''))
    modified.add('SKILL.md');
  const binary = !!imageType(active) || text === null;
  const visibleMode = binary ? 'preview' : isMarkdown ? mode : 'edit';
  const opened = [
    ...new Set([...tabs.filter((path) => paths.includes(path)), active]),
  ];
  function openFile(path: string) {
    setSelected(path);
    setTabs((current) =>
      current.includes(path) ? current : [...current, path],
    );
    setCursor({ line: 1, column: 1 });
    setFocusEditor(false);
  }
  const preview = (
    <SkillFilePreview
      path={active}
      text={text}
      file={viewFile}
      files={previewFiles}
      paths={paths.filter((path) => !deleted.includes(path))}
      openFile={openFile}
    />
  );
  return (
    <div className="skill-workspace" data-sidebar={sidebar}>
      <div className="skill-workbench-toolbar">
        <div className="skill-toolbar-group">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t(sidebar ? '收起文件目录' : '展开文件目录')}
            title={t(sidebar ? '收起文件目录' : '展开文件目录')}
            onClick={() => setSidebar(!sidebar)}
          >
            {sidebar ? <PanelLeftClose /> : <PanelLeftOpen />}
          </Button>
          <strong>{t('文件工作区')}</strong>
          <span className="skill-file-count">
            {t('{count} 个文件', { count: paths.length })}
          </span>
        </div>
        <div className="skill-toolbar-group">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t('新建文本文件')}
            title={t('新建文本文件')}
            disabled={disabled}
            onClick={() => {
              setSidebar(true);
              setCreating(!creating);
            }}
          >
            <FilePlus2 />
          </Button>
          {manageFiles && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={t('上传文件')}
              title={t('上传文件')}
              disabled={disabled}
              onClick={manageFiles}
            >
              <Upload />
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t(fullscreen ? '还原工作区' : '工作区全屏')}
            title={t(fullscreen ? '还原工作区' : '工作区全屏')}
            aria-pressed={fullscreen}
            onClick={toggle}
          >
            {fullscreen ? <Minimize2 /> : <Maximize2 />}
          </Button>
        </div>
      </div>
      {sidebar && (
        <nav className="skill-files" aria-label={t('技能文件')}>
          <div className="skill-explorer-heading">{t('技能文件')}</div>
          {creating && (
            <div className="skill-new-file">
              <label htmlFor="skill-new-path">{t('新文件路径')}</label>
              <Input
                id="skill-new-path"
                placeholder="references/guide.md"
                value={newPath}
                disabled={disabled}
                onChange={(event) => setNewPath(event.target.value)}
              />
              <Button
                type="button"
                variant="outline"
                disabled={disabled}
                onClick={() => {
                  if (
                    !safeFilePath(newPath) ||
                    paths.some((path) => {
                      const existing = path.toLowerCase(),
                        next = newPath.toLowerCase();
                      return (
                        existing === next ||
                        existing.startsWith(`${next}/`) ||
                        next.startsWith(`${existing}/`)
                      );
                    }) ||
                    deleted.includes(newPath) ||
                    paths.length >= 32
                  ) {
                    setError('路径无效、文件已存在或文件数量已达上限。');
                    return;
                  }
                  onChange({
                    ...uploads,
                    [newPath]: encodeSkillText('', false),
                  });
                  openFile(newPath);
                  setFocusEditor(true);
                  setMode('edit');
                  setNewPath('');
                  setCreating(false);
                  setError('');
                }}
              >
                {t('创建文件')}
              </Button>
              {error && <p role="alert">{t(error)}</p>}
            </div>
          )}
          <SkillFileTree
            paths={paths}
            selected={active}
            modified={modified}
            select={openFile}
            disabled={disabled}
          />
        </nav>
      )}
      <section className="skill-document" aria-label={active}>
        <fieldset className="skill-file-tabs" aria-label={t('已打开文件')}>
          {opened.map((path) => (
            <div
              key={path}
              className="skill-file-tab"
              data-active={active === path}
            >
              <button
                type="button"
                aria-label={t('切换到 {path}', { path })}
                aria-pressed={active === path}
                onClick={() => openFile(path)}
                disabled={disabled}
              >
                <SkillFileIcon path={path} />
                <span>{path.split('/').pop()}</span>
                {modified.has(path) && (
                  <span className="skill-modified-dot" aria-hidden="true" />
                )}
              </button>
              {opened.length > 1 && (
                <button
                  type="button"
                  className="skill-close-tab"
                  aria-label={t('关闭标签 {path}', { path })}
                  disabled={disabled}
                  onClick={() => {
                    const remaining = opened.filter((entry) => entry !== path);
                    setTabs(remaining);
                    if (path === active)
                      openFile(remaining.at(-1) ?? 'SKILL.md');
                  }}
                >
                  <X size={13} aria-hidden="true" />
                </button>
              )}
            </div>
          ))}
        </fieldset>
        <div className="skill-document-toolbar">
          <span className="skill-file-breadcrumb" title={active}>
            {active.split('/').join(' / ')}
          </span>
          <div className="skill-toolbar-group">
            {isMarkdown && (
              <fieldset className="skill-view-modes" aria-label={t('查看方式')}>
                {(
                  [
                    { value: 'edit', label: '编辑', Icon: Pencil },
                    { value: 'preview', label: '预览', Icon: Eye },
                    { value: 'split', label: '分栏', Icon: Columns2 },
                  ] as const
                ).map(({ value, label, Icon }) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={mode === value}
                    onClick={() => setMode(value)}
                  >
                    <Icon size={14} aria-hidden="true" />
                    {t(label)}
                  </button>
                ))}
              </fieldset>
            )}
            {!binary && visibleMode !== 'preview' && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                title={t('自动换行')}
                aria-label={t('自动换行')}
                aria-pressed={wrap}
                onClick={() => setWrap(!wrap)}
              >
                <WrapText />
              </Button>
            )}
            {viewFile && (
              <a
                className="skill-download"
                href={`data:application/octet-stream;base64,${viewFile.content}`}
                download={active.split('/').pop()}
              >
                {t('下载文件')}
              </a>
            )}
          </div>
        </div>
        <div className="skill-editor-panes" data-mode={visibleMode}>
          <Suspense
            fallback={
              <p className="skill-file-notice" role="status">
                {t('正在载入编辑器…')}
              </p>
            }
          >
            {typeof text === 'string' && !binary ? (
              <>
                <div
                  className="skill-source-pane"
                  hidden={visibleMode === 'preview'}
                >
                  <SkillCodeEditor
                    sessions={editorSessions}
                    path={active}
                    value={text}
                    label={t('{path} 内容', { path: active })}
                    disabled={disabled}
                    wrap={wrap}
                    focus={focusEditor}
                    onCursor={(line, column) => setCursor({ line, column })}
                    onChange={(value) =>
                      active === 'SKILL.md'
                        ? setContent(value)
                        : onChange({
                            ...uploads,
                            [active]: encodeSkillText(value, executable),
                          })
                    }
                  />
                </div>
                {visibleMode !== 'edit' && preview}
              </>
            ) : file || typeof text === 'string' ? (
              preview
            ) : loaded.isError ? (
              <div className="skill-file-notice">
                <ErrorNotice error={loaded.error} />
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void loaded.refetch()}
                >
                  {t('重试')}
                </Button>
              </div>
            ) : loaded.isFetching ? (
              <p role="status" className="skill-file-notice">
                {t('正在读取文件…')}
              </p>
            ) : (
              <p role="alert" className="skill-file-notice">
                {t('文件内容不可用，请关闭后重试。')}
              </p>
            )}
          </Suspense>
        </div>
        {isMarkdown && mode !== 'edit' && loaded.isError && (
          <div className="skill-preview-error">
            <ErrorNotice error={loaded.error} />
            <Button
              type="button"
              variant="ghost"
              onClick={() => void loaded.refetch()}
            >
              {t('重试')}
            </Button>
          </div>
        )}
        <footer className="skill-editor-status">
          <span>{modified.has(active) ? t('未保存') : t('已保存')}</span>
          <div>
            <span>{t('行 {line}，列 {column}', cursor)}</span>
            <span>
              {viewFile ? `${(viewFile.size / 1024).toFixed(1)} KB` : ''}
            </span>
            <span>
              {executable
                ? t('可执行')
                : binary
                  ? (active.split('.').pop()?.toUpperCase() ?? '')
                  : 'UTF-8'}
            </span>
            <span>{isMarkdown ? 'Markdown' : ''}</span>
          </div>
        </footer>
      </section>
    </div>
  );
}
