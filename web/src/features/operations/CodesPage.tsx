import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { date, number, request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
import { codeSchema } from './api';
import { OneTimeCode } from './OneTimeCode';
export default function CodesPage() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const list = usePagedList('/admin/redemption-codes', codeSchema);
  const [code, setCode] = useState('');
  const create = useMutation({
    mutationFn: async (body: { credits: number; note: string }) => {
      const result = await request(
        '/admin/redemption-codes',
        z.object({ code: z.string().min(1), item: codeSchema }),
        { method: 'POST', body: JSON.stringify(body) },
      );
      setCode(result.code);
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/redemption-codes'] });
    },
  });
  return (
    <>
      <Heading artwork="admin-codes" title={t('兑换码管理')}>
        {t('生成单次兑换凭据，查看发放和兑换状态。')}
      </Heading>
      <SidePanel
        title={t('生成兑换码')}
        trigger={t('生成兑换码')}
        locked={create.isPending || Boolean(code)}
      >
        <form
          className="editor-panel compact-editor"
          onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            create.mutate({
              credits: Number(data.get('credits')),
              note: String(data.get('note')),
            });
          }}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="code-credits">{t('兑换额度')}</FieldLabel>
              <Input
                id="code-credits"
                name="credits"
                type="number"
                min="1"
                max="1000000000000"
                step="1"
                required
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="code-note">{t('备注')}</FieldLabel>
              <Input id="code-note" name="note" maxLength={500} required />
            </Field>
            <ErrorNotice error={create.error} />
            <Button type="submit" disabled={create.isPending || Boolean(code)}>
              {create.isPending ? t('正在生成…') : t('生成兑换码')}
            </Button>
          </FieldGroup>
        </form>
        {code && (
          <OneTimeCode
            title={t('兑换码')}
            code={code}
            hide={() => setCode('')}
          />
        )}
      </SidePanel>
      <ErrorNotice error={list.error} retry={() => void list.retry()} />
      {list.isPending && <Loading />}
      {list.isSuccess && !list.items.length && (
        <p className="empty-state">{t('还没有生成兑换码。')}</p>
      )}
      <ul className="record-list" ref={list.listRef} tabIndex={-1}>
        {list.items.map((item) => (
          <li key={item.id}>
            <div className="record-main">
              <h2 className="numeric">
                {t('{credits} 额度', {
                  credits: number(item.credits, locale),
                })}
              </h2>
              <p>{item.note}</p>
              <p>{date(item.created_at, locale)}</p>
            </div>
            <span
              className="status-badge"
              data-tone={item.redeemed_at ? 'neutral' : 'success'}
            >
              {item.redeemed_at
                ? t('已兑换 · {value1}', {
                    value1: date(item.redeemed_at, locale),
                  })
                : t('未兑换')}
            </span>
          </li>
        ))}
      </ul>
      <Pagination label={t('兑换码')} {...list.pagination} />
    </>
  );
}
