import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { ApiError, request } from '../account/api';
import { ErrorNotice, Loading } from '../account/shared';
import {
  type MetadataOverride,
  metadataOverrideSchema,
  type Tool,
  type UpstreamTool,
  upstreamToolSchema,
} from './api';

function OverrideEditor({
  tool,
  remote,
  override,
  close,
}: {
  tool: Tool;
  remote: UpstreamTool;
  override?: MetadataOverride;
  close: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [description, setDescription] = useState(override?.description ?? '');
  const [schema, setSchema] = useState(
    JSON.stringify(
      override?.input_schema ?? remote.input_schema ?? { type: 'object' },
      null,
      2,
    ),
  );
  const save = useMutation({
    gcTime: 0,
    mutationFn: async () => {
      let parsed: unknown;
      try {
        parsed = schema.trim() ? JSON.parse(schema) : undefined;
      } catch {
        throw new ApiError(400, 'invalid_json');
      }
      return request(
        `/admin/tools/${tool.id}/metadata`,
        z.object({ item: metadataOverrideSchema }),
        {
          method: 'PUT',
          body: JSON.stringify({
            remote_name: remote.name,
            description,
            ...(parsed !== undefined ? { input_schema: parsed } : {}),
          }),
        },
      );
    },
    onSuccess: () => {
      void client.invalidateQueries({
        queryKey: ['/admin/tools', tool.id, 'upstream'],
      });
      close();
    },
  });
  return (
    <form
      aria-label={t('覆盖 {value1} 元数据', { value1: remote.name })}
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate();
      }}
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor={`override-description-${remote.name}`}>
            {t('覆盖描述')}
          </FieldLabel>
          <Input
            id={`override-description-${remote.name}`}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`override-schema-${remote.name}`}>
            {t('参数 Schema（JSON）')}
          </FieldLabel>
          <textarea
            id={`override-schema-${remote.name}`}
            rows={6}
            value={schema}
            onChange={(event) => setSchema(event.target.value)}
          />
        </Field>
        <ErrorNotice error={save.error} />
        <div className="action-row">
          <Button type="submit" disabled={save.isPending}>
            {save.isPending && (
              <span className="button-spinner" aria-hidden="true" />
            )}
            {save.isPending ? t('正在保存…') : t('保存覆盖')}
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
  );
}

export default function ToolMetadataPanel({ tool }: { tool: Tool }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [editing, setEditing] = useState<string | null>(null);
  const upstream = useQuery({
    queryKey: ['/admin/tools', tool.id, 'upstream'],
    queryFn: () =>
      request(
        `/admin/tools/${tool.id}/upstream`,
        z.object({
          tools: z.array(upstreamToolSchema),
          overrides: z.array(metadataOverrideSchema),
        }),
      ),
  });
  const remove = useMutation({
    mutationFn: (name: string) =>
      request(
        `/admin/tools/${tool.id}/metadata/${encodeURIComponent(name)}`,
        z.unknown(),
        {
          method: 'DELETE',
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({
        queryKey: ['/admin/tools', tool.id, 'upstream'],
      });
    },
  });
  if (tool.kind === 'builtin') return null;
  return (
    <section className="metadata-panel" aria-label={t('上游工具与描述')}>
      <h3>{t('上游工具与描述')}</h3>
      <ErrorNotice
        error={upstream.error}
        retry={() => void upstream.refetch()}
      />
      <ErrorNotice error={remove.error} />
      {upstream.isPending && <Loading />}
      {upstream.isSuccess && !upstream.data.tools.length && (
        <p className="empty-state">{t('上游暂未暴露工具。')}</p>
      )}
      <ul className="record-list remote-records">
        {upstream.data?.tools.map((remote) => {
          const override = upstream.data.overrides.find(
            (entry) => entry.remote_name === remote.name,
          );
          const current = override?.description || remote.description;
          return (
            <li key={remote.name}>
              <div className="record-main">
                <h4>
                  <code>{remote.name}</code>
                  {override ? t(' · 已覆盖') : ''}
                </h4>
                <p>{current}</p>
                {editing === remote.name ? (
                  <OverrideEditor
                    tool={tool}
                    remote={remote}
                    override={override}
                    close={() => setEditing(null)}
                  />
                ) : null}
              </div>
              <div className="action-row">
                <Button
                  variant="ghost"
                  aria-label={t('覆盖描述 {value1}', { value1: remote.name })}
                  onClick={() =>
                    setEditing(editing === remote.name ? null : remote.name)
                  }
                >
                  {t('覆盖描述')}
                </Button>
                {override ? (
                  <Button
                    variant="ghost"
                    aria-label={t('清除覆盖 {value1}', { value1: remote.name })}
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(remote.name)}
                  >
                    {t('清除覆盖')}
                  </Button>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
