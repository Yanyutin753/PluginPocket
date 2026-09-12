import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { api, type LedgerPage, type UsagePage } from './api';
import { Button } from './components/ui/button';
import { useI18n } from './i18n';

const statusKeys = {
  pending: '处理中',
  ok: '成功',
  error: '失败',
  denied: '已拒绝',
  recovered: '已恢复',
} as const;
const kindKeys = {
  registration: '注册赠送',
  adjustment: '额度调整',
  redemption: '兑换入账',
  team_transfer: '团队转账',
  reservation: '调用预扣',
  refund: '退款',
  recovery: '额度回收',
} as const;
const credits = (value: number) => `${value.toLocaleString('zh-CN')} credits`;
function Timestamp({ value }: { value: string }) {
  return (
    <time dateTime={value}>
      {new Date(value).toLocaleString('zh-CN', { hour12: false })}
    </time>
  );
}
export function BillingPanel({
  native,
  kind,
}: {
  native: boolean;
  kind: 'usage' | 'ledger';
}) {
  const { t } = useI18n();
  const query = useQuery<UsagePage | LedgerPage>({
    queryKey: ['billing', kind],
    queryFn: kind === 'usage' ? api.usage : api.ledger,
    enabled: native,
    retry: false,
    gcTime: 0,
  });
  const title = t(kind === 'usage' ? '用量明细' : '账变记录');
  return (
    <div
      className="log-panel billing-panel"
      aria-busy={native && query.isFetching}
    >
      <div className="log-panel-heading">
        <div>
          <strong>{title}</strong>
          <span className="small muted">
            {t('从服务端读取，不保存到本地操作日志。')}
          </span>
        </div>
        <Button
          variant="outline"
          disabled={!native || query.isFetching}
          onClick={() => void query.refetch()}
        >
          <RefreshCw aria-hidden="true" />
          {t('刷新账单')}
        </Button>
      </div>
      {!native ? (
        <p className="page-loading">
          {t('请在桌面应用中登录后查看真实账单。')}
        </p>
      ) : query.isFetching ? (
        <div className="page-loading">
          <p role="status" aria-label={t('正在读取账单')}>
            {t('正在读取账单')}…
          </p>
          <div className="skeleton-content" aria-hidden="true">
            <span className="skeleton skeleton-field" />
            <span className="skeleton skeleton-field" />
          </div>
        </div>
      ) : query.isError ? (
        <div className="page-error" role="alert">
          <p>{t('账单暂不可用，请在概览登录或重试读取。')}</p>
          <Button variant="outline" onClick={() => void query.refetch()}>
            {t('重试读取账单')}
          </Button>
        </div>
      ) : query.data && query.data.items.length === 0 ? (
        <div className="empty-state">
          <h2>{t('暂无记录')}</h2>
          <p>{t('这里显示当前账号的真实账单记录。')}</p>
        </div>
      ) : (
        query.data && (
          <>
            <section className="log-table-wrap" aria-label={title}>
              <table className="log-table billing-table">
                <caption className="sr-only">{title}</caption>
                <thead>
                  <tr>
                    {(kind === 'usage'
                      ? ['时间', '工具', '状态', '耗时', '费用']
                      : ['时间', '类型', '说明', '变动', '余额后']
                    ).map((label) => (
                      <th scope="col" key={label}>
                        {t(label)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {kind === 'usage'
                    ? (query.data as UsagePage).items.map((item) => (
                        <tr key={item.id}>
                          <td>
                            <Timestamp value={item.created_at} />
                          </td>
                          <td>{item.tool}</td>
                          <td>
                            <span className={`billing-status ${item.status}`}>
                              {t(statusKeys[item.status])}
                            </span>
                          </td>
                          <td className="numeric">
                            {item.duration_ms.toLocaleString('zh-CN')} ms
                          </td>
                          <td className="numeric">{credits(item.cost)}</td>
                        </tr>
                      ))
                    : (query.data as LedgerPage).items.map((item) => (
                        <tr key={item.id}>
                          <td>
                            <Timestamp value={item.created_at} />
                          </td>
                          <td>{t(kindKeys[item.kind])}</td>
                          <td>{item.note || '—'}</td>
                          <td
                            className={`numeric ${item.delta > 0 ? 'amount-positive' : item.delta < 0 ? 'amount-negative' : ''}`}
                          >
                            {item.delta > 0 ? '+' : item.delta < 0 ? '−' : ''}
                            {credits(Math.abs(item.delta))}
                          </td>
                          <td className="numeric">
                            {credits(item.balance_after)}
                          </td>
                        </tr>
                      ))}
                </tbody>
              </table>
            </section>
            {query.data.next_cursor && (
              <p className="page-loading small muted">
                {t('当前显示最近一页记录。')}
              </p>
            )}
          </>
        )
      )}
    </div>
  );
}
