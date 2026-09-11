import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Wrench } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { ApiError, number, request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
import { type Tool, toolSchema } from './api';
import MarketplacePanel from './MarketplacePanel';
import ToolMetadataPanel from './ToolMetadataPanel';

function ToolEditor({ item, close }: { item: Tool | null; close: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
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
      settlement:
        item?.settlement && Object.keys(item.settlement).length
          ? JSON.stringify(item.settlement, null, 2)
          : '',
    },
  });
  const kind = form.watch('kind');
  const save = useMutation({
    gcTime: 0,
    mutationFn: async () => {
      const value = form.getValues();
      let input_schema: unknown;
      let config: unknown;
      let settlement: unknown;
      try {
        input_schema = JSON.parse(value.input_schema);
        config = value.config.trim() ? JSON.parse(value.config) : undefined;
        settlement = value.settlement.trim()
          ? JSON.parse(value.settlement)
          : undefined;
      } catch {
        throw new ApiError(400, 'invalid_json');
      }
      const object = z.record(z.string(), z.unknown());
      if (
        !object.safeParse(input_schema).success ||
        (config !== undefined && !object.safeParse(config).success) ||
        (settlement !== undefined && !object.safeParse(settlement).success)
      )
        throw new ApiError(400, 'invalid_json');
      // 前端预检：脚本语法用 new Function 只编译不执行，坏脚本不出本机。
      const script =
        settlement && typeof settlement === 'object'
          ? (settlement as { script?: unknown }).script
          : undefined;
      if (typeof script === 'string' && script.length > 0) {
        try {
          // eslint-disable-next-line no-new-func
          new Function('result', script);
        } catch {
          throw new ApiError(400, 'invalid_settlement_script');
        }
      }
      await request(
        `/admin/tools${item ? `/${item.id}` : ''}`,
        z.object({ item: toolSchema }),
        {
          method: item ? 'PATCH' : 'POST',
          body: JSON.stringify({
            key: value.key,
            name: value.name,
            description: value.description,
            kind: value.kind,
            enabled: item?.enabled ?? true,
            units_per_call: Number(value.units_per_call),
            input_schema,
            ...(config !== undefined ? { config } : {}),
            ...(settlement !== undefined ? { settlement } : {}),
          }),
        },
      );
    },
    onSuccess: () => {
      form.reset();
      void client.invalidateQueries({ queryKey: ['/admin/tools'] });
      void client.invalidateQueries({ queryKey: ['/tools'] });
      close();
    },
  });
  return (
    <section className="editor-panel">
      <h2>{item ? t('编辑工具') : t('添加工具')}</h2>
      <form onSubmit={form.handleSubmit(() => save.mutate())}>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="tool-key">{t('工具标识')}</FieldLabel>
            <Input
              id="tool-key"
              required
              {...form.register('key')}
              placeholder={t('例如：echo')}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="tool-name">{t('显示名称')}</FieldLabel>
            <Input id="tool-name" required {...form.register('name')} />
          </Field>
          <Field>
            <FieldLabel htmlFor="tool-description">{t('工具说明')}</FieldLabel>
            <Input id="tool-description" {...form.register('description')} />
          </Field>
          <Field>
            <FieldLabel htmlFor="tool-kind">{t('连接方式')}</FieldLabel>
            <Select
              id="tool-kind"
              value={kind}
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
          </Field>
          <Field>
            <FieldLabel htmlFor="tool-schema">
              {t('参数 Schema（JSON）')}
            </FieldLabel>
            <textarea
              id="tool-schema"
              rows={5}
              required
              {...form.register('input_schema')}
            />
          </Field>
          {kind !== 'builtin' && (
            <Field>
              <FieldLabel htmlFor="tool-settlement">
                {t('结算策略（JSON，可选）')}
              </FieldLabel>
              <textarea
                id="tool-settlement"
                rows={3}
                {...form.register('settlement')}
                aria-describedby="settlement-help"
              />
              <p id="settlement-help">
                {t(
                  '留空按协议结果扣费；{"content":{"path":"code","equals":0}} 按业务码判定（equals 可为数组），{"content":{"pattern":"^OK"}} 按文本匹配，或 {"script":"return JSON.parse(result.text).code === 0"} 写 JS（沙箱，入参 result={isError,text}，返回真值才扣费），未通过即退款。',
                )}
              </p>
            </Field>
          )}
          {kind !== 'builtin' && (
            <Field>
              <FieldLabel htmlFor="tool-config">
                {t('连接配置（JSON）')}
              </FieldLabel>
              <textarea
                id="tool-config"
                rows={5}
                {...form.register('config')}
                aria-describedby="config-help"
              />
              <p id="config-help">
                {kind === 'http'
                  ? t(
                      '填写 url 和可选 headers。服务地址必须符合服务器允许的网络范围。',
                    )
                  : t(
                      '填写已获服务器允许的 command 别名，以及可选 args、env。',
                    )}
                {item
                  ? t(' 留空保留原配置；已保存的凭证不会回传。')
                  : t(' 凭证仅用于运营方预设池。')}
              </p>
            </Field>
          )}
          <ErrorNotice error={save.error} />
          <div className="action-row">
            <Button type="submit" disabled={save.isPending}>
              {save.isPending && (
                <span className="button-spinner" aria-hidden="true" />
              )}
              {save.isPending ? t('正在保存…') : t('保存工具')}
            </Button>
            <Button
              variant="outline"
              type="button"
              onClick={close}
              disabled={save.isPending}
            >
              {t('取消')}
            </Button>
          </div>
        </FieldGroup>
      </form>
    </section>
  );
}
export default function ToolsPage({ admin = false }: { admin?: boolean }) {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const path = admin ? '/admin/tools' : '/tools';
  const tools = usePagedList(path, toolSchema);
  const [editing, setEditing] = useState<Tool | null | undefined>();
  const [expanded, setExpanded] = useState<number | null>(null);
  const toggle = useMutation({
    mutationFn: (item: Tool) =>
      request(`/admin/tools/${item.id}`, z.object({ item: toolSchema }), {
        method: 'PATCH',
        body: JSON.stringify({
          key: item.key,
          name: item.name,
          description: item.description,
          kind: item.kind,
          units_per_call: item.units_per_call,
          input_schema: item.input_schema,
          enabled: !item.enabled,
        }),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: [path] });
      void client.invalidateQueries({ queryKey: ['/tools'] });
    },
  });
  return (
    <>
      <Heading
        artwork={admin ? 'admin-tools' : 'tool-catalog'}
        title={admin ? t('工具管理') : t('工具目录')}
      >
        {admin
          ? t('维护预设工具、连接配置和每次调用的额度。')
          : t('查看当前可用的工具与调用成本。')}
      </Heading>
      {admin && editing === undefined && (
        <Button onClick={() => setEditing(null)}>{t('添加工具')}</Button>
      )}
      {admin && <MarketplacePanel />}
      {editing !== undefined && (
        <ToolEditor item={editing} close={() => setEditing(undefined)} />
      )}
      <ErrorNotice error={tools.error} retry={() => void tools.retry()} />
      <ErrorNotice error={toggle.error} />
      {tools.isPending && <Loading />}
      {tools.isSuccess && !tools.items.length && (
        <p className="empty-state">{t('暂时没有工具。')}</p>
      )}
      <ul
        className="record-list tool-records"
        ref={tools.listRef}
        tabIndex={-1}
      >
        {tools.items.map((item) => (
          <li key={item.id}>
            <span className="tool-icon">
              <Wrench aria-hidden="true" />
            </span>
            <div className="record-main">
              <h2>{item.name}</h2>
              <p>{item.description}</p>
              <p>
                <code>{item.key}</code> ·{' '}
                {t('{credits} 额度 / 次', {
                  credits: number(item.units_per_call, locale),
                })}{' '}
                · <span>{item.enabled ? t('可用') : t('已停用')}</span>
                {admin && item.configured ? t(' · 已配置连接') : ''}
              </p>
              <Button
                variant="ghost"
                aria-expanded={expanded === item.id}
                aria-label={t('查看参数 {value1}', { value1: item.name })}
                onClick={() =>
                  setExpanded(expanded === item.id ? null : item.id)
                }
              >
                {t('查看参数')}
              </Button>
              {expanded === item.id && (
                <>
                  <pre className="schema-code">
                    <code>{JSON.stringify(item.input_schema, null, 2)}</code>
                  </pre>
                  {admin ? <ToolMetadataPanel tool={item} /> : null}
                </>
              )}
            </div>
            {admin && (
              <div className="action-row">
                <Button
                  variant="outline"
                  aria-label={t('编辑 {value1}', { value1: item.name })}
                  disabled={editing !== undefined}
                  onClick={() => setEditing(item)}
                >
                  {t('编辑')}
                </Button>
                <Button
                  variant="outline"
                  aria-label={`${item.enabled ? t('停用') : t('启用')} ${item.name}`}
                  disabled={toggle.isPending || editing !== undefined}
                  onClick={() => toggle.mutate(item)}
                >
                  {item.enabled ? t('停用') : t('启用')}
                </Button>
              </div>
            )}
          </li>
        ))}
      </ul>
      <Pagination label={t('工具')} {...tools.pagination} />
    </>
  );
}
