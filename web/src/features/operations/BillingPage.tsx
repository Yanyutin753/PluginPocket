import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import {
  accountQuery,
  date,
  listOptions,
  metaQuery,
  number,
  request,
} from '../account/api';
import { ErrorNotice, Heading, Loading, More } from '../account/shared';
import { ledgerSchema, orderSchema, planSchema, price } from './api';
export default function BillingPage() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const account = useQuery(accountQuery);
  const meta = useQuery(metaQuery);
  const plans = useInfiniteQuery(listOptions('/plans', planSchema));
  const ledger = useInfiniteQuery(listOptions('/account/ledger', ledgerSchema));
  const orders = useInfiniteQuery(listOptions('/account/orders', orderSchema));
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
      <Heading artwork="credits" title={t('额度与账单')}>
        {t('当前可用额度 {credits}。兑换额度、查看变动记录和订单。', {
          credits: account.data
            ? number(account.data.user.balance, locale)
            : '—',
        })}
      </Heading>
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
      <div className="billing-sections">
        <section className="section-stack">
          <h2>{t('可用套餐')}</h2>
          {meta.data?.payments === false && (
            <p>{t('在线支付暂未配置，可使用兑换码或联系管理员补充额度。')}</p>
          )}
          <ErrorNotice error={meta.error} retry={() => void meta.refetch()} />
          <ErrorNotice error={plans.error} retry={() => void plans.refetch()} />
          <ErrorNotice error={purchase.error} />
          {plans.isPending && <Loading />}
          {plans.isSuccess && !plans.data.pages[0].items.length && (
            <p className="text-muted-foreground">
              {t('运营方暂未提供套餐，可使用兑换码补充额度。')}
            </p>
          )}
          <ul className="record-list">
            {plans.data?.pages
              .flatMap((page) => page.items)
              .map((plan) => (
                <li key={plan.id}>
                  <div className="record-main">
                    <h3>{plan.name}</h3>
                    <p>
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
                      purchase.isPending ||
                      !meta.isSuccess ||
                      !meta.data.payments
                    }
                    onClick={() => purchase.mutate(plan.id)}
                  >
                    {t('购买')}
                  </Button>
                </li>
              ))}
          </ul>
          <More
            hasNext={plans.hasNextPage}
            pending={plans.isFetchingNextPage}
            onClick={() => void plans.fetchNextPage()}
          />
        </section>
        <section className="section-stack">
          <h2>{t('额度流水')}</h2>
          <ErrorNotice
            error={ledger.error}
            retry={() => void ledger.refetch()}
          />
          {ledger.isPending && <Loading />}
          {ledger.isSuccess && !ledger.data.pages[0].items.length ? (
            <p className="text-muted-foreground">{t('还没有额度变动。')}</p>
          ) : (
            ledger.data && (
              <section
                className="table-scroll"
                aria-label={t('额度流水表格')}
                // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
                tabIndex={0}
              >
                <table>
                  <thead>
                    <tr>
                      <th>{t('时间')}</th>
                      <th>{t('变动')}</th>
                      <th>{t('备注')}</th>
                      <th>{t('变动后余额')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {ledger.data.pages
                      .flatMap((page) => page.items)
                      .map((item) => (
                        <tr key={item.id}>
                          <td>{date(item.created_at, locale)}</td>
                          <td>
                            {item.delta > 0 ? '+' : ''}
                            {number(item.delta, locale)}
                          </td>
                          <td>{item.note || item.kind}</td>
                          <td>
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
          <More
            hasNext={ledger.hasNextPage}
            pending={ledger.isFetchingNextPage}
            onClick={() => void ledger.fetchNextPage()}
          />
        </section>
        <section className="section-stack">
          <h2>{t('订单记录')}</h2>
          <ErrorNotice
            error={orders.error}
            retry={() => void orders.refetch()}
          />
          {orders.isPending && <Loading />}
          {orders.isSuccess && !orders.data.pages[0].items.length && (
            <p className="text-muted-foreground">{t('还没有订单。')}</p>
          )}
          <ul className="record-list">
            {orders.data?.pages
              .flatMap((page) => page.items)
              .map((item) => (
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
                  <span>
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
          <More
            hasNext={orders.hasNextPage}
            pending={orders.isFetchingNextPage}
            onClick={() => void orders.fetchNextPage()}
          />
        </section>
      </div>
    </>
  );
}
