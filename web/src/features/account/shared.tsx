import type { ReactNode } from 'react';
import { LoadingSkeleton } from '@/components/LoadingSkeleton';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';

export function Heading({
  title,
  children,
  translateTitle = true,
  artwork,
}: {
  title: string;
  translateTitle?: boolean;
  children?: ReactNode;
  artwork?:
    | 'tokens'
    | 'usage'
    | 'teams'
    | 'team-detail'
    | 'team-usage'
    | 'health'
    | 'device'
    | 'settings'
    | 'admin-users'
    | 'admin-tools'
    | 'admin-plans'
    | 'admin-codes'
    | 'admin-usage'
    | 'admin-ledger'
    | 'admin-settings'
    | 'tool-catalog'
    | 'verify-email'
    | 'billing';
}) {
  const { t } = useI18n();
  return (
    <header
      className={artwork ? 'page-heading illustrated-heading' : 'page-heading'}
    >
      <div>
        <h1>{translateTitle ? t(title) : title}</h1>
        {children && (
          <p>{typeof children === 'string' ? t(children) : children}</p>
        )}
      </div>
      {artwork && (
        <img
          src={`/images/workshop-${artwork}.webp`}
          alt=""
          width="900"
          height="600"
        />
      )}
    </header>
  );
}
export function ErrorNotice({
  error,
  retry,
}: {
  error: Error | null;
  retry?: () => void;
}) {
  const { t } = useI18n();
  if (!error) return null;
  return (
    <div className="error-notice">
      <p role="alert">{t(error.message)}</p>
      {retry && (
        <Button variant="outline" onClick={retry}>
          {t('重试')}
        </Button>
      )}
    </div>
  );
}
export function Loading({
  variant,
}: {
  variant?: 'list' | 'summary' | 'form' | 'page';
}) {
  return <LoadingSkeleton variant={variant} />;
}
export function More({
  hasNext,
  pending,
  onClick,
}: {
  hasNext: boolean;
  pending: boolean;
  onClick: () => void;
}) {
  const { t } = useI18n();
  return hasNext ? (
    <Button variant="outline" disabled={pending} onClick={onClick}>
      {pending ? t('正在加载…') : t('加载更多')}
    </Button>
  ) : null;
}
