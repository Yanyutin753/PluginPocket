import { useQuery } from '@tanstack/react-query';
import { Link, useLocation } from 'react-router';
import { PreferencesControls } from '@/components/Preferences';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';
import './LandingPage.css';
import { publicSessionQuery } from './account/api';
export function PublicHeader() {
  const { t } = useI18n();
  const session = useQuery(publicSessionQuery);
  const location = useLocation();
  return (
    <header className="landing-nav">
      <Link
        className="landing-brand"
        to="/"
        aria-label={t('PluginPocket 首页')}
      >
        <img src="/images/workshop-mark.webp" alt="" width="48" height="48" />
        <span>
          <span className="brand-wordmark">PluginPocket</span>
          <small>{t('AI 装备工坊')}</small>
        </span>
      </Link>
      <nav aria-label={t('主导航')}>
        <PreferencesControls />
        <Link className="inline-action" to="/plugins">
          {t('插件市场')}
        </Link>
        <Link className="inline-action" to="/download">
          {t('下载客户端')}
        </Link>
        {session.data && !session.error ? (
          <Button asChild>
            <Link to="/overview">{t('进入工作空间')}</Link>
          </Button>
        ) : (
          <>
            <Link
              className="inline-action"
              to="/login"
              state={{ from: location.pathname + location.search }}
            >
              {t('登录')}
            </Link>
            <Button asChild>
              <Link to="/register">{t('创建账号')}</Link>
            </Button>
          </>
        )}
      </nav>
    </header>
  );
}
