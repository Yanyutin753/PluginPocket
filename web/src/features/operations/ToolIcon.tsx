import { useId, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import './ToolIcon.css';

type ToolIconProps = { icon?: string; name: string };
type ToolIconEditorProps = {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
};
const maxBytes = 64 * 1024;
const invalidMessage = '请输入无账号密码的 HTTPS 图片地址，或安全的 SVG';
const uploadMessage = '请选择 SVG、PNG、JPEG 或 WebP 图片，大小不超过 64 KiB';
const imageTypes = new Set([
  'image/svg+xml',
  'image/png',
  'image/jpeg',
  'image/webp',
]);

function svgData(source: string) {
  if (
    new TextEncoder().encode(source).length > maxBytes ||
    /<!DOCTYPE|<!ENTITY/i.test(source)
  )
    throw new Error(invalidMessage);
  const doc = new DOMParser().parseFromString(source, 'image/svg+xml');
  if (
    doc.querySelector('parsererror') ||
    doc.documentElement.localName !== 'svg'
  )
    throw new Error(invalidMessage);
  for (const element of doc.querySelectorAll('*')) {
    if (
      /^(script|foreignObject|iframe|object|embed|style|animate|animateMotion|animateTransform|set)$/i.test(
        element.localName,
      )
    )
      throw new Error(invalidMessage);
    for (const attribute of element.attributes) {
      if (
        /^on/i.test(attribute.localName) ||
        attribute.localName === 'base' ||
        (attribute.localName === 'href' && !attribute.value.startsWith('#')) ||
        /url\s*\(\s*[^#]|@import|javascript:/i.test(attribute.value)
      )
        throw new Error(invalidMessage);
    }
  }
  return `data:image/svg+xml;base64,${btoa(Array.from(new TextEncoder().encode(source), (byte) => String.fromCharCode(byte)).join(''))}`;
}

function parseIcon(source: string) {
  const value = source.trim();
  if (!value) return '';
  if (value.startsWith('<')) return svgData(value);
  const data =
    /^data:(image\/(?:svg\+xml|png|jpeg|webp));base64,([A-Za-z0-9+/]*={0,2})$/.exec(
      value,
    );
  if (data) {
    const bytes = atob(data[2]);
    if (!bytes.length || bytes.length > maxBytes)
      throw new Error(uploadMessage);
    return data[1] === 'image/svg+xml'
      ? svgData(
          new TextDecoder('utf-8', { fatal: true }).decode(
            Uint8Array.from(bytes, (char) => char.charCodeAt(0)),
          ),
        )
      : value;
  }
  const url = new URL(value);
  if (
    url.protocol !== 'https:' ||
    url.username ||
    url.password ||
    value.length > 2048
  )
    throw new Error(invalidMessage);
  return value;
}

export function ToolIcon({ icon, name }: ToolIconProps) {
  const [failed, setFailed] = useState<string>();
  return (
    <img
      className="custom-tool-icon"
      src={icon && failed !== icon ? icon : '/images/workshop-tool-icon.png'}
      alt={name}
      width={40}
      height={40}
      referrerPolicy="no-referrer"
      onError={() => setFailed(icon)}
    />
  );
}

export function ToolIconEditor({
  value,
  onChange,
  disabled,
}: ToolIconEditorProps) {
  const { t } = useI18n();
  const id = useId();
  const field = useRef<HTMLTextAreaElement>(null);
  const revision = useRef(0);
  const [draft, setDraft] = useState(value.startsWith('data:') ? '' : value);
  const [filename, setFilename] = useState('');
  const [error, setError] = useState('');
  const [reading, setReading] = useState(false);
  function report(message: string) {
    setReading(false);
    setError(message);
    field.current?.setCustomValidity(message ? t(message) : '');
  }
  function accept(source: string) {
    try {
      const next = parseIcon(source);
      report('');
      onChange(next);
      return true;
    } catch {
      report(invalidMessage);
      return false;
    }
  }
  return (
    <Field className="tool-icon-editor">
      <div className="tool-icon-actions">
        <ToolIcon icon={value} name={value ? t('图标预览') : ''} />
        <div className="tool-icon-upload">
          <FieldLabel htmlFor={`${id}-file`}>{t('上传图标')}</FieldLabel>
          <Input
            id={`${id}-file`}
            type="file"
            accept=".svg,.png,.jpg,.jpeg,.webp,image/svg+xml,image/png,image/jpeg,image/webp"
            disabled={disabled}
            onChange={(event) => {
              const file = event.target.files?.[0];
              event.target.value = '';
              if (!file) return;
              const currentRevision = ++revision.current;
              if (
                !imageTypes.has(file.type) ||
                file.size > maxBytes ||
                !file.size
              ) {
                report(uploadMessage);
                return;
              }
              report('');
              setReading(true);
              field.current?.setCustomValidity(t('正在读取图标，请稍候'));
              const reader = new FileReader();
              reader.onerror = () => {
                if (revision.current === currentRevision)
                  report('图片读取失败，请重新选择文件');
              };
              reader.onload = () => {
                if (revision.current !== currentRevision) return;
                const source = String(reader.result);
                if (accept(source)) {
                  setDraft('');
                  setFilename(file.name);
                }
              };
              reader.readAsDataURL(file);
            }}
          />
          {value.startsWith('data:') && !draft && (
            <p className="field-help tool-icon-filename">
              {filename || t('已上传图片')}
            </p>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          disabled={disabled || (!value && !draft && !error && !reading)}
          onClick={() => {
            revision.current++;
            setDraft('');
            setFilename('');
            report('');
            onChange('');
          }}
        >
          {t('移除图标')}
        </Button>
      </div>
      <FieldLabel htmlFor={id}>{t('图标地址或 SVG')}</FieldLabel>
      <textarea
        id={id}
        ref={field}
        rows={3}
        value={draft}
        disabled={disabled}
        aria-invalid={!!error}
        aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
        onChange={(event) => {
          revision.current++;
          setDraft(event.target.value);
          if (accept(event.target.value)) setFilename('');
        }}
      />
      <p id={`${id}-help`} className="field-help">
        {t(
          '支持 HTTPS 地址、粘贴 SVG 或上传 SVG / PNG / JPEG / WebP（最多 64 KiB）。保存后图标配置存入共享数据库，所有服务副本共用。',
        )}{' '}
        {t(
          'SVG 支持静态图形和文字；样式请写成 fill、stroke 等属性，不支持 style、动画或外部资源。',
        )}
      </p>
      {reading && (
        <p role="status" className="field-help">
          {t('正在读取图标，请稍候')}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="tool-icon-error">
          {t(error)}
        </p>
      )}
    </Field>
  );
}
