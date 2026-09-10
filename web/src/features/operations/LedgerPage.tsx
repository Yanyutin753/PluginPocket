import { useInfiniteQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { date, listOptions, number } from '../account/api';
import { ErrorNotice, Heading, Loading, More } from '../account/shared';
import { ledgerSchema } from './api';

const schema = ledgerSchema.extend({
  user_id: z.number().int(),
  actor_id: z.number().int().nullable(),
  wallet_id: z.number().int(),
});
const kinds: Record<string, string> = {
  registration: '注册额度',
  adjustment: '人工调账',
  redemption: '兑换码',
  team_transfer: '团队转账',
  reservation: '调用预占',
  refund: '失败退款',
  recovery: '异常恢复',
};
export default function LedgerPage() {
  const { t, locale } = useI18n();
  const [params, setParams] = useSearchParams();
  const filter = new URLSearchParams();
  for (const key of ['user_id', 'actor_id', 'kind']) {
    const value = params.get(key);
    if (value) filter.set(key, value);
  }
  const ledger = useInfiniteQuery(
    listOptions(`/admin/ledger${filter.size ? `?${filter}` : ''}`, schema),
  );
  return (
    <>
      <Heading title={t('额度审计')}>
        {t('按用户、操作人和变动类型核对不可变账本。')}
      </Heading>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const next = new URLSearchParams();
          for (const key of ['user_id', 'actor_id', 'kind']) {
            const value = String(data.get(key) ?? '');
            if (value) next.set(key, value);
          }
          setParams(next);
        }}
      >
        <FieldGroup className="filter-fields grid gap-4">
          <Field>
            <FieldLabel htmlFor="ledger-user">{t('用户编号')}</FieldLabel>
            <Input
              id="ledger-user"
              name="user_id"
              type="number"
              min="1"
              step="1"
              defaultValue={params.get('user_id') ?? ''}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="ledger-actor">{t('操作人编号')}</FieldLabel>
            <Input
              id="ledger-actor"
              name="actor_id"
              type="number"
              min="1"
              step="1"
              defaultValue={params.get('actor_id') ?? ''}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="ledger-kind">{t('变动类型')}</FieldLabel>
            <Select
              id="ledger-kind"
              name="kind"
              defaultValue={params.get('kind') ?? ''}
              options={[
                { value: '', label: t('所有类型') },
                ...Object.entries(kinds).map(([value, label]) => ({
                  value,
                  label: t(label),
                })),
              ]}
            />
          </Field>
          <Button type="submit" variant="outline">
            {t('筛选流水')}
          </Button>
        </FieldGroup>
      </form>
      <ErrorNotice error={ledger.error} retry={() => void ledger.refetch()} />
      {ledger.isPending && <Loading />}
      {ledger.isSuccess && !ledger.data.pages[0].items.length ? (
        <p className="empty-state">{t('没有符合条件的额度变动。')}</p>
      ) : (
        ledger.data && (
          <section
            className="table-scroll"
            aria-label={t('额度审计表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th>{t('时间')}</th>
                  <th>{t('用户')}</th>
                  <th>{t('操作人')}</th>
                  <th>{t('钱包')}</th>
                  <th>{t('变动类型')}</th>
                  <th>{t('额度变动')}</th>
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
                      <td>{item.user_id}</td>
                      <td>{item.actor_id ?? t('系统')}</td>
                      <td>{item.wallet_id}</td>
                      <td>{t(kinds[item.kind] ?? item.kind)}</td>
                      <td>
                        {item.delta > 0 ? '+' : ''}
                        {number(item.delta, locale)}
                      </td>
                      <td>{item.note || '—'}</td>
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
    </>
  );
}
