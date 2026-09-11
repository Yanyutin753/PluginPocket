import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useI18n } from '@/i18n';
import { Button } from './ui/button';
import { Select } from './ui/select';

type PaginationProps = {
  label: string;
  page: number;
  pageSize: number;
  start: number;
  end: number;
  hasData: boolean;
  hasNext: boolean;
  pending: boolean;
  previous: () => void;
  next: () => void;
  setPageSize: (size: number) => void;
};
export function Pagination({
  label,
  page,
  pageSize,
  start,
  end,
  hasData,
  hasNext,
  pending,
  previous,
  next,
  setPageSize,
}: PaginationProps) {
  const { t } = useI18n();
  if (!hasData || end === 0) return null;
  return (
    <nav className="pagination" aria-label={t('{label}分页', { label })}>
      <p aria-live="polite" className="pagination-count">
        {pending
          ? t('正在加载…')
          : end
            ? t('第 {start}–{end} 条', { start, end })
            : t('暂无记录')}
      </p>
      <div className="pagination-controls">
        <Select
          aria-label={t('每页条数')}
          value={String(pageSize)}
          onValueChange={(value) => setPageSize(Number(value))}
          disabled={pending}
          options={[10, 20, 50].map((value) => ({
            value: String(value),
            label: t('{count} 条 / 页', { count: value }),
          }))}
        />
        <div className="pagination-pages">
          <Button
            variant="ghost"
            aria-label={t('上一页')}
            disabled={page === 1 || pending}
            onClick={previous}
          >
            <ChevronLeft aria-hidden="true" />
            <span>{t('上一页')}</span>
          </Button>
          <span className="pagination-current">
            {t('第 {page} 页', { page })}
          </span>
          <Button
            variant="ghost"
            aria-label={t('下一页')}
            disabled={!hasNext || pending}
            onClick={next}
          >
            <span>{t('下一页')}</span>
            <ChevronRight aria-hidden="true" />
          </Button>
        </div>
      </div>
    </nav>
  );
}
