import { useQuery } from '@tanstack/react-query';
import { ArrowRight, Cloud, Plug } from 'lucide-react';
import { api, clientLabels } from './api';
import { Button } from './components/ui/button';

export function GatewayEquipment({
  native,
  search,
  kind,
  client,
  onManage,
}: {
  native: boolean;
  search: string;
  kind: string;
  client: string;
  onManage: () => void;
}) {
  const status = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    enabled: native,
    retry: false,
  });
  if (kind === 'skill') return null;
  const account = status.isError ? undefined : status.data?.account;
  const targets =
    status.data?.clients.filter((target) => target.configured) ?? [];
  const tools = account?.tools ?? [];
  const query = search.trim().toLowerCase();
  const filtered = tools.filter((tool) => tool.toLowerCase().includes(query));
  if (
    account &&
    client !== 'all' &&
    !targets.some((target) => target.client === client)
  )
    return null;
  if (account && query && filtered.length === 0) return null;
  return (
    <section className="gateway-equipment" aria-labelledby="gateway-title">
      <div className="gateway-heading">
        <Cloud size={24} aria-hidden="true" />
        <div>
          <h2 id="gateway-title">PluginPocket MCP 网关</h2>
          <p>本地通过 bridge 接入，工具在服务端运行。</p>
        </div>
        <span className="type-label">服务端托管</span>
      </div>
      {!native ? (
        <p className="muted small">在桌面应用中查看真实可用工具与接入状态。</p>
      ) : status.isPending ? (
        <p role="status" className="muted">
          正在读取网关工具…
        </p>
      ) : !account ? (
        <div className="gateway-unavailable">
          <p role="alert">网关目录暂不可用，请在概览登录或重试读取。</p>
          <Button
            variant="outline"
            onClick={() => void status.refetch()}
            disabled={status.isFetching}
          >
            重试读取网关
          </Button>
        </div>
      ) : (
        <>
          <div className="gateway-connection">
            <Plug size={16} aria-hidden="true" />
            <span>
              {targets.length
                ? `已接入 ${targets.map((target) => clientLabels[target.client]).join('、')}`
                : '尚未接入客户端，请在概览完成配置'}
            </span>
            <span className="muted">{tools.length} 个可用工具</span>
          </div>
          {filtered.length > 0 ? (
            <ul className="gateway-tools" aria-label="可用网关工具">
              {filtered.map((tool) => (
                <li key={tool}>{tool}</li>
              ))}
            </ul>
          ) : (
            <p className="muted small">
              当前账号暂无可用网关工具，请联系服务管理员。
            </p>
          )}
        </>
      )}
      <div className="gateway-footer">
        <span>无需本机安装；实际调用受服务端权限与额度限制。</span>
        <Button variant="ghost" onClick={onManage}>
          管理网关接入
          <ArrowRight aria-hidden="true" />
        </Button>
      </div>
    </section>
  );
}
