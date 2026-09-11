import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice } from '../account/shared';
import { type Tool, toolSchema } from './api';
import SettlementEditor, {
  ruleDraft,
  settlementValue,
} from './SettlementEditor';
import { ToolIconEditor } from './ToolIcon';
import './ToolEditor.css';

const object = z.record(z.string(), z.unknown());
function jsonObject(raw: string, message: string) {
  try {
    return object.parse(JSON.parse(raw));
  } catch {
    throw new Error(message);
  }
}

export default function ToolEditor({
  item,
  close,
}: {
  item: Tool | null;
  close: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [icon, setIcon] = useState(item?.icon ?? '');
  const [rule, setRule] = useState(() => ruleDraft(item?.settlement));
  const [updateConnection, setUpdateConnection] = useState(!item);
  const [connectionMode, setConnectionMode] = useState('fields');
  const [validation, setValidation] = useState('');
  const form = useForm({
    defaultValues: {
      key: item?.key ?? '',
      name: item?.name ?? '',
      description: item?.description ?? '',
      kind: item?.kind ?? 'builtin',
      units_per_call: item?.units_per_call ?? 1,
      input_schema: JSON.stringify(
        item?.input_schema ?? { type: 'object', properties: {} },
        null,
        2,
      ),
      config: '',
      url: '',
      token: '',
      headers: '',
      command: '',
      args: '',
      env: '',
    },
  });
  const kind = form.watch('kind');
  const replaceConnection = updateConnection || kind !== item?.kind;
  const save = useMutation({
    gcTime: 0,
    mutationFn: (body: Record<string, unknown>) =>
      request(
        `/admin/tools${item ? `/${item.id}` : ''}`,
        z.object({ item: toolSchema }),
        { method: item ? 'PATCH' : 'POST', body: JSON.stringify(body) },
      ),
    onSuccess: () => {
      form.reset();
      void client.invalidateQueries({ queryKey: ['/admin/tools'] });
      void client.invalidateQueries({ queryKey: ['/tools'] });
      close();
    },
  });
  const submit = form.handleSubmit((value) => {
    setValidation('');
    save.reset();
    try {
      const input_schema = jsonObject(
        value.input_schema,
        '参数 Schema 必须是 JSON 对象，请检查引号和逗号。',
      );
      if (input_schema.type !== 'object')
        throw new Error('参数 Schema 的 type 必须为 object。');
      let config: Record<string, unknown> | undefined;
      if (kind !== 'builtin' && replaceConnection) {
        if (connectionMode === 'json')
          config = jsonObject(
            value.config,
            '连接配置必须是 JSON 对象，请检查引号和逗号。',
          );
        else if (kind === 'http') {
          let address: URL;
          try {
            address = new URL(value.url);
          } catch {
            throw new Error('请输入完整的 HTTP 或 HTTPS MCP 服务地址。');
          }
          if (
            !['https:', 'http:'].includes(address.protocol) ||
            address.username ||
            address.password
          )
            throw new Error('请输入完整的 HTTP 或 HTTPS MCP 服务地址。');
          let headers: Record<string, string> = {};
          if (value.headers.trim()) {
            const parsed = z
              .record(z.string(), z.string())
              .safeParse(
                jsonObject(
                  value.headers,
                  '请求头必须是 JSON 对象，名称和值均为字符串。',
                ),
              );
            if (!parsed.success)
              throw new Error('请求头必须是 JSON 对象，名称和值均为字符串。');
            headers = parsed.data;
          }
          if (value.token.trim()) {
            for (const key of Object.keys(headers))
              if (key.toLowerCase() === 'authorization') delete headers[key];
            headers.Authorization = `Bearer ${value.token.trim()}`;
          }
          config = {
            url: value.url.trim(),
            ...(Object.keys(headers).length ? { headers } : {}),
          };
        } else {
          if (!value.command.trim())
            throw new Error('请填写服务器允许的命令别名。');
          let args: string[] = [];
          try {
            args = value.args.trim()
              ? z.array(z.string()).parse(JSON.parse(value.args))
              : [];
          } catch {
            throw new Error(
              '启动参数必须是 JSON 字符串数组，例如 ["--port","3000"]。',
            );
          }
          const env = value.env.trim()
            ? z
                .record(z.string(), z.string())
                .safeParse(
                  jsonObject(
                    value.env,
                    '环境变量必须是 JSON 对象，名称和值均为字符串。',
                  ),
                )
            : null;
          if (env && !env.success)
            throw new Error('环境变量必须是 JSON 对象，名称和值均为字符串。');
          config = {
            command: value.command.trim(),
            ...(args.length ? { args } : {}),
            ...(env?.success ? { env: env.data } : {}),
          };
        }
      }
      save.mutate({
        key: value.key,
        name: value.name,
        description: value.description,
        kind,
        enabled: item?.enabled ?? true,
        units_per_call: Number(value.units_per_call),
        input_schema,
        ...(icon || item?.icon ? { icon } : {}),
        ...(config ? { config } : {}),
        ...(kind !== 'builtin' ? { settlement: settlementValue(rule) } : {}),
      });
    } catch (error) {
      setValidation(
        error instanceof Error ? error.message : '请检查填写内容。',
      );
    }
  });
  return (
    <SidePanel
      title={item ? t('编辑工具') : t('添加工具')}
      onClose={close}
      locked={save.isPending}
    >
      <form className="tool-editor" onSubmit={submit}>
        <p className="tool-editor-intro">
          {t(
            '先完善工具信息，再连接上游并设置扣费条件。所有配置由服务端保存。',
          )}
        </p>
        <fieldset disabled={save.isPending} className="tool-editor-fields">
          <section
            className="tool-editor-section"
            aria-labelledby="tool-basics-heading"
          >
            <h3 id="tool-basics-heading">{t('基本信息与图标')}</h3>
            <ToolIconEditor
              value={icon}
              onChange={setIcon}
              disabled={save.isPending}
            />
            <div className="tool-field-pair">
              <Field>
                <FieldLabel htmlFor="tool-name">{t('显示名称')}</FieldLabel>
                <Input
                  id="tool-name"
                  required
                  maxLength={80}
                  {...form.register('name')}
                  placeholder={t('例如：文档搜索')}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="tool-key">{t('工具标识')}</FieldLabel>
                <Input
                  id="tool-key"
                  required
                  {...form.register('key')}
                  readOnly={!!item && item.kind === 'builtin'}
                  placeholder="docs-search"
                  aria-describedby="tool-key-help"
                />
              </Field>
            </div>
            <p id="tool-key-help" className="field-help">
              {t(
                '标识用于网关调用，需唯一；使用 3–32 位字母、数字、下划线或连字符。内置工具的标识不可更改。',
              )}
            </p>
            <Field>
              <FieldLabel htmlFor="tool-description">
                {t('工具说明')}
              </FieldLabel>
              <textarea
                id="tool-description"
                rows={3}
                maxLength={2000}
                {...form.register('description')}
                placeholder={t(
                  '说明工具能做什么、适合何时使用，以及必要的输入。',
                )}
              />
            </Field>
          </section>

          <section
            className="tool-editor-section"
            aria-labelledby="tool-connection-heading"
          >
            <h3 id="tool-connection-heading">{t('连接上游')}</h3>
            <Field>
              <FieldLabel htmlFor="tool-kind">{t('连接方式')}</FieldLabel>
              <Select
                id="tool-kind"
                value={kind}
                disabled={save.isPending || !!item}
                onValueChange={(value) =>
                  form.setValue(
                    'kind',
                    value === 'http'
                      ? 'http'
                      : value === 'stdio'
                        ? 'stdio'
                        : 'builtin',
                    { shouldDirty: true },
                  )
                }
                options={[
                  { value: 'builtin', label: t('内置工具') },
                  { value: 'http', label: t('HTTP 服务') },
                  { value: 'stdio', label: t('本地进程') },
                ]}
              />
            </Field>
            {kind === 'builtin' ? (
              <p className="field-help">
                {t(
                  '内置工具由 Loadout 执行，无需连接地址。新增仅支持 echo（回声）和 time_now（当前时间）；接入自己的工具请选择 HTTP 服务。',
                )}
              </p>
            ) : (
              <>
                <p className="field-help">
                  {kind === 'http'
                    ? t(
                        '填写支持 MCP 的 HTTP 端点，不是普通 REST API。服务地址必须符合服务器允许的网络范围。',
                      )
                    : t(
                        '进程在服务端运行。每个副本都必须安装相同程序，并配置 LOADOUT_STDIO_COMMANDS 允许名单；浏览器所在电脑不会启动进程。',
                      )}
                </p>
                {item && (
                  <label className="checkbox-field">
                    <input
                      type="checkbox"
                      checked={updateConnection}
                      onChange={(e) => setUpdateConnection(e.target.checked)}
                    />
                    {t('更新连接配置')}
                  </label>
                )}
                {item && !replaceConnection && (
                  <p className="field-help">
                    {t(
                      '已保存的连接与凭证继续使用，不会回显。需要修改时勾选更新，并完整填写新配置。',
                    )}
                  </p>
                )}
                {replaceConnection && (
                  <>
                    <Field>
                      <FieldLabel htmlFor="connection-mode">
                        {t('填写方式')}
                      </FieldLabel>
                      <Select
                        id="connection-mode"
                        value={connectionMode}
                        onValueChange={setConnectionMode}
                        options={[
                          { value: 'fields', label: t('引导填写') },
                          { value: 'json', label: t('高级 JSON') },
                        ]}
                      />
                    </Field>
                    {connectionMode === 'json' ? (
                      <Field>
                        <FieldLabel htmlFor="tool-config">
                          {t('连接配置（JSON）')}
                        </FieldLabel>
                        <textarea
                          id="tool-config"
                          rows={6}
                          {...form.register('config')}
                          placeholder={
                            kind === 'http'
                              ? '{"url":"https://mcp.example.com/mcp","headers":{}}'
                              : '{"command":"allowed-alias","args":[],"env":{}}'
                          }
                        />
                        <p className="field-help">
                          {t(
                            '这里的示例需要替换为你实际使用的地址或命令；新配置会完整替换旧配置。',
                          )}
                        </p>
                      </Field>
                    ) : kind === 'http' ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="tool-url">
                            {t('MCP 服务地址')}
                          </FieldLabel>
                          <Input
                            id="tool-url"
                            type="url"
                            required
                            {...form.register('url')}
                            placeholder="https://mcp.example.com/mcp"
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="tool-token">
                            {t('Bearer 令牌（可选）')}
                          </FieldLabel>
                          <Input
                            id="tool-token"
                            type="password"
                            autoComplete="new-password"
                            {...form.register('token')}
                          />
                          <p className="field-help">
                            {t(
                              '只填写令牌本身，系统会添加 Bearer 前缀；只使用运营方的上游凭证。',
                            )}
                          </p>
                        </Field>
                        <details className="tool-help">
                          <summary>{t('自定义请求头（可选）')}</summary>
                          <Field>
                            <FieldLabel htmlFor="tool-headers">
                              {t('请求头（JSON）')}
                            </FieldLabel>
                            <textarea
                              id="tool-headers"
                              rows={3}
                              {...form.register('headers')}
                              placeholder={'{"X-API-Key":"your-key"}'}
                            />
                            <p className="field-help">
                              {t(
                                '名称和值都必须是字符串。填写 Bearer 令牌时，它会覆盖 Authorization 请求头。',
                              )}
                            </p>
                          </Field>
                        </details>
                      </>
                    ) : (
                      <>
                        <Field>
                          <FieldLabel htmlFor="tool-command">
                            {t('命令别名')}
                          </FieldLabel>
                          <Input
                            id="tool-command"
                            required
                            {...form.register('command')}
                            placeholder="allowed-alias"
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="tool-args">
                            {t('启动参数（JSON 数组，可选）')}
                          </FieldLabel>
                          <Input
                            id="tool-args"
                            {...form.register('args')}
                            placeholder={'["--port","3000"]'}
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="tool-env">
                            {t('环境变量（JSON，可选）')}
                          </FieldLabel>
                          <textarea
                            id="tool-env"
                            rows={3}
                            {...form.register('env')}
                            placeholder={'{"API_KEY":"your-key"}'}
                          />
                        </Field>
                      </>
                    )}
                  </>
                )}
              </>
            )}
          </section>

          <section
            className="tool-editor-section"
            aria-labelledby="tool-parameters-heading"
          >
            <h3 id="tool-parameters-heading">{t('调用参数')}</h3>
            <p className="field-help">
              {kind === 'builtin'
                ? t(
                    'Schema 描述调用时允许的输入。type 固定为 object，properties 定义字段，required 列出必填字段。',
                  )
                : t(
                    '这里保存工具的参数说明；HTTP 和本地进程的实际可调用工具由上游发现。保存后通过“查看参数”管理上游工具及参数覆盖。',
                  )}
            </p>
            <Field>
              <FieldLabel htmlFor="tool-schema">
                {t('参数 Schema（JSON）')}
              </FieldLabel>
              <textarea
                id="tool-schema"
                rows={7}
                required
                spellCheck={false}
                {...form.register('input_schema')}
              />
            </Field>
            <details className="tool-help">
              <summary>{t('参数填写示例')}</summary>
              <p>
                {t(
                  '下面是必填文本 query 的示例。description 告诉 AI 应该传什么；required 使用字段名数组，不能写成 true。',
                )}
              </p>
              <pre className="schema-code">
                <code>
                  {JSON.stringify(
                    {
                      type: 'object',
                      properties: {
                        query: {
                          type: 'string',
                          description: t('要搜索的关键词'),
                        },
                      },
                      required: ['query'],
                    },
                    null,
                    2,
                  )}
                </code>
              </pre>
            </details>
            <Button
              type="button"
              variant="outline"
              onClick={() =>
                form.setValue(
                  'input_schema',
                  JSON.stringify(
                    {
                      type: 'object',
                      properties: {
                        query: {
                          type: 'string',
                          description: t('要搜索的关键词'),
                        },
                      },
                      required: ['query'],
                    },
                    null,
                    2,
                  ),
                  { shouldDirty: true },
                )
              }
            >
              {t('填入文本参数示例')}
            </Button>
            <p className="field-help">
              {t('填入示例会替换上方内容，保存工具后才生效。')}
            </p>
          </section>

          <section
            className="tool-editor-section"
            aria-labelledby="tool-billing-heading"
          >
            <h3 id="tool-billing-heading">{t('额度与结算规则')}</h3>
            <Field>
              <FieldLabel htmlFor="tool-cost">{t('每次调用额度')}</FieldLabel>
              <Input
                id="tool-cost"
                type="number"
                min="0"
                max="1000000000000"
                step="1"
                required
                {...form.register('units_per_call')}
              />
              <p className="field-help">
                {t(
                  '填 0 表示免费；只能填写非负整数。上游服务暴露多个工具时，每次工具调用按此额度计费。',
                )}
              </p>
            </Field>
            {kind === 'builtin' ? (
              <p className="field-help">
                {t('内置工具使用协议结果结算，无需额外业务规则。')}
              </p>
            ) : (
              <SettlementEditor
                value={rule}
                onChange={setRule}
                disabled={save.isPending}
              />
            )}
          </section>
        </fieldset>
        <div className="tool-editor-footer">
          {validation && (
            <p className="field-error" role="alert">
              {t(validation)}
            </p>
          )}
          <ErrorNotice error={save.error} />
          <div className="action-row">
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? t('正在保存…') : t('保存工具')}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={close}
              disabled={save.isPending}
            >
              {t('取消')}
            </Button>
          </div>
        </div>
      </form>
    </SidePanel>
  );
}
