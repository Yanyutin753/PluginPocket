import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';

const schema = z.object({
  item: z.object({
    revision: z.number().int().nonnegative(),
    initial_credits: z.number().int().min(0).max(1_000_000_000_000),
    access_token_seconds: z.number().int().min(60).max(86400).default(900),
    refresh_token_seconds: z
      .number()
      .int()
      .min(60)
      .max(31536000)
      .default(604800),
    github_enabled: z.boolean(),
    github_client_id: z.string(),
    github_org: z.string(),
    github_client_secret_set: z.boolean(),
    smtp_enabled: z.boolean(),
    smtp_address: z.string(),
    smtp_from: z.string(),
    smtp_username: z.string(),
    smtp_password_set: z.boolean(),
  }),
  secret_writes_available: z.boolean(),
});
const queryKey = ['/admin/settings'];
const fields = {
  github: [
    ['github_client_id', 'GitHub Client ID'],
    ['github_org', 'GitHub 组织限制'],
  ],
  smtp: [
    ['smtp_address', 'SMTP 地址'],
    ['smtp_from', '邮件发件人'],
    ['smtp_username', 'SMTP 用户名'],
  ],
} as const;

export default function SystemSettingsPage() {
  const { t } = useI18n();
  const client = useQueryClient();
  const settings = useQuery({
    queryKey,
    queryFn: ({ signal }) => request('/admin/settings', schema, { signal }),
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      request('/admin/settings', schema, {
        method: 'PATCH',
        body: JSON.stringify(body),
      }),
    onSuccess: (data) => {
      client.setQueryData(queryKey, data);
      void client.invalidateQueries({ queryKey: ['meta'] });
      void client.invalidateQueries({ queryKey: ['/account/email'] });
    },
  });
  const item = settings.data?.item;
  return (
    <>
      <Heading title="系统配置" artwork="admin-settings">
        保存后对新请求生效，多个服务副本共享配置。
      </Heading>
      <ErrorNotice
        error={settings.error}
        retry={() => void settings.refetch()}
      />
      {settings.isPending && <Loading variant="form" />}
      {item && (
        <section className="section-stack">
          <dl className="settings-summary">
            <div>
              <dt>{t('注册赠送额度')}</dt>
              <dd>{item.initial_credits}</dd>
            </div>
            <div>
              <dt>{t('GitHub 登录')}</dt>
              <dd>{item.github_enabled ? t('启用') : t('停用')}</dd>
            </div>
            <div>
              <dt>{t('邮件服务')}</dt>
              <dd>{item.smtp_enabled ? t('启用') : t('停用')}</dd>
            </div>
          </dl>
          <SidePanel
            title={t('编辑系统配置')}
            trigger={t('编辑系统配置')}
            locked={save.isPending}
          >
            <form
              key={item.revision}
              className="max-w-3xl"
              onSubmit={(event) => {
                event.preventDefault();
                const data = new FormData(event.currentTarget);
                const body: Record<string, unknown> = {
                  revision: item.revision,
                  initial_credits: Number(data.get('initial_credits')),
                  access_token_seconds: Number(
                    data.get('access_token_seconds'),
                  ),
                  refresh_token_seconds: Number(
                    data.get('refresh_token_seconds'),
                  ),
                  github_enabled: data.get('github_enabled') === 'true',
                  smtp_enabled: data.get('smtp_enabled') === 'true',
                };
                for (const [key] of [...fields.github, ...fields.smtp])
                  body[key] = String(data.get(key) ?? '').trim();
                for (const key of ['github_client_secret', 'smtp_password']) {
                  const value = data.get(key);
                  if (data.get(`clear_${key}`) === 'on') body[key] = '';
                  else if (typeof value === 'string' && value.length > 0)
                    body[key] = value;
                }
                save.mutate(body);
              }}
            >
              <FieldSet disabled={save.isPending}>
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="initial_credits">
                      {t('注册赠送额度')}
                    </FieldLabel>
                    <Input
                      id="initial_credits"
                      name="initial_credits"
                      type="number"
                      min="0"
                      max="1000000000000"
                      step="1"
                      required
                      defaultValue={item.initial_credits}
                    />
                    <p>{t('仅影响之后注册的新账号，不修改已有余额。')}</p>
                  </Field>
                  <FieldSet>
                    <FieldLegend>{t('浏览器登录有效期')}</FieldLegend>
                    <p>
                      {t(
                        '保存后各副本立即采用新期限签发凭据；已有 RT 到期时间不变，刷新 AT 不延长 RT。',
                      )}
                    </p>
                    <FieldGroup>
                      {(
                        [
                          ['access_token_seconds', 'AT 有效期（秒）', 86400],
                          [
                            'refresh_token_seconds',
                            'RT 有效期（秒）',
                            31536000,
                          ],
                        ] as const
                      ).map(([key, label, max]) => (
                        <Field key={key}>
                          <FieldLabel htmlFor={key}>{t(label)}</FieldLabel>
                          <Input
                            id={key}
                            name={key}
                            type="number"
                            min="60"
                            max={max}
                            step="1"
                            required
                            defaultValue={item[key]}
                          />
                        </Field>
                      ))}
                    </FieldGroup>
                  </FieldSet>
                  {(['github', 'smtp'] as const).map((group) => {
                    const secret =
                      group === 'github'
                        ? 'github_client_secret'
                        : 'smtp_password';
                    const secretLabel =
                      group === 'github' ? 'GitHub Client Secret' : 'SMTP 密码';
                    const enabled =
                      group === 'github' ? 'github_enabled' : 'smtp_enabled';
                    return (
                      <FieldSet key={group}>
                        <FieldLegend>
                          {t(group === 'github' ? 'GitHub 登录' : '邮件服务')}
                        </FieldLegend>
                        <FieldGroup>
                          <Field>
                            <FieldLabel htmlFor={enabled}>
                              {t(
                                group === 'github'
                                  ? '启用 GitHub 登录'
                                  : '启用邮件服务',
                              )}
                            </FieldLabel>
                            <Select
                              id={enabled}
                              name={enabled}
                              defaultValue={String(item[enabled])}
                              options={[
                                { value: 'false', label: t('停用') },
                                { value: 'true', label: t('启用') },
                              ]}
                            />
                          </Field>
                          {fields[group].map(([key, label]) => (
                            <Field key={key}>
                              <FieldLabel htmlFor={key}>{t(label)}</FieldLabel>
                              <Input
                                id={key}
                                name={key}
                                maxLength={512}
                                defaultValue={item[key]}
                                autoComplete="off"
                              />
                            </Field>
                          ))}
                          <Field>
                            <FieldLabel htmlFor={secret}>
                              {t(secretLabel)}
                            </FieldLabel>
                            <Input
                              id={secret}
                              name={secret}
                              type="password"
                              autoComplete="new-password"
                              maxLength={4096}
                              disabled={!settings.data?.secret_writes_available}
                            />
                            <p>
                              {t(
                                item[`${secret}_set`]
                                  ? '已设置密钥；留空保留，填写后替换。'
                                  : '尚未设置密钥。',
                              )}
                            </p>
                          </Field>
                          {item[`${secret}_set`] && (
                            <Field orientation="horizontal">
                              <Input
                                id={`clear_${secret}`}
                                name={`clear_${secret}`}
                                type="checkbox"
                                className="size-4"
                              />
                              <FieldLabel htmlFor={`clear_${secret}`}>
                                {t('清除已保存的密钥')} ({t(secretLabel)})
                              </FieldLabel>
                            </Field>
                          )}
                        </FieldGroup>
                      </FieldSet>
                    );
                  })}
                  {!settings.data?.secret_writes_available && (
                    <p>{t('部署未配置加密主密钥，暂不能保存新的集成密钥。')}</p>
                  )}
                  <ErrorNotice error={save.error} />
                  {save.isSuccess && (
                    <p role="status">
                      {t('配置已保存，新请求将使用更新后的配置。')}
                    </p>
                  )}
                  <div className="flex flex-wrap gap-3">
                    <Button type="submit" disabled={save.isPending}>
                      {t(save.isPending ? '正在保存…' : '保存系统配置')}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={save.isPending || settings.isFetching}
                      onClick={async () => {
                        const result = await settings.refetch();
                        if (result.isSuccess) save.reset();
                      }}
                    >
                      {t('重新加载配置')}
                    </Button>
                  </div>
                </FieldGroup>
              </FieldSet>
            </form>
          </SidePanel>
        </section>
      )}
      <p>
        {t(
          '数据库、Redis、监听地址、公开访问地址和加密主密钥由部署配置管理，修改后需要重启。',
        )}
      </p>
    </>
  );
}
