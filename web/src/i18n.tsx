import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { en } from './i18n/catalog';
import { landing } from './i18n/landing';
import { marketplace } from './i18n/marketplace';
import { plugins } from './i18n/plugins';
import { shell } from './i18n/shell';
import { toolEditor } from './i18n/tool-editor';
import { toolIcons } from './i18n/tool-icons';

type Locale = 'zh-CN' | 'en';
type Theme = 'light' | 'dark' | 'system';
type Values = Record<string, string | number>;
const english = {
  ...toolIcons,
  ...toolEditor,
  ...landing,
  ...en,
  ...shell,
  ...plugins,
  ...marketplace,
};
function translate(locale: Locale, key: string, values?: Values) {
  const message = locale === 'en' ? (english[key] ?? key) : key;
  return message
    .replace(/。(?=\S)/g, ' ')
    .replaceAll('。', '')
    .replace(/\{(\w+)\}/g, (match, name: string) =>
      String(values?.[name] ?? match),
    );
}
function stored(key: string) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}
function save(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* Preferences still work when storage is unavailable. */
  }
}
const defaultLocale: Locale = 'zh-CN';
const Preferences = createContext({
  locale: defaultLocale as Locale,
  theme: 'system' as Theme,
  setLocale: (_: Locale) => {},
  setTheme: (_: Theme) => {},
  t: (key: string, values?: Values) => translate(defaultLocale, key, values),
});
export function PreferencesProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(() =>
    stored('loadout.locale') === 'en' ? 'en' : 'zh-CN',
  );
  const [theme, setTheme] = useState<Theme>(() => {
    const value = stored('loadout.theme');
    return value === 'light' || value === 'dark' ? value : 'system';
  });
  useEffect(() => {
    document.documentElement.lang = locale;
    document.title =
      locale === 'en'
        ? 'Loadout · Your AI, fully loaded.'
        : 'Loadout · 你的 AI，准备就绪';
    save('loadout.locale', locale);
  }, [locale]);
  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: dark)');
    const apply = () => {
      const resolved =
        theme === 'system' ? (media?.matches ? 'dark' : 'light') : theme;
      document.documentElement.dataset.theme = resolved;
      document.documentElement.style.colorScheme = resolved;
      document
        .querySelector('meta[name="theme-color"]')
        ?.setAttribute('content', resolved === 'dark' ? '#161719' : '#f5f5f7');
    };
    apply();
    save('loadout.theme', theme);
    media?.addEventListener('change', apply);
    return () => media?.removeEventListener('change', apply);
  }, [theme]);
  const value = useMemo(
    () => ({
      locale,
      theme,
      setLocale,
      setTheme,
      t: (key: string, values?: Values) => translate(locale, key, values),
    }),
    [locale, theme],
  );
  return <Preferences value={value}>{children}</Preferences>;
}
export function useI18n() {
  return useContext(Preferences);
}
