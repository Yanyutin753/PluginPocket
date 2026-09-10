import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';

const emailSchema = z.object({
  email: z.string().nullable(),
  verified_at: z.iso.datetime({ offset: true }).nullable(),
  configured: z.boolean(),
});
export default function SettingsPage() {
  const { t } = useI18n();
  const client = useQueryClient();
  const email = useQuery({
    queryKey: ['/account/email'],
    queryFn: ({ signal }) => request('/account/email', emailSchema, { signal }),
    retry: false,
  });
  const send = useMutation({
    mutationFn: (address: string) =>
      request(
        '/account/email/request',
        z.object({ status: z.literal('sent') }),
        { method: 'POST', body: JSON.stringify({ email: address }) },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/account/email'] });
    },
  });
  return (
    <>
      <Heading title={t('个人设置')}>
        {t('管理账号的联系方式和验证状态。')}
      </Heading>
      <ErrorNotice error={email.error} retry={() => void email.refetch()} />
      {email.isPending && <Loading variant="form" />}
      {email.data && (
        <section className="editor-panel">
          <h2>{t('邮箱验证')}</h2>
          <p>
            {email.data.email
              ? `${email.data.email} · ${email.data.verified_at ? t('已验证') : t('未验证')}`
              : t('还没有绑定邮箱。')}
          </p>
          {!email.data.configured && <p>{t('邮件服务尚未配置')}</p>}
          <form
            onSubmit={(event) => {
              event.preventDefault();
              send.mutate(
                String(new FormData(event.currentTarget).get('email')),
              );
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="email-address">{t('邮箱地址')}</FieldLabel>
                <Input
                  id="email-address"
                  type="email"
                  name="email"
                  required
                  defaultValue={email.data.email ?? ''}
                  autoComplete="email"
                  disabled={!email.data.configured || send.isPending}
                />
              </Field>
              <ErrorNotice error={send.error} />
              {send.isSuccess && (
                <p role="status">
                  {t('验证邮件已发送，请检查收件箱并点击验证链接。')}
                </p>
              )}
              <Button
                disabled={!email.data.configured || send.isPending}
                type="submit"
              >
                {send.isPending ? t('正在发送…') : t('发送验证邮件')}
              </Button>
            </FieldGroup>
          </form>
        </section>
      )}
    </>
  );
}
