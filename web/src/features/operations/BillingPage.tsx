import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { accountQuery, date, metaQuery, number, request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
import { ledgerSchema, orderSchema, planSchema, price } from './api';
export default function BillingPage() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const account = useQuery(accountQuery);
  const meta = useQuery(metaQuery);
  const plans = usePagedList('/plans', planSchema);
  const ledger = usePagedList('/account/ledger', ledgerSchema);
  const orders = usePagedList('/account/orders', orderSchema);
  const [code, setCode] = useState('');
  const redeem = useMutation({
    mutationFn: async () => {
      const data = await request(
        '/account/redeem',
        z.object({ balance: z.number().int(), credits: z.number().int() }),
        { method: 'POST', body: JSON.stringify({ code }) },
      );
      setCode('');
      return data;
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['account'] });
      void client.invalidateQueries({ queryKey: ['/account/ledger'] });
    },
  });
  const purchase = useMutation({
    mutationFn: (id: number) =>
      request('/account/orders', z.object({ item: orderSchema }), {
        method: 'POST',
        body: JSON.stringify({ plan_id: id }),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/account/orders'] });
    },
  });
  return (
    <>
      <Heading artwork="billing" title={t('额度与账单')}>
        {t('当前可用额度 {credits}。兑换额度、查看变动记录和订单。', {
          credits: account.data
            ? number(account.data.user.balance, locale)
            : '—',
        })}
      </Heading>
      <SidePanel
        title={t('兑换额度')}
        trigger={t('兑换额度')}
        locked={redeem.isPending}
      >
        <form
          className="inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            redeem.mutate();
          }}
        >
          <FieldGroup className="min-w-0 flex-1">
            <Field>
              <FieldLabel htmlFor="redeem-code">{t('兑换码')}</FieldLabel>
              <Input
                id="redeem-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                required
                autoComplete="off"
                disabled={redeem.isPending}
              />
            </Field>
          </FieldGroup>
          <Button type="submit" disabled={redeem.isPending}>
            {redeem.isPending ? t('正在兑换…') : t('兑换额度')}
          </Button>
        </form>
        <ErrorNotice error={redeem.error} />
        {redeem.data && (
          <p role="status">
            {t('已兑换 {credits} 额度，当前余额 {balance}。', {
              credits: number(redeem.data.credits, locale),
              balance: number(redeem.data.balance, locale),
            })}
          </p>
        )}
      </SidePanel>
      <div className="billing-sections">
        <section className="section-stack">
          <h2>{t('可用套餐')}</h2>
          {meta.data?.payments === false && (
            <p>{t('在线支付暂未配置，可使用兑换码或联系管理员补充额度。')}</p>
          )}
          <ErrorNotice error={meta.error} retry={() => void meta.refetch()} />
          <ErrorNotice error={plans.error} retry={() => void plans.retry()} />
          <ErrorNotice error={purchase.error} />
          {plans.isPending && <Loading />}
          {plans.isSuccess && !plans.items.length && (
            <p className="text-muted-foreground">
              {t('运营方暂未提供套餐，可使用兑换码补充额度。')}
            </p>
          )}
          <ul
            className="record-list plan-options"
            ref={plans.listRef}
            tabIndex={-1}
          >
            {plans.items.map((plan) => (
              <li key={plan.id}>
                <div className="record-main">
                  <h3>{plan.name}</h3>
                  <p className="record-values numeric">
                    {t('{credits} 额度 · {price}', {
                      credits: number(plan.credits, locale),
                      price: price(plan.price_cents, plan.currency, locale),
                    })}
                  </p>
                </div>
                <Button
                  variant="outline"
                  aria-label={t('购买 {value1}', { value1: plan.name })}
                  disabled={
                    purchase.isPending || !meta.isSuccess || !meta.data.payments
                  }
                  onClick={() => purchase.mutate(plan.id)}
                >
                  {t('购买')}
                </Button>
              </li>
            ))}
          </ul>
          <Pagination label={t('套餐')} {...plans.pagination} />
        </section>
        <section className="section-stack">
          <h2>{t('额度流水')}</h2>
          <ErrorNotice error={ledger.error} retry={() => void ledger.retry()} />
          {ledger.isPending && <Loading />}
          {ledger.isSuccess && !ledger.items.length ? (
            <p className="text-muted-foreground">{t('还没有额度变动。')}</p>
          ) : (
            ledger.data && (
              <section
                className="table-scroll"
                ref={ledger.listRef}
                aria-label={t('额度流水表格')}
                // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
                tabIndex={0}
              >
                <table>
                  <thead>
                    <tr>
                      <th scope="col">{t('时间')}</th>
                      <th scope="col" className="numeric">
                        {t('变动')}
                      </th>
                      <th scope="col">{t('备注')}</th>
                      <th scope="col" className="numeric">
                        {t('变动后余额')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {ledger.items.map((item) => (
                      <tr key={item.id}>
                        <td data-label={t('时间')}>
                          {date(item.created_at, locale)}
                        </td>
                        <td data-label={t('变动')} className="numeric">
                          {item.delta > 0 ? '+' : ''}
                          {number(item.delta, locale)}
                        </td>
                        <td data-label={t('备注')} className="cell-wrap">
                          {item.note || item.kind}
                        </td>
                        <td data-label={t('变动后余额')} className="numeric">
                          {item.balance_after === null
                            ? '—'
                            : number(item.balance_after, locale)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            )
          )}
          <Pagination label={t('额度流水')} {...ledger.pagination} />
        </section>
        <section className="section-stack">
          <h2>{t('订单记录')}</h2>
          <ErrorNotice error={orders.error} retry={() => void orders.retry()} />
          {orders.isPending && <Loading />}
          {orders.isSuccess && !orders.items.length && (
            <p className="text-muted-foreground">{t('还没有订单。')}</p>
          )}
          <ul className="record-list" ref={orders.listRef} tabIndex={-1}>
            {orders.items.map((item) => (
              <li key={item.id}>
                <div className="record-main">
                  <h3>{t('订单 #{id}', { id: item.id })}</h3>
                  <p>
                    {t('{credits} 额度 · {price}', {
                      credits: number(item.credits, locale),
                      price: price(item.price_cents, item.currency, locale),
                    })}{' '}
                    · {date(item.created_at, locale)}
                  </p>
                </div>
                <span
                  className="status-badge"
                  data-tone={
                    item.status === 'paid'
                      ? 'success'
                      : item.status === 'failed'
                        ? 'danger'
                        : 'neutral'
                  }
                >
                  {(
                    {
                      pending: t('待支付'),
                      paid: t('已支付'),
                      cancelled: t('已取消'),
                      failed: t('失败'),
                    } as Record<string, string>
                  )[item.status] ?? t('处理中')}
                </span>
              </li>
            ))}
          </ul>
          <Pagination label={t('订单')} {...orders.pagination} />
        </section>
      </div>
    </>
  );
}
