import { useQuery } from '@tanstack/react-query';
import { SidePanel } from '@/components/SidePanel';
import { useI18n } from '@/i18n';
import {
  date,
  multiplierText,
  number,
  request,
  usageDetailSchema,
} from './api';
import { ErrorNotice, Loading } from './shared';

export function UsageDetail({
  path,
  onClose,
}: {
  path: string;
  onClose: () => void;
}) {
  const { t, locale } = useI18n();
  const detail = useQuery({
    queryKey: ['usage-detail', path],
    queryFn: ({ signal }) => request(path, usageDetailSchema, { signal }),
    retry: false,
  });
  const item = detail.data?.item;
  return (
    <SidePanel title={t('调用详情')} onClose={onClose}>
      {detail.isPending && <Loading variant="form" />}
      <ErrorNotice error={detail.error} retry={() => void detail.refetch()} />
      {item && (
        <>
          <div className="grid min-w-0 gap-2">
            <code className="break-all">{item.tool}</code>
            <p className="text-sm text-muted-foreground">
              {date(item.created_at, locale)} ·{' '}
              {number(item.duration_ms, locale)} ms · {t('额度')}{' '}
              {number(item.cost, locale)}
              {item.billing_role && item.multiplier_bp !== undefined
                ? ` · ${item.billing_role} ${multiplierText(item.multiplier_bp)}`
                : ''}
            </p>
          </div>
          {(
            [
              ['传入数据', item.input_data, item.input_truncated],
              ['传出数据', item.output_data, item.output_truncated],
            ] as const
          ).map(([label, value, truncated]) => (
            <section
              key={label}
              className="grid min-w-0 gap-3"
              aria-label={t(label)}
            >
              <h3 className="text-base font-semibold">{t(label)}</h3>
              {value === null ? (
                <p className="text-sm text-muted-foreground">{t('未记录')}</p>
              ) : (
                <pre
                  className="max-h-96 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted p-4 text-sm"
                  // biome-ignore lint/a11y/noNoninteractiveTabindex: Recorded payloads need keyboard scrolling.
                  tabIndex={0}
                >
                  {value}
                </pre>
              )}
              {truncated && (
                <p className="text-sm text-muted-foreground">
                  {t('内容较大，仅保留前 64 KiB。')}
                </p>
              )}
            </section>
          ))}
        </>
      )}
    </SidePanel>
  );
}
