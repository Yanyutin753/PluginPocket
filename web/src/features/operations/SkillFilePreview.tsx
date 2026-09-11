import { File, ImageOff } from 'lucide-react';
import { useState } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useI18n } from '@/i18n';
import { type FileUpload, imageType, safeFilePath } from '@/lib/files';

function relativePath(from: string, target: string) {
  try {
    const value = decodeURIComponent(target.split(/[?#]/)[0]);
    if (!value || /^[a-z][a-z\d+.-]*:|^[/\\]/i.test(value)) return '';
    const parts = from.split('/').slice(0, -1);
    for (const part of value.split('/')) {
      if (part === '..') {
        if (!parts.length) return '';
        parts.pop();
      } else if (part !== '.') parts.push(part);
    }
    const path = parts.join('/');
    return safeFilePath(path) ? path : '';
  } catch {
    return '';
  }
}

function PreviewImage({
  path,
  content,
  alt,
}: {
  path: string;
  content: string;
  alt: string;
}) {
  const { t } = useI18n();
  const [failedContent, setFailedContent] = useState<string | null>(null);
  return failedContent === content ? (
    <span role="status" className="skill-image-error">
      <ImageOff aria-hidden="true" />
      {t('图片无法预览，请下载查看。')}
    </span>
  ) : (
    <img
      src={`data:${imageType(path)};base64,${content}`}
      alt={alt}
      onError={() => setFailedContent(content)}
    />
  );
}

export function SkillFilePreview({
  path,
  text,
  file,
  files,
  paths,
  openFile,
}: {
  path: string;
  text: string | null | undefined;
  file?: Pick<FileUpload, 'content' | 'size'>;
  files: Record<string, Pick<FileUpload, 'content' | 'size'>>;
  paths: string[];
  openFile: (path: string) => void;
}) {
  const { t } = useI18n();
  if (imageType(path) && file)
    return (
      <section className="skill-image-preview" aria-label={t('文件预览')}>
        <PreviewImage
          key={path}
          path={path}
          content={file.content}
          alt={path}
        />
      </section>
    );
  if (typeof text === 'string')
    return (
      <section className="skill-markdown-preview" aria-label={t('文件预览')}>
        {/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.test(text) && (
          <details className="skill-frontmatter">
            <summary>{t('技能元数据')}</summary>
            <pre>
              {text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/)?.[1]}
            </pre>
          </details>
        )}
        <Markdown
          remarkPlugins={[remarkGfm]}
          skipHtml
          components={{
            a: ({ href, children }) => {
              const local = relativePath(path, href ?? '');
              if (paths.includes(local))
                return (
                  <button
                    type="button"
                    className="skill-document-link"
                    onClick={() => openFile(local)}
                  >
                    {children}
                  </button>
                );
              return href && /^https?:\/\//i.test(href) ? (
                <a href={href} target="_blank" rel="noreferrer">
                  {children}
                </a>
              ) : (
                <span>{children}</span>
              );
            },
            img: ({ src, alt }) => {
              const local = relativePath(
                path,
                typeof src === 'string' ? src : '',
              );
              return files[local] && imageType(local) ? (
                <PreviewImage
                  key={local}
                  path={local}
                  content={files[local].content}
                  alt={alt ?? local}
                />
              ) : (
                <span className="skill-image-placeholder">
                  {alt || t('图片')} · {t('仅预览技能内的图片')}
                </span>
              );
            },
          }}
        >
          {text.replace(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/, '')}
        </Markdown>
      </section>
    );
  return (
    <section className="skill-binary-preview" aria-label={t('文件预览')}>
      <File aria-hidden="true" size={40} />
      <strong>{path.split('/').pop()}</strong>
      <p>{t('此文件不支持文本编辑，可在文件管理中上传替换。')}</p>
      <span>{file?.size ?? 0} bytes</span>
    </section>
  );
}
