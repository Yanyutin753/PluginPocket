import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router';
import { z } from 'zod';
import { PreferencesControls } from '@/components/Preferences';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice, Heading } from '../account/shared';
export default function VerifyEmailPage() {
  const { t } = useI18n();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const token = params.get('token');
  const verify = useMutation({
    mutationFn: () =>
      request(
        '/auth/email/verify',
        z.object({ status: z.literal('verified') }),
        { method: 'POST', body: JSON.stringify({ token }) },
      ),
    onSuccess: () => {
      setParams({}, { replace: true });
      void client.invalidateQueries({ queryKey: ['/account/email'] });
    },
  });
  return (
    <main className="standalone section-stack">
      <div className="flex justify-end">
        <PreferencesControls />
      </div>
      <Heading title={t('验证邮箱')} artwork="verify-email">
        {t('确认将此邮箱用于你的 PluginPocket 账号。')}
      </Heading>
      {verify.isSuccess ? (
        <p role="status">{t('邮箱验证成功。')}</p>
      ) : token ? (
        <>
          <Button disabled={verify.isPending} onClick={() => verify.mutate()}>
            {verify.isPending ? t('正在验证…') : t('确认验证邮箱')}
          </Button>
          <ErrorNotice error={verify.error} />
        </>
      ) : (
        <p>{t('验证链接不完整，请重新发送验证邮件。')}</p>
      )}
      <Link className="inline-action" to="/settings">
        {t('返回个人设置')}
      </Link>
    </main>
  );
}
