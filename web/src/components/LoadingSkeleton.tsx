import { useI18n } from '@/i18n';
import './LoadingSkeleton.css';

type LoadingVariant = 'list' | 'summary' | 'form' | 'page';

export function LoadingSkeleton({
  variant = 'list',
}: {
  variant?: LoadingVariant;
}) {
  const { t } = useI18n();
  const label = t('正在加载，请稍候…');
  return (
    <div className={`loading-skeleton loading-skeleton--${variant}`}>
      {variant === 'page' ? (
        <header className="loading-preparation">
          <img
            src="/images/workshop-loading.webp"
            alt=""
            width="1000"
            height="750"
          />
          <h2>{t('正在准备你的工作空间')}</h2>
          <p role="status">{t('正在连接，请稍候…')}</p>
        </header>
      ) : (
        <p className="sr-only" role="status">
          {label}
        </p>
      )}
      <section aria-label={label} aria-busy="true">
        <div className="skeleton-shapes" aria-hidden="true">
          {variant === 'form' && <span className="skeleton-heading" />}
          {(variant === 'summary' || variant === 'page') && (
            <div className="skeleton-metrics">
              {['balance', 'calls', 'cost', 'tokens'].map((key) => (
                <div className="skeleton-metric" key={key}>
                  <span className="skeleton-label" />
                  <span className="skeleton-value" />
                </div>
              ))}
            </div>
          )}
          {variant !== 'summary' && (
            <div className="skeleton-rows">
              {(variant === 'form'
                ? ['field']
                : ['first', 'second', 'third']
              ).map((key) => (
                <div className="skeleton-row" key={key}>
                  <span className="skeleton-label" />
                  <span className="skeleton-detail" />
                </div>
              ))}
            </div>
          )}
          {variant === 'form' && <span className="skeleton-action" />}
        </div>
      </section>
    </div>
  );
}
