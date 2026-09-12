import { useIsMutating } from '@tanstack/react-query';
import { isTauri } from '@tauri-apps/api/core';
import {
  Activity,
  ArrowUpRight,
  BookOpen,
  LayoutDashboard,
  Monitor,
  Package,
  ShieldCheck,
  SunMoon,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import workshopMark from '../../../web/public/images/workshop-mark.webp';
import { Select } from '../../../web/src/components/ui/select';
import { DiagnosticsPage } from './DiagnosticsPage';
import { EquipmentPage } from './EquipmentPage';
import { I18nProvider, useI18n } from './i18n';
import { LogsPage } from './LogsPage';
import { Overview } from './Overview';
import { updater } from './updater';

const pageDefinitions = [
  {
    id: 'overview',
    label: '概览',
    icon: LayoutDashboard,
    description: '你的本机 AI 装备工作台',
  },
  {
    id: 'equipment',
    label: '我的装备',
    icon: Package,
    description: '管理这台电脑上由 PluginPocket 安装的工具与技能',
  },
  {
    id: 'logs',
    label: '运行日志',
    icon: BookOpen,
    description: '每一步操作，有迹可循',
  },
  {
    id: 'diagnostics',
    label: '连接诊断',
    icon: Activity,
    description: '找到连接问题，知道下一步怎么做',
  },
] as const;
type Page = (typeof pageDefinitions)[number]['id'];

export function App() {
  return (
    <I18nProvider>
      <Workbench />
    </I18nProvider>
  );
}
function Workbench() {
  const { t, language, setLanguage } = useI18n();
  const pages = pageDefinitions.map((page) => ({
    ...page,
    label: t(page.label),
    description: t(page.description),
  }));
  const native = isTauri();
  const [page, setPage] = useState<Page>('overview');
  const [theme, setTheme] = useState('system');
  const [appVersion, setAppVersion] = useState('…');
  const busy = useIsMutating() > 0;
  const heading = useRef<HTMLHeadingElement>(null);
  const selected = pages.find((item) => item.id === page) ?? pages[0];
  useEffect(() => {
    document.documentElement.style.colorScheme =
      theme === 'system' ? 'light dark' : theme;
  }, [theme]);
  useEffect(() => {
    updater
      .currentVersion()
      .then(setAppVersion)
      .catch(() => {});
  }, []);
  function navigate(next: Page) {
    setPage(next);
    requestAnimationFrame(() => heading.current?.focus());
  }
  return (
    <div className="workbench">
      <header className="workbench-topbar">
        <span className="workspace-location">
          <Monitor size={16} aria-hidden="true" /> {t('本机工作台')}{' '}
          <span aria-hidden="true">/</span> <strong>{selected.label}</strong>
        </span>
        <div className="theme-control">
          <Select
            icon={<SunMoon />}
            aria-label={t('外观')}
            value={theme}
            onValueChange={setTheme}
            options={[
              { value: 'light', label: t('浅色') },
              { value: 'dark', label: t('深色') },
              { value: 'system', label: t('跟随系统') },
            ]}
          />
          <Select
            icon={<Monitor />}
            aria-label={t('语言')}
            value={language}
            onValueChange={(value) => setLanguage(value as 'zh' | 'en')}
            options={[
              { value: 'zh', label: t('中文') },
              { value: 'en', label: t('英文') },
            ]}
          />
        </div>
      </header>
      <aside className="workbench-sidebar">
        <div className="desktop-brand">
          <img src={workshopMark} alt="" width="44" height="44" />
          <span>
            <span className="brand-wordmark">PluginPocket</span>
            <small>{t('插件口袋 · AI 装备工坊')}</small>
          </span>
        </div>
        <nav aria-label={t('工作台')}>
          {pages.map(({ id, label, icon: Icon }) => (
            <button
              type="button"
              key={id}
              onClick={() => navigate(id)}
              aria-current={page === id ? 'page' : undefined}
              disabled={busy}
            >
              <Icon size={19} aria-hidden="true" />
              <span>{label}</span>
              {page === id && (
                <span className="nav-marker" aria-hidden="true" />
              )}
            </button>
          ))}
        </nav>
        <div className="sidebar-note">
          <ShieldCheck size={20} aria-hidden="true" />
          <strong>{t('工具随身，密钥留在本机')}</strong>
          <p>{t('一个入口，连接你的 AI 客户端。')}</p>
        </div>
        <div className="sidebar-foot">
          <span className="runtime-mark">
            <Monitor size={15} aria-hidden="true" />{' '}
            {native ? t('桌面应用') : t('浏览器预览')}
          </span>
          <span>v{appVersion}</span>
        </div>
      </aside>
      <main className="workbench-content" id="main-content">
        <div className="page-heading">
          <h1 ref={heading} tabIndex={-1}>
            {selected.label}
          </h1>
          <p>{selected.description}</p>
        </div>
        {!native && (
          <div className="preview-notice">
            <Monitor size={20} aria-hidden="true" />
            <p>
              <strong>{t('界面实时预览')}</strong>
              <span>
                {t(
                  '可切换页面与外观。本机日志、装备和诊断需在桌面应用中读取。',
                )}
              </span>
            </p>
            <ArrowUpRight size={18} aria-hidden="true" />
          </div>
        )}
        {page === 'overview' && <Overview native={native} />}
        {page === 'equipment' && (
          <EquipmentPage
            native={native}
            onManageConnection={() => navigate('overview')}
          />
        )}
        {page === 'logs' && <LogsPage native={native} />}
        {page === 'diagnostics' && <DiagnosticsPage native={native} />}
      </main>
    </div>
  );
}
