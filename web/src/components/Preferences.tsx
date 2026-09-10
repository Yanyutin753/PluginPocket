import { Globe2, SunMoon } from 'lucide-react';
import { useI18n } from '@/i18n';
import { Select } from './ui/select';
export function PreferencesControls() {
  const { locale, setLocale, theme, setTheme, t } = useI18n();
  return (
    <div className="preferences-controls">
      <div className="preference-control">
        <span className="sr-only">{t('语言')}</span>
        <Select
          icon={<Globe2 />}
          aria-label={t('语言')}
          value={locale}
          onValueChange={(value) => setLocale(value === 'en' ? 'en' : 'zh-CN')}
          options={[
            { value: 'zh-CN', label: '中文' },
            { value: 'en', label: 'English' },
          ]}
        />
      </div>
      <div className="preference-control">
        <span className="sr-only">{t('外观')}</span>
        <Select
          icon={<SunMoon />}
          aria-label={t('外观')}
          value={theme}
          onValueChange={(value) =>
            setTheme(
              value === 'light'
                ? 'light'
                : value === 'dark'
                  ? 'dark'
                  : 'system',
            )
          }
          options={[
            { value: 'system', label: t('跟随系统') },
            { value: 'light', label: t('浅色') },
            { value: 'dark', label: t('深色') },
          ]}
        />
      </div>
    </div>
  );
}
