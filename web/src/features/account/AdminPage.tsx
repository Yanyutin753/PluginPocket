import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Pagination } from '@/components/Pagination';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import {
  accountQuery,
  billingRolesQuery,
  multiplierText,
  number,
  request,
  resultSchema,
  type User,
  userSchema,
} from './api';
import { ErrorNotice, Heading, Loading } from './shared';
import { usePagedList } from './usePagedList';

function Adjustment({ user, close }: { user: User; close: () => void }) {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const [delta, setDelta] = useState('');
  const [note, setNote] = useState('');
  const [operation, setOperation] = useState<{
    delta: number;
    note: string;
    idempotency_key: string;
  } | null>(null);
  const adjust = useMutation({
    mutationFn: (data: {
      delta: number;
      note: string;
      idempotency_key: string;
    }) =>
      request(`/admin/users/${user.id}/balance`, resultSchema, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/users'] });
      void client.invalidateQueries({ queryKey: ['account'] });
    },
  });
  return (
    <SidePanel
      title={t('调整 {value1} 的余额', { value1: user.username })}
      onClose={close}
      locked={adjust.isPending}
    >
      <section
        className="adjustment-panel"
        aria-label={t('调整 {value1} 的余额', { value1: user.username })}
      >
        <h2>{t('调整 {value1} 的余额', { value1: user.username })}</h2>
        {adjust.isSuccess ? (
          <>
            <p role="status">
              {t('余额已调整，当前额度 {credits}。', {
                credits: number(adjust.data.user.balance, locale),
              })}
            </p>
            <Button variant="outline" onClick={close}>
              {t('完成')}
            </Button>
          </>
        ) : (
          <form
            className="form-stack"
            onSubmit={(event) => {
              event.preventDefault();
              const next = operation ?? {
                delta: Number(delta),
                note,
                idempotency_key: crypto.randomUUID(),
              };
              setOperation(next);
              adjust.mutate(next);
            }}
          >
            <p>{t('正数增加额度，负数扣减额度。每次调整都会保留备注。')}</p>
            <FieldGroup className="min-w-0 flex-1">
              <Field className="field">
                <FieldLabel htmlFor="delta">{t('调整额度')}</FieldLabel>
                <Input
                  id="delta"
                  type="number"
                  step="1"
                  required
                  value={delta}
                  disabled={Boolean(operation)}
                  onChange={(event) => setDelta(event.target.value)}
                />
              </Field>
              <Field className="field">
                <FieldLabel htmlFor="note">{t('备注')}</FieldLabel>
                <Input
                  id="note"
                  required
                  maxLength={500}
                  value={note}
                  disabled={Boolean(operation)}
                  onChange={(event) => setNote(event.target.value)}
                />
              </Field>
            </FieldGroup>
            <ErrorNotice error={adjust.error} />
            {operation && adjust.isError && (
              <p>
                {t('请求结果未确认。重试将沿用本次操作编号，避免重复调账。')}
              </p>
            )}
            <div className="action-row">
              <Button
                type="submit"
                disabled={
                  adjust.isPending ||
                  Number(delta) === 0 ||
                  !Number.isSafeInteger(Number(delta))
                }
              >
                {adjust.isPending ? t('正在调账…') : t('确认调账')}
              </Button>
              <Button
                variant="outline"
                type="button"
                disabled={adjust.isPending}
                onClick={close}
              >
                {t('关闭')}
              </Button>
            </div>
          </form>
        )}
      </section>
    </SidePanel>
  );
}
export default function AdminPage() {
  const { t, locale } = useI18n();
  const account = useQuery(accountQuery);
  const permitted = account.data?.user.role === 'admin';
  const users = usePagedList('/admin/users', userSchema, permitted);
  const roles = useQuery({ ...billingRolesQuery, enabled: permitted });
  const assignRole = useMutation({
    mutationFn: (input: { id: number; role: string }) =>
      request(`/admin/users/${input.id}`, resultSchema, {
        method: 'PATCH',
        body: JSON.stringify({ billing_role: input.role }),
      }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/users'] });
    },
  });
  const [selected, setSelected] = useState<User | null>(null);
  const client = useQueryClient();
  const [confirmUser, setConfirmUser] = useState<User | null>(null);
  const toggleUser = useMutation({
    mutationFn: (user: User) =>
      request(`/admin/users/${user.id}`, resultSchema, {
        method: 'PATCH',
        body: JSON.stringify({ enabled: !user.enabled }),
      }),
    onSuccess: () => {
      setConfirmUser(null);
      void client.invalidateQueries({ queryKey: ['/admin/users'] });
    },
  });
  if (!permitted)
    return (
      <Heading title={t('没有管理员权限')}>
        {t('请使用有权限的账号访问此页面。')}
      </Heading>
    );
  return (
    <>
      <Heading title={t('用户管理')} artwork="admin-users">
        {t('查看账号与额度，记录每一笔人工调整。')}
      </Heading>
      <ErrorNotice error={users.error} retry={() => void users.retry()} />
      {users.isPending && <Loading />}
      <ErrorNotice error={toggleUser.error} />
      <ErrorNotice error={assignRole.error} />
      {confirmUser && (
        <section className="editor-panel">
          <h2>
            {confirmUser.enabled ? t('停用') : t('启用')} {confirmUser.username}
          </h2>
          <p>
            {confirmUser.enabled
              ? t('停用后，此账号将无法登录或调用工具。')
              : t('此账号将恢复登录和调用权限。')}
          </p>
          <div className="action-row">
            <Button
              disabled={toggleUser.isPending}
              onClick={() => toggleUser.mutate(confirmUser)}
            >
              {confirmUser.enabled ? t('确认停用') : t('确认启用')}
            </Button>
            <Button
              variant="outline"
              disabled={toggleUser.isPending}
              onClick={() => setConfirmUser(null)}
            >
              {t('取消')}
            </Button>
          </div>
        </section>
      )}
      {selected && (
        <Adjustment
          key={selected.id}
          user={selected}
          close={() => setSelected(null)}
        />
      )}
      {users.isSuccess && users.items.length === 0 ? (
        <p>{t('暂无用户。')}</p>
      ) : (
        users.data && (
          <section
            className="table-scroll"
            ref={users.listRef}
            aria-label={t('用户表格')}
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Horizontal tables need keyboard scrolling.
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  <th scope="col">{t('用户')}</th>
                  <th scope="col">{t('角色')}</th>
                  <th scope="col">{t('计费角色')}</th>
                  <th scope="col">{t('状态')}</th>
                  <th scope="col" className="numeric">
                    {t('可用额度')}
                  </th>
                  <th scope="col">{t('操作')}</th>
                </tr>
              </thead>
              <tbody>
                {users.items.map((user) => (
                  <tr key={user.id}>
                    <td data-label={t('用户')} className="cell-wrap">
                      {user.username}
                    </td>
                    <td data-label={t('角色')}>
                      {user.role === 'admin' ? t('管理员') : t('用户')}
                    </td>
                    <td data-label={t('计费角色')}>
                      <Select
                        aria-label={t('设置 {value1} 的计费角色', {
                          value1: user.username,
                        })}
                        value={user.billing_role ?? 'default'}
                        disabled={assignRole.isPending}
                        options={(roles.data?.items ?? []).map((role) => ({
                          value: role.name,
                          label: `${role.name} ${multiplierText(role.multiplier_bp)}`,
                        }))}
                        onValueChange={(value) =>
                          assignRole.mutate({ id: user.id, role: value })
                        }
                      />
                    </td>
                    <td data-label={t('状态')}>
                      <span
                        className="status-badge"
                        data-tone={user.enabled ? 'success' : 'neutral'}
                      >
                        {user.enabled ? t('正常') : t('已停用')}
                      </span>
                    </td>
                    <td data-label={t('可用额度')} className="numeric">
                      {number(user.balance, locale)}
                    </td>
                    <td data-label={t('操作')} className="cell-actions">
                      <Button
                        variant="outline"
                        disabled={Boolean(selected)}
                        onClick={() => setSelected(user)}
                      >
                        {t('调整余额')}
                      </Button>
                      {user.id !== account.data?.user.id && (
                        <Button
                          variant="outline"
                          aria-label={`${user.enabled ? t('停用') : t('启用')} ${user.username}`}
                          onClick={() => {
                            toggleUser.reset();
                            setConfirmUser(user);
                          }}
                        >
                          {user.enabled ? t('停用') : t('启用')}
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        )
      )}
      <Pagination label={t('用户')} {...users.pagination} />
    </>
  );
}
