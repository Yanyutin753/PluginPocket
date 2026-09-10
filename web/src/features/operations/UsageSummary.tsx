import { useInfiniteQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Field, FieldLabel } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { listOptions, number } from '../account/api';
import { ErrorNotice, Loading, More } from '../account/shared';

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
  const summary = useInfiniteQuery(
    listOptions(`${path}/summary?days=${days}`, schema),
  );
  const items = summary.data?.pages.flatMap((page) => page.items) ?? [];
  return (
    <section className="section-stack">
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
      <ErrorNotice
        error={summary.error}
        retry={() => {
          if (summary.isFetchNextPageError) void summary.fetchNextPage();
          else void summary.refetch();
        }}
      />
      {summary.isPending && <Loading variant="summary" />}
      {summary.data &&
        (!items.length ? (
          <p>{t('当前时间范围暂无调用。')}</p>
        ) : (
          <section
            className="table-scroll"
            aria-label={t('调用汇总表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th>{team ? t('成员') : t('工具')}</th>
                  <th>{t('调用次数')}</th>
                  <th>{t('消耗额度')}</th>
                  <th>{t('失败次数')}</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.key}>
                    <td>{item.label}</td>
                    <td>{number(item.calls, locale)}</td>
                    <td>{number(item.cost, locale)}</td>
                    <td>{number(item.errors, locale)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        ))}
      <More
        hasNext={summary.hasNextPage}
        pending={summary.isFetching}
        onClick={() => void summary.fetchNextPage()}
      />
    </section>
  );
}
