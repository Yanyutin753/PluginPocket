import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useSearchParams } from 'react-router';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice, Heading } from '../account/shared';
export default function DevicePage() {
  const { t } = useI18n();
  const [params] = useSearchParams();
  const [code, setCode] = useState(params.get('user_code') ?? '');
  const approve = useMutation({
    mutationFn: () =>
      request('/account/devices/approve', z.unknown(), {
        method: 'POST',
        body: JSON.stringify({ user_code: code }),
      }),
  });
  return (
    <>
      <Heading title={t('设备授权')} artwork="device">
        {t('将本地 CLI 或桌面客户端连接到你的账号。')}
      </Heading>
      <section className="editor-panel device-authorization-panel">
        <p>
          {t(
            '请确认下面的授权码与你自己设备上显示的一致。批准后，该设备可以使用你的额度调用工具。',
          )}
        </p>
        {approve.isSuccess ? (
          <p role="status">{t('设备已获授权，请回到本地设备完成登录。')}</p>
        ) : (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              approve.mutate();
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="device-code">{t('设备授权码')}</FieldLabel>
                <Input
                  id="device-code"
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  required
                  autoComplete="off"
                  maxLength={32}
                />
              </Field>
              <ErrorNotice error={approve.error} />
              <Button type="submit" disabled={approve.isPending}>
                {approve.isPending ? t('正在批准…') : t('批准此设备')}
              </Button>
            </FieldGroup>
          </form>
        )}
      </section>
    </>
  );
}
