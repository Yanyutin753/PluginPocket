import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';
export function OneTimeCode({
  title,
  code,
  hide,
  expires,
}: {
  title: string;
  code: string;
  hide: () => void;
  expires?: string;
}) {
  const { t } = useI18n();
  const [message, setMessage] = useState('');
  return (
    <section className="secret-panel" aria-label={t(title)}>
      <h2>{t('请立即保存{title}', { title: t(title) })}</h2>
      <p>
        {t('隐藏或离开页面后，不再显示明文。')}
        {expires ? t('有效期至 {value1}。', { value1: expires }) : ''}
      </p>
      <code>{code}</code>
      <div className="action-row">
        <Button
          variant="outline"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(code);
              setMessage('已复制');
            } catch {
              setMessage('无法自动复制，请选中并手动复制。');
            }
          }}
        >
          {t('复制{title}', { title: t(title) })}
        </Button>
        <Button onClick={hide}>
          {t('我已保存，隐藏{title}', { title: t(title) })}
        </Button>
      </div>
      {message && <p role="status">{t(message)}</p>}
    </section>
  );
}
