import { useState } from 'react';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { Field, FieldLabel } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { number } from '../account/api';
import { ErrorNotice, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';

type UsageSummaryProps = { path: string; team: boolean };

export function UsageSummary({ path, team }: UsageSummaryProps) {
  const { t, locale } = useI18n();
  const [days, setDays] = useState('7');
  const metrics = {
    calls: z.number().int(),
    cost: z.number().int(),
    errors: z.number().int(),
  };
  const schema = team
    ? z
        .object({ ...metrics, user_id: z.number().int(), username: z.string() })
        .transform((item) => ({
          ...item,
          label: item.username,
          key: String(item.user_id),
        }))
    : z
        .object({ ...metrics, tool: z.string() })
        .transform((item) => ({ ...item, label: item.tool, key: item.tool }));
  const summary = usePagedList(`${path}/summary?days=${days}`, schema);
  const items = summary.items ?? [];
  return (
    <section className="section-stack usage-summary">
      <div className="summary-heading">
        <h2>{t('调用汇总')}</h2>
        <Field className="max-w-xs">
          <FieldLabel htmlFor="summary-days">{t('汇总时间范围')}</FieldLabel>
          <Select
            id="summary-days"
            value={days}
            onValueChange={setDays}
            options={[
              { value: '7', label: t('近 7 天（UTC）') },
              { value: '1', label: t('今天（UTC）') },
            ]}
          />
        </Field>
      </div>
      <ErrorNotice
        error={summary.error}
        retry={() => {
          void summary.retry();
        }}
      />
      {summary.isPending && <Loading variant="summary" />}
      {summary.data &&
        (!items.length ? (
          <p>{t('当前时间范围暂无调用。')}</p>
        ) : (
          <section
            className="table-scroll"
            ref={summary.listRef}
            aria-label={t('调用汇总表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th scope="col">{team ? t('成员') : t('工具')}</th>
                  <th scope="col" className="numeric">
                    {t('调用次数')}
                  </th>
                  <th scope="col" className="numeric">
                    {t('消耗额度')}
                  </th>
                  <th scope="col" className="numeric">
                    {t('失败次数')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.key}>
                    <td
                      data-label={team ? t('成员') : t('工具')}
                      className="cell-wrap"
                    >
                      {item.label}
                    </td>
                    <td data-label={t('调用次数')} className="numeric">
                      {number(item.calls, locale)}
                    </td>
                    <td data-label={t('消耗额度')} className="numeric">
                      {number(item.cost, locale)}
                    </td>
                    <td data-label={t('失败次数')} className="numeric">
                      {number(item.errors, locale)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        ))}
      <Pagination label={t('调用汇总')} {...summary.pagination} />
    </section>
  );
}
