import {
  useIsMutating,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';
import { number, request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
import { type Tool, toolSchema } from './api';
import MarketplacePanel from './MarketplacePanel';
import { ruleDraft } from './SettlementEditor';
import ToolEditor from './ToolEditor';
import { ToolIcon } from './ToolIcon';
import ToolMetadataPanel from './ToolMetadataPanel';

const kindLabels = {
  builtin: '内置工具',
  http: 'HTTP 服务',
  stdio: '本地进程',
};
const ruleLabels: Record<string, string> = {
  default: '协议结算',
  path: '业务码匹配',
  pattern: '文本匹配',
  script: '脚本判定',
  json: '自定义规则',
};

export default function ToolsPage({ admin = false }: { admin?: boolean }) {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const path = admin ? '/admin/tools' : '/tools';
  const tools = usePagedList(path, toolSchema);
  const [editing, setEditing] = useState<Tool | null | undefined>();
  const [expanded, setExpanded] = useState<Tool | null>(null);
  const marketPending = useIsMutating({ mutationKey: ['marketplace'] }) > 0;
  const metadataPending =
    useIsMutating({ mutationKey: ['tool-metadata', expanded?.id] }) > 0;
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
      {admin && (
        <div className="action-row">
          <Button onClick={() => setEditing(null)}>{t('添加工具')}</Button>
          <SidePanel
            title={t('插件市场')}
            trigger={t('插件市场')}
            locked={marketPending}
          >
            <MarketplacePanel />
          </SidePanel>
        </div>
      )}
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
        className="record-list tool-management-list"
        ref={tools.listRef}
        tabIndex={-1}
      >
        {tools.items.map((item) => (
          <li key={item.id}>
            <ToolIcon icon={item.icon} name={item.name} />
            <div className="record-main">
              <div className="tool-name-row">
                <h2>{item.name}</h2>
                <span
                  className="status-badge"
                  data-tone={item.enabled ? 'success' : 'neutral'}
                >
                  {item.enabled ? t('可用') : t('已停用')}
                </span>
              </div>
              <p>{item.description}</p>
              <div className="tool-identifiers">
                <code>{item.key}</code>
                <span>{t(kindLabels[item.kind])}</span>
                <span>{t(ruleLabels[ruleDraft(item.settlement).mode])}</span>
                {item.allowed_roles?.length ? (
                  <span>
                    {t('限 {value1}', {
                      value1: item.allowed_roles.join(' / '),
                    })}
                  </span>
                ) : null}
              </div>
            </div>
            <div className="tool-list-cost">
              <span className="tool-cost">
                {t('{credits} 额度 / 次', {
                  credits: number(item.units_per_call, locale),
                })}
              </span>
            </div>
            <div className="tool-actions">
              <Button
                variant="ghost"
                aria-haspopup="dialog"
                aria-label={t('查看参数 {value1}', { value1: item.name })}
                onClick={() => setExpanded(item)}
              >
                {t('查看参数')}
              </Button>
              {admin && (
                <>
                  <Button
                    variant="secondary"
                    aria-label={t('编辑 {value1}', { value1: item.name })}
                    disabled={editing !== undefined}
                    onClick={() => setEditing(item)}
                  >
                    {t('编辑')}
                  </Button>
                  <Button
                    variant="ghost"
                    aria-label={`${item.enabled ? t('停用') : t('启用')} ${item.name}`}
                    disabled={toggle.isPending || editing !== undefined}
                    onClick={() => toggle.mutate(item)}
                  >
                    {item.enabled ? t('停用') : t('启用')}
                  </Button>
                </>
              )}
            </div>
          </li>
        ))}
      </ul>
      <Pagination label={t('工具')} {...tools.pagination} />
      {expanded && (
        <SidePanel
          title={expanded.name}
          onClose={() => setExpanded(null)}
          locked={metadataPending}
        >
          <div className="tool-detail-intro">
            <ToolIcon icon={expanded.icon} name={expanded.name} />
            <p>{expanded.description}</p>
          </div>
          <div className="tool-summary">
            <code>{expanded.key}</code>
            <span>{t(kindLabels[expanded.kind])}</span>
            <span>{t(ruleLabels[ruleDraft(expanded.settlement).mode])}</span>
            <span>
              {t('{credits} 额度 / 次', {
                credits: number(expanded.units_per_call, locale),
              })}
            </span>
            <span
              className="status-badge"
              data-tone={expanded.enabled ? 'success' : 'neutral'}
            >
              {expanded.enabled ? t('可用') : t('已停用')}
            </span>
          </div>
          {admin && expanded.configured && <p>{t('已配置连接')}</p>}
          <section>
            <h3>{t('查看参数')}</h3>
            <pre className="schema-code">
              <code>{JSON.stringify(expanded.input_schema, null, 2)}</code>
            </pre>
          </section>
          {admin && expanded.kind !== 'builtin' && (
            <ToolMetadataPanel tool={expanded} />
          )}
        </SidePanel>
      )}
    </>
  );
}
