import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { ApiError, number, request } from '../account/api';
import { ErrorNotice, Loading } from '../account/shared';
import { type MarketplaceItem, marketplaceItemSchema } from './api';
import {
  MarketplaceEditor,
  RecommendedSkills,
  recommendations,
} from './MarketplaceEditor';
import './MarketplacePanel.css';

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
    mutationKey: ['marketplace'],
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
    <SidePanel
      title={t('安装 {value1}', { value1: item.name })}
      onClose={close}
      locked={install.isPending}
    >
      <section
        className="editor-panel"
        aria-label={t('安装 {value1}', { value1: item.name })}
      >
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
              <FieldLabel htmlFor="market-units">
                {t('每次调用额度')}
              </FieldLabel>
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
    </SidePanel>
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
  if (transport === 'gateway') return t('网关托管');
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
  const [editing, setEditing] = useState<{
    kind: 'skill' | 'bundle';
    item?: MarketplaceItem;
  } | null>(null);
  const [recommended, setRecommended] = useState(false);
  const [saved, setSaved] = useState(false);
  const [kind, setKind] = useState('all');
  const [search, setSearch] = useState('');
  const items = market.data?.items ?? [];
  const visible = items.filter(
    (item) =>
      (kind === 'all' || item.kind === kind) &&
      `${item.name} ${item.slug} ${item.description}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const sync = useMutation({
    mutationKey: ['marketplace'],
    mutationFn: async () => {
      const result = { synced: 0, skills: 0, failed: [] as string[] };
      try {
        const data = await request(
          '/admin/marketplace/sync',
          z.object({ synced: z.number().int() }),
          { method: 'POST', body: '{}' },
        );
        result.synced = data.synced;
      } catch {
        result.failed.push('MCP');
      }
      const skills = new Map(
        recommendations
          .filter((entry) => !items.some((item) => item.slug === entry.slug))
          .map((entry) => [entry.slug, entry]),
      );
      for (const item of items) {
        if (
          item.kind === 'skill' &&
          item.spec?.source === 'github' &&
          item.spec.repo &&
          item.spec.path
        ) {
          skills.set(item.slug, {
            slug: item.slug,
            name: item.name,
            description: item.description,
            repo: item.spec.repo,
            path: item.spec.path,
          });
        }
      }
      for (const entry of skills.values()) {
        try {
          await request(
            '/admin/marketplace/skills',
            z.object({ item: marketplaceItemSchema }),
            {
              method: 'POST',
              body: JSON.stringify({ ...entry, source: 'github' }),
            },
          );
          result.skills++;
        } catch {
          result.failed.push(entry.name);
        }
      }
      return result;
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
      void client.invalidateQueries({ queryKey: ['public-plugins'] });
      void client.invalidateQueries({ queryKey: ['skill-editor-files'] });
    },
  });
  const uninstall = useMutation({
    mutationKey: ['marketplace'],
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
          disabled={sync.isPending || !market.isSuccess}
          onClick={() => sync.mutate()}
        >
          {sync.isPending && (
            <span className="button-spinner" aria-hidden="true" />
          )}
          {sync.isPending ? t('正在同步…') : t('同步 GitHub')}
        </Button>
      </div>
      <div className="action-row market-actions">
        <Button
          onClick={() => {
            setSaved(false);
            setEditing({ kind: 'skill' });
          }}
        >
          {t('创建技能')}
        </Button>
        <Button
          variant="outline"
          onClick={() => {
            setSaved(false);
            setEditing({ kind: 'bundle' });
          }}
        >
          {t('创建装备组')}
        </Button>
        <Button variant="outline" onClick={() => setRecommended(true)}>
          {t('精选 GitHub 技能')}
        </Button>
      </div>
      <fieldset className="market-filters" aria-label={t('类型筛选')}>
        {['all', 'mcp', 'skill', 'bundle'].map((value) => (
          <Button
            key={value}
            variant={kind === value ? 'secondary' : 'ghost'}
            aria-pressed={kind === value}
            onClick={() => setKind(value)}
          >
            {value === 'all'
              ? t('全部')
              : kindLabel(value as MarketplaceItem['kind'], t)}{' '}
            (
            {value === 'all'
              ? items.length
              : items.filter((item) => item.kind === value).length}
            )
          </Button>
        ))}
      </fieldset>
      <Input
        aria-label={t('搜索市场')}
        placeholder={t('搜索名称、标识或描述')}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      {saved && <p role="status">{t('已保存')}</p>}
      {sync.isSuccess && (
        <p role="status">
          {t('同步了 {mcp} 个 MCP、{skills} 个技能；失败 {failed} 项', {
            mcp: number(sync.data.synced, locale),
            skills: number(sync.data.skills, locale),
            failed: number(sync.data.failed.length, locale),
          })}
        </p>
      )}
      <ErrorNotice error={market.error} retry={() => void market.refetch()} />
      {sync.isSuccess && sync.data.failed.length > 0 && (
        <p role="alert">
          {t('同步失败，请重试')} · {sync.data.failed.join('、')}
        </p>
      )}
      <ErrorNotice error={sync.error} />
      <ErrorNotice error={uninstall.error} />
      {market.isPending && <Loading />}
      {market.isSuccess && !visible.length && (
        <p className="empty-state">{t('暂时没有市场条目。')}</p>
      )}
      <ul className="record-list market-records">
        {visible.map((item) => (
          <li key={item.id}>
            <div className="record-main">
              <div className="market-record-title">
                <h3>{item.name}</h3>
                <span>{kindLabel(item.kind, t)}</span>
              </div>
              {item.description && <p>{item.description}</p>}
              <div className="market-record-meta">
                <span>{item.slug}</span>
                {item.kind === 'mcp' && (
                  <span>{transportLabel(item.transport, t)}</span>
                )}
                {item.source === 'curated' && <span>{t('精选')}</span>}
                {(item.source === 'github' ||
                  item.spec?.source === 'github') && <span>GitHub</span>}
                {item.source === 'github' && (
                  <span>
                    {t('{credits} 星', { credits: number(item.stars, locale) })}
                  </span>
                )}
                {item.repo_url && (
                  <a href={item.repo_url} rel="noreferrer" target="_blank">
                    {t('仓库')}
                  </a>
                )}
                {item.kind === 'mcp' && item.installed && (
                  <span>{t('已加入工具池')}</span>
                )}
              </div>
            </div>
            <div className="action-row market-record-actions">
              {item.kind !== 'mcp' && (
                <Button
                  variant="ghost"
                  aria-label={t('编辑 {value1}', { value1: item.name })}
                  onClick={() => {
                    setSaved(false);
                    setEditing({
                      kind: item.kind === 'skill' ? 'skill' : 'bundle',
                      item,
                    });
                  }}
                >
                  {t('编辑')}
                </Button>
              )}
              {item.kind === 'mcp' &&
                (item.installed ? (
                  <Button
                    variant="ghost"
                    aria-label={t('移出工具池 {value1}', { value1: item.name })}
                    disabled={uninstall.isPending}
                    onClick={() => uninstall.mutate(item.slug)}
                  >
                    {t('移出工具池')}
                  </Button>
                ) : (
                  <Button
                    variant="outline"
                    aria-label={t('安装到工具池 {value1}', {
                      value1: item.name,
                    })}
                    disabled={installing !== null}
                    onClick={() => setInstalling(item)}
                  >
                    {t('安装到工具池')}
                  </Button>
                ))}
            </div>
          </li>
        ))}
      </ul>
      {installing && (
        <InstallEditor item={installing} close={() => setInstalling(null)} />
      )}
      {editing && (
        <MarketplaceEditor
          {...editing}
          items={items}
          close={() => setEditing(null)}
          saved={() => setSaved(true)}
        />
      )}
      {recommended && <RecommendedSkills close={() => setRecommended(false)} />}
    </section>
  );
}
