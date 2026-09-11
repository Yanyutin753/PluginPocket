import { useSearchParams } from 'react-router';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { date, number } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
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
  const ledger = usePagedList(
    `/admin/ledger${filter.size ? `?${filter}` : ''}`,
    schema,
  );
  return (
    <>
      <Heading title={t('额度审计')} artwork="admin-ledger">
        {t('按用户、操作人和变动类型核对不可变账本。')}
      </Heading>
      <form
        key={filter.toString()}
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
          <div className="filter-actions">
            <Button type="submit" variant="outline">
              {t('筛选流水')}
            </Button>
            {filter.size > 0 && (
              <Button
                type="button"
                variant="ghost"
                onClick={() => setParams(new URLSearchParams())}
              >
                {t('清除筛选')}
              </Button>
            )}
          </div>
        </FieldGroup>
      </form>
      <ErrorNotice error={ledger.error} retry={() => void ledger.retry()} />
      {ledger.isPending && <Loading />}
      {ledger.isSuccess && !ledger.items.length ? (
        <p className="empty-state">{t('没有符合条件的额度变动。')}</p>
      ) : (
        ledger.data && (
          <section
            className="table-scroll"
            ref={ledger.listRef}
            aria-label={t('额度审计表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th scope="col">{t('时间')}</th>
                  <th scope="col">{t('用户')}</th>
                  <th scope="col">{t('操作人')}</th>
                  <th scope="col">{t('钱包')}</th>
                  <th scope="col">{t('变动类型')}</th>
                  <th scope="col" className="numeric">
                    {t('额度变动')}
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
                    <td data-label={t('用户')} className="cell-wrap">
                      {item.user_id}
                    </td>
                    <td data-label={t('操作人')}>
                      {item.actor_id ?? t('系统')}
                    </td>
                    <td data-label={t('钱包')}>{item.wallet_id}</td>
                    <td data-label={t('变动类型')}>
                      {t(kinds[item.kind] ?? item.kind)}
                    </td>
                    <td data-label={t('额度变动')} className="numeric">
                      {item.delta > 0 ? '+' : ''}
                      {number(item.delta, locale)}
                    </td>
                    <td data-label={t('备注')} className="cell-wrap">
                      {item.note || '—'}
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
    </>
  );
}
