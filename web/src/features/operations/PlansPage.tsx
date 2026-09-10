import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { listOptions, number, request } from '../account/api';
import { ErrorNotice, Heading, Loading, More } from '../account/shared';
import { type Plan, planSchema, price } from './api';

function PlanEditor({ item, close }: { item: Plan | null; close: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const save = useMutation({
    mutationFn: (body: object) =>
      request(
        `/admin/plans${item ? `/${item.id}` : ''}`,
        z.object({ item: planSchema }),
        { method: item ? 'PATCH' : 'POST', body: JSON.stringify(body) },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/plans'] });
      void client.invalidateQueries({ queryKey: ['/plans'] });
      close();
    },
  });
  return (
    <section className="editor-panel">
      <h2>{item ? t('编辑套餐') : t('添加套餐')}</h2>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          save.mutate({
            name: data.get('name'),
            credits: Number(data.get('credits')),
            price_cents: Number(data.get('price')),
            currency: data.get('currency'),
            enabled: item?.enabled ?? true,
          });
        }}
      >
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="plan-name">{t('套餐名称')}</FieldLabel>
            <Input
              id="plan-name"
              name="name"
              required
              defaultValue={item?.name}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="plan-credits">{t('额度数量')}</FieldLabel>
            <Input
              id="plan-credits"
              name="credits"
              type="number"
              min="1"
              max="1000000000000"
              step="1"
              required
              defaultValue={item?.credits}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="plan-price">{t('价格（分）')}</FieldLabel>
            <Input
              id="plan-price"
              name="price"
              type="number"
              min="0"
              max="1000000000000"
              step="1"
              required
              defaultValue={item?.price_cents}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="currency">{t('币种')}</FieldLabel>
            <Input
              id="currency"
              name="currency"
              pattern="[A-Z]{3}"
              maxLength={3}
              required
              defaultValue={item?.currency ?? 'CNY'}
            />
          </Field>
          <ErrorNotice error={save.error} />
          <div className="action-row">
            <Button type="submit" disabled={save.isPending}>
              {t('保存套餐')}
            </Button>
            <Button
              variant="outline"
              type="button"
              disabled={save.isPending}
              onClick={close}
            >
              {t('取消')}
            </Button>
          </div>
        </FieldGroup>
      </form>
    </section>
  );
}
export default function PlansPage() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const plans = useInfiniteQuery(listOptions('/admin/plans', planSchema));
  const [editing, setEditing] = useState<Plan | null | undefined>();
  const toggle = useMutation({
    mutationFn: (item: Plan) =>
      request(`/admin/plans/${item.id}`, z.object({ item: planSchema }), {
        method: 'PATCH',
        body: JSON.stringify({
          name: item.name,
          credits: item.credits,
          price_cents: item.price_cents,
          currency: item.currency,
          enabled: !item.enabled,
        }),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/plans'] });
      void client.invalidateQueries({ queryKey: ['/plans'] });
    },
  });
  return (
    <>
      <Heading title={t('套餐管理')}>
        {t('套餐额度和价格由你定义。付款能力取决于支付服务配置。')}
      </Heading>
      {editing === undefined ? (
        <Button onClick={() => setEditing(null)}>{t('添加套餐')}</Button>
      ) : (
        <PlanEditor item={editing} close={() => setEditing(undefined)} />
      )}
      <ErrorNotice error={plans.error} retry={() => void plans.refetch()} />
      <ErrorNotice error={toggle.error} />
      {plans.isPending && <Loading />}
      {plans.isSuccess && !plans.data.pages[0].items.length && (
        <p className="empty-state">{t('暂时没有套餐。')}</p>
      )}
      <ul className="record-list">
        {plans.data?.pages
          .flatMap((page) => page.items)
          .map((item) => (
            <li key={item.id}>
              <div className="record-main">
                <h2>{item.name}</h2>
                <p>
                  {t('{credits} 额度 · {price}', {
                    credits: number(item.credits, locale),
                    price: price(item.price_cents, item.currency, locale),
                  })}
                </p>
                <p>{item.enabled ? t('在售') : t('已停用')}</p>
              </div>
              <div className="action-row">
                <Button
                  variant="outline"
                  aria-label={t('编辑 {value1}', { value1: item.name })}
                  disabled={editing !== undefined}
                  onClick={() => setEditing(item)}
                >
                  {t('编辑')}
                </Button>
                <Button
                  variant="outline"
                  aria-label={`${item.enabled ? t('停用') : t('启用')} ${item.name}`}
                  disabled={toggle.isPending || editing !== undefined}
                  onClick={() => toggle.mutate(item)}
                >
                  {item.enabled ? t('停用') : t('启用')}
                </Button>
              </div>
            </li>
          ))}
      </ul>
      <More
        hasNext={plans.hasNextPage}
        pending={plans.isFetchingNextPage}
        onClick={() => void plans.fetchNextPage()}
      />
    </>
  );
}
