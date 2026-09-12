import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowDownToLine,
  Package,
  Plug,
  RefreshCw,
  Search,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { useState } from 'react';
import { api, clientLabels, type InstalledItem } from './api';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { GatewayEquipment } from './GatewayEquipment';

export function EquipmentPage({
  native,
  onManageConnection,
}: {
  native: boolean;
  onManageConnection: () => void;
}) {
  const cache = useQueryClient();
  const items = useQuery({
    queryKey: ['installed'],
    queryFn: api.installed,
    enabled: native,
  });
  const [search, setSearch] = useState('');
  const [kind, setKind] = useState('all');
  const [client, setClient] = useState('all');
  const [confirmation, setConfirmation] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const operation = useMutation({
    mutationFn: ({ item, remove }: { item: InstalledItem; remove: boolean }) =>
      remove ? api.uninstallInstalled(item) : api.updateInstalled(item),
    onMutate: () => {
      setError('');
      setMessage('');
    },
    onSuccess: (_data, { item, remove }) => {
      setConfirmation(null);
      setMessage(`${remove ? '已卸载' : '已更新'} ${item.slug}`);
    },
    onError: (_error, { remove }) =>
      setError(
        remove
          ? '卸载未完成，请检查本地文件是否有手动改动，然后重试。详情见运行日志。'
          : '更新未完成，请检查连接、凭证与文件冲突，然后重试。详情见运行日志。',
      ),
    onSettled: async () => {
      await Promise.all(
        ['installed', 'clients', 'logs'].map((key) =>
          cache.invalidateQueries({ queryKey: [key] }),
        ),
      );
    },
  });
  const all = items.isError ? [] : (items.data ?? []);
  const visible = all.filter(
    (item) =>
      (kind === 'all' || item.kind === kind) &&
      (client === 'all' || item.clients.some((target) => target === client)) &&
      item.slug.toLowerCase().includes(search.trim().toLowerCase()),
  );
  const disabled = operation.isPending || items.isFetching || items.isError;
  return (
    <div className="equipment-page">
      <div className="page-toolbar">
        <label className="search-field" htmlFor="equipment-search">
          <Search size={18} aria-hidden="true" />
          <Input
            id="equipment-search"
            type="search"
            aria-label="搜索已安装装备"
            placeholder="搜索装备名称…"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setConfirmation(null);
            }}
          />
        </label>
        <label className="filter-select">
          <span className="sr-only">安装目标</span>
          <select
            aria-label="安装目标"
            value={client}
            onChange={(event) => {
              setClient(event.target.value);
              setConfirmation(null);
            }}
          >
            <option value="all">全部客户端</option>
            {Object.entries(clientLabels).map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <Button
          variant="outline"
          onClick={() => {
            void items.refetch();
            void cache.invalidateQueries({ queryKey: ['status'] });
          }}
          disabled={!native || operation.isPending || items.isFetching}
        >
          <RefreshCw aria-hidden="true" />
          刷新装备
        </Button>
      </div>
      <div className="list-section-heading">
        <fieldset className="filter-tabs">
          <legend className="sr-only">装备类型</legend>
          {[
            { id: 'all', label: '全部装备' },
            { id: 'mcp', label: 'MCP 插件' },
            { id: 'skill', label: 'Skills' },
          ].map((tab) => (
            <button
              type="button"
              key={tab.id}
              aria-pressed={kind === tab.id}
              onClick={() => {
                setKind(tab.id);
                setConfirmation(null);
              }}
            >
              {tab.label}
            </button>
          ))}
        </fieldset>
        <span className="small muted">
          {native && items.isSuccess
            ? `${visible.length} 项本地安装`
            : '本地托管清单'}
        </span>
      </div>
      <GatewayEquipment
        native={native}
        search={search}
        kind={kind}
        client={client}
        onManage={onManageConnection}
      />
      {native && items.isPending && (
        <p role="status" className="page-loading">
          正在读取本地装备…
        </p>
      )}
      {native && items.isError && (
        <div className="page-error" role="alert">
          <p>无法读取已安装装备，请检查本地清单后重试</p>
          <Button variant="outline" onClick={() => void items.refetch()}>
            重试读取装备
          </Button>
        </div>
      )}
      {(!native || (items.isSuccess && visible.length === 0)) && (
        <div className="empty-state">
          <Package size={34} aria-hidden="true" />
          <h2>
            {!native
              ? '你的装备，在本机就位'
              : all.length
                ? '没有匹配的装备'
                : '还没有本地安装的装备'}
          </h2>
          <p>
            {!native
              ? '在桌面应用中查看已安装的 MCP 和 Skills，以及它们接入的客户端。'
              : all.length
                ? '试试其他关键词或清除筛选。'
                : '通过 PluginPocket CLI 安装后，装备会出现在这里。'}
          </p>
          <div className="empty-detail">
            <ArrowDownToLine size={16} aria-hidden="true" />
            <span>支持查看安装目标、更新与安全卸载</span>
          </div>
        </div>
      )}
      {visible.length > 0 && (
        <ul className="equipment-list">
          {visible.map((item) => (
            <li key={`${item.kind}:${item.slug}`}>
              <div className="equipment-row">
                <div
                  className={`equipment-icon ${item.kind}`}
                  aria-hidden="true"
                >
                  {item.kind === 'skill' ? (
                    <Sparkles size={22} />
                  ) : (
                    <Plug size={22} />
                  )}
                </div>
                <div className="equipment-info">
                  <div className="equipment-name">
                    <h2>{item.slug}</h2>
                    <span className="type-label">
                      {item.kind === 'skill' ? 'Skill' : 'MCP'}
                    </span>
                  </div>
                  <p className="muted small">{item.version ?? '版本未记录'}</p>
                  <div className="client-tags">
                    {item.clients.map((target) => (
                      <span key={target}>{clientLabels[target]}</span>
                    ))}
                  </div>
                </div>
                <div className="equipment-actions">
                  <Button
                    variant="outline"
                    aria-label={`更新 ${item.slug} · ${item.kind === 'skill' ? 'Skill' : 'MCP'}`}
                    onClick={() => operation.mutate({ item, remove: false })}
                    disabled={disabled}
                  >
                    <RefreshCw aria-hidden="true" />
                    {operation.isPending &&
                    operation.variables?.item.slug === item.slug &&
                    operation.variables.item.kind === item.kind &&
                    !operation.variables.remove
                      ? '更新中…'
                      : '更新'}
                  </Button>
                  <Button
                    variant="ghost"
                    aria-label={`卸载 ${item.slug} · ${item.kind === 'skill' ? 'Skill' : 'MCP'}`}
                    onClick={() => setConfirmation(`${item.kind}:${item.slug}`)}
                    disabled={disabled}
                  >
                    <Trash2 aria-hidden="true" />
                    卸载
                  </Button>
                </div>
              </div>
              {confirmation === `${item.kind}:${item.slug}` && (
                <div className="inline-confirmation">
                  <p>
                    将从{' '}
                    {item.clients
                      .map((target) => clientLabels[target])
                      .join('、')}{' '}
                    移除 {item.slug}。
                    {item.kind === 'skill'
                      ? '卸载会删除已安装的技能文件，包括你对这些文件内容的修改。请先备份需要保留的内容；发现新增文件或路径冲突时将停止。'
                      : '配置条目有手动改动时会停止并保留配置。'}
                  </p>
                  <div className="action-row">
                    <Button
                      variant="destructive"
                      disabled={disabled}
                      aria-label={`确认卸载 ${item.slug} · ${item.kind === 'skill' ? 'Skill' : 'MCP'}`}
                      onClick={() => operation.mutate({ item, remove: true })}
                    >
                      {operation.isPending ? '卸载中…' : '确认卸载'}
                    </Button>
                    <Button
                      variant="outline"
                      disabled={operation.isPending}
                      onClick={() => setConfirmation(null)}
                    >
                      取消卸载
                    </Button>
                  </div>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      <div className="feedback" aria-live="polite">
        {message && <p role="status">{message}</p>}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
      </div>
      <p className="page-footnote">
        仅显示 PluginPocket
        托管的安装记录。装备组展开为成员；网关工具由服务端管理。更新与卸载作用于该装备列出的全部安装目标。
      </p>
    </div>
  );
}
