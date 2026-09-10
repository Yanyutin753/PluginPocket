import { useInfiniteQuery, useMutation } from '@tanstack/react-query';
import { useParams, useSearchParams } from 'react-router';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { UsageSummary } from '../operations/UsageSummary';
import { date, listOptions, number, requestCSV, usageSchema } from './api';
import { ErrorNotice, Heading, Loading, More } from './shared';

const status = {
  pending: '处理中',
  ok: '成功',
  error: '失败',
  recovered: '已恢复额度',
  denied: '已拒绝',
};
export default function UsagePage({
  admin = false,
  team = false,
}: {
  admin?: boolean;
  team?: boolean;
}) {
  const { t, locale } = useI18n();
  const { id } = useParams();
  const [params, setParams] = useSearchParams();
  const base = admin
    ? '/admin/usage'
    : team
      ? `/account/teams/${id}/usage`
      : '/account/usage';
  const filter = new URLSearchParams();
  for (const key of ['tool', 'status', ...(admin || team ? ['user_id'] : [])]) {
    const value = params.get(key);
    if (value) filter.set(key, value);
  }
  const path = `${base}${filter.size ? `?${filter}` : ''}`;
  const usage = useInfiniteQuery(listOptions(path, usageSchema));
  const exporter = useMutation({
    mutationFn: (cursor: string) =>
      requestCSV(
        `${base}/export?${filter}&limit=50&cursor=${encodeURIComponent(cursor)}`,
      ),
  });
  return (
    <>
      <Heading
        artwork="insights"
        title={admin ? t('全局用量') : team ? t('团队用量') : t('用量明细')}
      >
        {t('逐次查看工具调用、执行结果与实际消耗。')}
      </Heading>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const next = new URLSearchParams();
          for (const key of ['tool', 'status', 'user_id']) {
            const value = String(data.get(key) ?? '').trim();
            if (value) next.set(key, value);
          }
          exporter.reset();
          setParams(next);
        }}
      >
        <FieldGroup className="filter-fields grid gap-4">
          <Field>
            <FieldLabel htmlFor="filter-tool">{t('工具筛选')}</FieldLabel>
            <Input
              id="filter-tool"
              name="tool"
              defaultValue={params.get('tool') ?? ''}
              placeholder={t('工具标识')}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="filter-status">{t('结果筛选')}</FieldLabel>
            <Select
              id="filter-status"
              name="status"
              defaultValue={params.get('status') ?? ''}
              options={[
                { value: '', label: t('全部结果') },
                ...Object.entries(status).map(([value, label]) => ({
                  value,
                  label: t(label),
                })),
              ]}
            />
          </Field>
          {(admin || team) && (
            <Field>
              <FieldLabel htmlFor="filter-user">{t('成员编号筛选')}</FieldLabel>
              <Input
                id="filter-user"
                name="user_id"
                type="number"
                min="1"
                step="1"
                defaultValue={params.get('user_id') ?? ''}
              />
            </Field>
          )}
          <Button type="submit" variant="outline">
            {t('应用筛选')}
          </Button>
        </FieldGroup>
      </form>
      {(admin || team) && (
        <div className="section-stack">
          <div className="action-row">
            <Button
              variant="outline"
              disabled={exporter.isPending}
              onClick={() => exporter.mutate('')}
            >
              {exporter.isPending ? t('正在导出…') : t('导出 CSV')}
            </Button>
            {exporter.data && (
              <a
                className="inline-action"
                href={`data:text/csv;charset=utf-8,${encodeURIComponent(`\uFEFF${exporter.data.text}`)}`}
                download="loadout-usage.csv"
              >
                {t('下载 CSV')}
              </a>
            )}
            {exporter.data?.cursor && (
              <Button
                variant="outline"
                disabled={exporter.isPending}
                onClick={() => exporter.mutate(exporter.data?.cursor ?? '')}
              >
                {t('导出下一页')}
              </Button>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {t('每次导出最多 50 条记录。下载当前文件后，可继续导出下一页。')}
          </p>
          <ErrorNotice error={exporter.error} />
        </div>
      )}
      {(admin || team) && <UsageSummary path={base} team={team} />}
      <ErrorNotice error={usage.error} retry={() => void usage.refetch()} />
      {usage.isPending && <Loading />}
      {usage.isSuccess && usage.data.pages[0].items.length === 0 ? (
        <section className="empty-state">
          <h2>
            {filter.size ? t('没有符合筛选条件的记录') : t('还没有调用记录')}
          </h2>
          <p>{t('通过客户端调用工具后，记录会显示在这里。')}</p>
        </section>
      ) : (
        usage.data && (
          <section
            className="table-scroll"
            aria-label={t('调用明细表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  {(admin || team) && <th>{t('成员编号')}</th>}
                  <th>{t('工具')}</th>
                  <th>{t('时间')}</th>
                  <th>{t('结果')}</th>
                  <th>{t('耗时')}</th>
                  <th>{t('额度')}</th>
                </tr>
              </thead>
              <tbody>
                {usage.data.pages
                  .flatMap((page) => page.items)
                  .map((item) => (
                    <tr key={item.id}>
                      {(admin || team) && <td>{item.user_id ?? '—'}</td>}
                      <td>
                        <code>{item.tool}</code>
                      </td>
                      <td>{date(item.created_at, locale)}</td>
                      <td>{t(status[item.status])}</td>
                      <td>{number(item.duration_ms, locale)} ms</td>
                      <td>{number(item.cost, locale)}</td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </section>
        )
      )}
      <More
        hasNext={usage.hasNextPage}
        pending={usage.isFetchingNextPage}
        onClick={() => void usage.fetchNextPage()}
      />
    </>
  );
}
