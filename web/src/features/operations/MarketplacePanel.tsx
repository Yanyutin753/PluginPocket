import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { ApiError, number, request } from '../account/api';
import { ErrorNotice, Loading } from '../account/shared';
import { type MarketplaceItem, marketplaceItemSchema } from './api';

function InstallEditor({
  item,
  close,
}: {
  item: MarketplaceItem;
  close: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [key, setKey] = useState(item.slug);
  const [units, setUnits] = useState('1');
  const [config, setConfig] = useState(
    item.endpoint ? JSON.stringify({ url: item.endpoint }) : '',
  );
  const install = useMutation({
    gcTime: 0,
    mutationFn: async () => {
      let parsed: unknown;
      try {
        parsed = config.trim() ? JSON.parse(config) : undefined;
      } catch {
        throw new ApiError(400, 'invalid_json');
      }
      return request(
        '/admin/marketplace/install',
        z.object({ item: marketplaceItemSchema, tool_id: z.number().int() }),
        {
          method: 'POST',
          body: JSON.stringify({
            slug: item.slug,
            transport: 'http',
            key,
            units_per_call: Number(units),
            ...(parsed !== undefined ? { config: parsed } : {}),
          }),
        },
      );
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
      void client.invalidateQueries({ queryKey: ['/admin/tools'] });
      void client.invalidateQueries({ queryKey: ['/tools'] });
      close();
    },
  });
  return (
    <section
      className="editor-panel"
      aria-label={t('安装 {value1}', { value1: item.name })}
    >
      <h2>
        {t('安装')} {item.name}
      </h2>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          install.mutate();
        }}
      >
        <FieldGroup>
          {item.transport === 'unknown' && (
            <p>{t('市场只收录 HTTP 服务；同步项需填写其实际提供的端点。')}</p>
          )}
          <Field>
            <FieldLabel htmlFor="market-key">{t('工具标识')}</FieldLabel>
            <Input
              id="market-key"
              required
              value={key}
              onChange={(event) => setKey(event.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="market-units">{t('每次调用额度')}</FieldLabel>
            <Input
              id="market-units"
              type="number"
              min="0"
              step="1"
              required
              value={units}
              onChange={(event) => setUnits(event.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="market-config">
              {t('连接配置（JSON）')}
            </FieldLabel>
            <textarea
              id="market-config"
              rows={5}
              value={config}
              onChange={(event) => setConfig(event.target.value)}
              aria-describedby="market-config-help"
            />
            <p id="market-config-help">
              {t(
                '填写 url 和可选 headers。服务地址必须符合服务器允许的网络范围。',
              )}
            </p>
          </Field>
          <ErrorNotice error={install.error} />
          <div className="action-row">
            <Button type="submit" disabled={install.isPending}>
              {install.isPending && (
                <span className="button-spinner" aria-hidden="true" />
              )}
              {install.isPending ? t('正在安装…') : t('安装到工具池')}
            </Button>
            <Button
              variant="outline"
              type="button"
              onClick={close}
              disabled={install.isPending}
            >
              {t('取消')}
            </Button>
          </div>
        </FieldGroup>
      </form>
    </section>
  );
}

function kindLabel(kind: MarketplaceItem['kind'], t: (key: string) => string) {
  if (kind === 'skill') return t('技能');
  if (kind === 'bundle') return t('装备组');
  return 'MCP';
}

function transportLabel(
  transport: MarketplaceItem['transport'],
  t: (key: string) => string,
) {
  if (transport === 'http') return t('HTTP 服务');
  if (transport === 'stdio') return t('本地进程');
  return t('需指定连接方式');
}

export default function MarketplacePanel() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const market = useQuery({
    queryKey: ['/admin/marketplace'],
    queryFn: () =>
      request(
        '/admin/marketplace',
        z.object({ items: z.array(marketplaceItemSchema) }),
      ),
  });
  const [installing, setInstalling] = useState<MarketplaceItem | null>(null);
  const sync = useMutation({
    mutationFn: () =>
      request(
        '/admin/marketplace/sync',
        z.object({ synced: z.number().int() }),
        {
          method: 'POST',
          body: '{}',
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
    },
  });
  const uninstall = useMutation({
    mutationFn: (slug: string) =>
      request(
        '/admin/marketplace/uninstall',
        z.object({ item: marketplaceItemSchema }),
        { method: 'POST', body: JSON.stringify({ slug }) },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
      void client.invalidateQueries({ queryKey: ['/admin/tools'] });
      void client.invalidateQueries({ queryKey: ['/tools'] });
    },
  });
  return (
    <section className="market-panel" aria-label={t('插件市场')}>
      <div className="panel-heading">
        <h2>{t('插件市场')}</h2>
        <Button
          variant="outline"
          disabled={sync.isPending}
          onClick={() => sync.mutate()}
        >
          {sync.isPending && (
            <span className="button-spinner" aria-hidden="true" />
          )}
          {sync.isPending ? t('正在同步…') : t('同步 GitHub')}
        </Button>
      </div>
      {sync.isSuccess && (
        <p role="status">
          {t('同步了 {value1} 个插件', {
            value1: number(sync.data.synced, locale),
          })}
        </p>
      )}
      <ErrorNotice error={market.error} retry={() => void market.refetch()} />
      <ErrorNotice error={sync.error} />
      <ErrorNotice error={uninstall.error} />
      {market.isPending && <Loading />}
      {market.isSuccess && !market.data.items.length && (
        <p className="empty-state">{t('暂时没有市场条目。')}</p>
      )}
      <ul className="record-list market-records">
        {market.data?.items.map((item) => (
          <li key={item.id}>
            <div className="record-main">
              <h3>{item.name}</h3>
              <p>{item.description}</p>
              <p>
                {kindLabel(item.kind, t)} ·{' '}
                {item.source === 'curated' ? t('精选') : 'GitHub'} ·{' '}
                {transportLabel(item.transport, t)}
                {item.source === 'github'
                  ? ` · ${t('{credits} 星', { credits: number(item.stars, locale) })}`
                  : ''}
                {item.repo_url ? (
                  <>
                    {' · '}
                    <a href={item.repo_url} rel="noreferrer" target="_blank">
                      {t('仓库')}
                    </a>
                  </>
                ) : (
                  ''
                )}
              </p>
            </div>
            <div className="action-row">
              {item.installed ? (
                <>
                  <span>{t('已安装')}</span>
                  <Button
                    variant="outline"
                    disabled={uninstall.isPending}
                    onClick={() => uninstall.mutate(item.slug)}
                  >
                    {t('卸载')}
                  </Button>
                </>
              ) : item.kind === 'mcp' ? (
                <Button
                  aria-label={t('安装 {value1}', { value1: item.name })}
                  disabled={installing !== null}
                  onClick={() => setInstalling(item)}
                >
                  {t('安装')}
                </Button>
              ) : (
                <span className="hint-text">
                  {t('用 CLI 安装：loadout install {value1}', {
                    value1: item.slug,
                  })}
                </span>
              )}
            </div>
          </li>
        ))}
      </ul>
      {installing && (
        <InstallEditor item={installing} close={() => setInstalling(null)} />
      )}
    </section>
  );
}
