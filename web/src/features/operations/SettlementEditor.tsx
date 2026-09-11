import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice } from '../account/shared';

export type RuleDraft = {
  mode: string;
  path: string;
  equals: string;
  pattern: string;
  script: string;
  json: string;
};
const contentSchema = z.object({
  path: z.string().optional(),
  equals: z.unknown().optional(),
  pattern: z.string().optional(),
});
export function ruleDraft(value: Record<string, unknown> = {}): RuleDraft {
  const content = contentSchema.safeParse(value.content);
  const script = typeof value.script === 'string' ? value.script : '';
  const mode = script
    ? 'script'
    : content.success && content.data.pattern
      ? 'pattern'
      : content.success && content.data.path
        ? 'path'
        : Object.keys(value).length
          ? 'json'
          : 'default';
  return {
    mode,
    path: content.success ? (content.data.path ?? 'code') : 'code',
    equals: content.success
      ? JSON.stringify(
          content.data.equals === undefined ? 0 : content.data.equals,
        )
      : '0',
    pattern: content.success ? (content.data.pattern ?? '^OK') : '^OK',
    script:
      script ||
      'const body = JSON.parse(result.text);\nreturn body.code === 0;',
    json: Object.keys(value).length ? JSON.stringify(value, null, 2) : '',
  };
}
export function settlementValue(draft: RuleDraft): Record<string, unknown> {
  if (draft.mode === 'default') return {};
  if (draft.mode === 'path') {
    if (!draft.path.trim() || draft.path.length > 64)
      throw new Error('请填写成功字段路径，例如 data.code（最多 64 字节）。');
    let equals: unknown;
    try {
      equals = JSON.parse(draft.equals);
    } catch {
      throw new Error('成功值必须是 JSON，例如 0、"ok"、true 或 [0,200]。');
    }
    const scalar = z.union([z.string(), z.number(), z.boolean(), z.null()]);
    if (
      !z
        .union([
          scalar,
          z
            .array(z.union([z.string(), z.number(), z.boolean()]))
            .min(1)
            .max(16),
        ])
        .safeParse(equals).success
    )
      throw new Error('成功值必须是 JSON，例如 0、"ok"、true 或 [0,200]。');
    return { content: { path: draft.path.trim(), equals } };
  }
  if (draft.mode === 'pattern') {
    if (!draft.pattern || new TextEncoder().encode(draft.pattern).length > 256)
      throw new Error('请填写文本匹配表达式（最多 256 字节）。');
    return { content: { pattern: draft.pattern } };
  }
  let value: Record<string, unknown>;
  if (draft.mode === 'script') value = { script: draft.script };
  else {
    try {
      value = z
        .record(z.string(), z.unknown())
        .parse(draft.json.trim() ? JSON.parse(draft.json) : {});
    } catch {
      throw new Error('结算策略必须是 JSON 对象，请检查引号和逗号。');
    }
  }
  if (typeof value.script === 'string') {
    if (
      !value.script.trim() ||
      new TextEncoder().encode(value.script).length > 8192
    )
      throw new Error('脚本不能为空，且不能超过 8 KiB。');
    try {
      new Function('result', `"use strict";\n${value.script}`);
    } catch {
      throw new Error('结算脚本语法有误，请检查后重试。');
    }
  }
  return value;
}

export default function SettlementEditor({
  value,
  onChange,
  disabled,
}: {
  value: RuleDraft;
  onChange: (value: RuleDraft) => void;
  disabled: boolean;
}) {
  const { t } = useI18n();
  const [text, setText] = useState('');
  const [isError, setIsError] = useState(false);
  const [localError, setLocalError] = useState('');
  const preview = useMutation({
    gcTime: 0,
    mutationFn: (body: {
      settlement: Record<string, unknown>;
      text: string;
      is_error: boolean;
    }) =>
      request(
        '/admin/tools/settlement-preview',
        z.object({ charge: z.boolean() }),
        { method: 'POST', body: JSON.stringify(body) },
      ),
  });
  const reset = () => {
    preview.reset();
    setLocalError('');
  };
  const change = (patch: Partial<RuleDraft>) => {
    reset();
    onChange({ ...value, ...patch });
  };
  const changeMode = (mode: string) => {
    try {
      if (mode === 'json')
        change({ mode, json: JSON.stringify(settlementValue(value), null, 2) });
      else if (value.mode === 'json') {
        const parsed = ruleDraft(settlementValue(value));
        change({ ...parsed, mode });
      } else change({ mode });
    } catch (error) {
      setLocalError(
        error instanceof Error
          ? error.message
          : '结算策略必须是 JSON 对象，请检查引号和逗号。',
      );
    }
  };
  return (
    <div className="tool-rule-fields">
      <p className="field-help">
        {t(
          '先预留每次调用额度，返回结果满足条件才扣费；上游报错、超时或规则未通过时退回额度。',
        )}
      </p>
      <Field>
        <FieldLabel htmlFor="rule-mode">{t('扣费条件')}</FieldLabel>
        <Select
          id="rule-mode"
          value={value.mode}
          disabled={disabled || preview.isPending}
          onValueChange={changeMode}
          options={[
            { value: 'default', label: t('协议成功即扣费') },
            { value: 'path', label: t('JSON 字段匹配') },
            { value: 'pattern', label: t('返回文本匹配') },
            { value: 'script', label: t('JavaScript 脚本') },
            { value: 'json', label: t('高级 JSON') },
          ]}
        />
      </Field>
      {value.mode === 'default' && (
        <p className="field-help">
          {t(
            '适合正常使用 MCP isError 标志的服务。HTTP 200 不代表业务成功；若返回 code 等业务码，请选 JSON 字段匹配。',
          )}
        </p>
      )}
      {value.mode === 'path' && (
        <>
          <Field>
            <FieldLabel htmlFor="rule-path">{t('成功字段路径')}</FieldLabel>
            <Input
              id="rule-path"
              value={value.path}
              onChange={(e) => change({ path: e.target.value })}
              aria-describedby="rule-path-help"
            />
            <p id="rule-path-help" className="field-help">
              {t(
                '例如返回 {"data":{"code":0}}，路径填 data.code；只支持对象字段，用英文句点连接，不支持数组下标。',
              )}
            </p>
          </Field>
          <Field>
            <FieldLabel htmlFor="rule-equals">{t('成功值（JSON）')}</FieldLabel>
            <Input
              id="rule-equals"
              value={value.equals}
              onChange={(e) => change({ equals: e.target.value })}
              aria-describedby="rule-equals-help"
            />
            <p id="rule-equals-help" className="field-help">
              {t(
                '数字填 0，字符串填 "ok"，布尔值填 true；多个成功码填 [0,200]（最多 16 个）。字段缺失或返回内容不是 JSON 时退款。',
              )}
            </p>
          </Field>
        </>
      )}
      {value.mode === 'pattern' && (
        <Field>
          <FieldLabel htmlFor="rule-pattern">{t('文本匹配表达式')}</FieldLabel>
          <Input
            id="rule-pattern"
            value={value.pattern}
            onChange={(e) => change({ pattern: e.target.value })}
            aria-describedby="rule-pattern-help"
          />
          <p id="rule-pattern-help" className="field-help">
            {t(
              '例如 ^OK 匹配以 OK 开头的返回文本。使用 Go 正则语法，不支持前后查找或反向引用；先用下方试算验证。',
            )}
          </p>
        </Field>
      )}
      {value.mode === 'script' && (
        <Field>
          <FieldLabel htmlFor="rule-script">{t('判定脚本')}</FieldLabel>
          <textarea
            id="rule-script"
            rows={5}
            spellCheck={false}
            value={value.script}
            onChange={(e) => change({ script: e.target.value })}
            aria-describedby="rule-script-help"
          />
          <p id="rule-script-help" className="field-help">
            {t(
              'result.text 是所有文本块拼接的字符串，result.isError 是协议错误标志。return 真值才扣费；异常或超过 200ms 即退款。沙箱不能联网、读文件或访问 Node.js，脚本最多 8 KiB。',
            )}
          </p>
        </Field>
      )}
      {value.mode === 'json' && (
        <Field>
          <FieldLabel htmlFor="tool-settlement">
            {t('结算策略（JSON，可选）')}
          </FieldLabel>
          <textarea
            id="tool-settlement"
            rows={6}
            spellCheck={false}
            value={value.json}
            onChange={(e) => change({ json: e.target.value })}
            aria-describedby="rule-json-help"
          />
          <p id="rule-json-help" className="field-help">
            {t(
              '留空或 {} 恢复默认。三种规则互斥：content.path + equals、content.pattern、script。保存前可用下方真实引擎试算。',
            )}
          </p>
          <pre className="schema-code">
            <code>{'{"content":{"path":"code","equals":0}}'}</code>
          </pre>
        </Field>
      )}
      <details className="tool-help">
        <summary>{t('如何选择规则？')}</summary>
        <dl>
          <dt>{t('协议成功即扣费')}</dt>
          <dd>{t('上游已经通过 isError 正确标记失败时使用。')}</dd>
          <dt>{t('JSON 字段匹配')}</dt>
          <dd>{t('有明确业务成功码时使用，例如 code=0 或 status="ok"。')}</dd>
          <dt>{t('返回文本匹配')}</dt>
          <dd>{t('返回自然语言或纯文本，成功结果有稳定特征时使用。')}</dd>
          <dt>{t('JavaScript 脚本')}</dt>
          <dd>{t('需要同时检查多个字段时使用；优先采用前面的简单规则。')}</dd>
        </dl>
      </details>
      <div className="rule-preview">
        <h4>{t('保存前试算')}</h4>
        <p className="field-help">
          {t(
            '粘贴一次真实上游返回的文本内容，用服务端相同引擎判定。不会调用上游、保存工具或扣额度。',
          )}
        </p>
        <Field>
          <FieldLabel htmlFor="rule-sample">{t('上游返回文本')}</FieldLabel>
          <textarea
            id="rule-sample"
            rows={4}
            value={text}
            onChange={(e) => {
              reset();
              setText(e.target.value);
            }}
            placeholder={'{"code":0,"data":{}}'}
          />
        </Field>
        <label className="checkbox-field">
          <input
            type="checkbox"
            checked={isError}
            onChange={(e) => {
              reset();
              setIsError(e.target.checked);
            }}
          />
          {t('上游标记为错误（isError）')}
        </label>
        <Button
          type="button"
          variant="outline"
          disabled={disabled || preview.isPending}
          onClick={() => {
            reset();
            try {
              const settlement = settlementValue(value);
              if (new TextEncoder().encode(text).length > 65536)
                throw new Error('试算文本不能超过 64 KiB。');
              preview.mutate({ settlement, text, is_error: isError });
            } catch (error) {
              setLocalError(
                error instanceof Error
                  ? error.message
                  : '结算策略必须是 JSON 对象，请检查引号和逗号。',
              );
            }
          }}
        >
          {preview.isPending ? t('正在试算…') : t('试算规则')}
        </Button>
        {localError && (
          <p role="alert" className="field-error">
            {t(localError)}
          </p>
        )}
        <ErrorNotice error={preview.error} />
        {preview.data && (
          <p
            role="status"
            className="rule-result"
            data-tone={preview.data.charge ? 'success' : 'neutral'}
          >
            {preview.data.charge
              ? t('满足条件，将扣除每次调用额度')
              : t('不扣费，退回预留额度')}
          </p>
        )}
      </div>
    </div>
  );
}
