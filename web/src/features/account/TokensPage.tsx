import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { useState } from 'react';
import { useSearchParams } from 'react-router';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { teamSchema } from '../operations/api';
import { date, listOptions, request, tokenSchema } from './api';
import { ErrorNotice, Heading, Loading, More } from './shared';
import { usePagedList } from './usePagedList';
export default function TokensPage() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const [params] = useSearchParams();
  const [teamId, setTeamId] = useState(params.get('team_id') ?? '');
  const teams = useInfiniteQuery(listOptions('/account/teams', teamSchema));
  const listedTeams = teams.data?.pages.flatMap((page) => page.items) ?? [];
  const selectedTeam = useQuery({
    queryKey: [`/account/teams/${teamId}`],
    queryFn: ({ signal }) =>
      request(
        `/account/teams/${encodeURIComponent(teamId)}`,
        z.object({ item: teamSchema }),
        { signal },
      ),
    enabled:
      Boolean(teamId) &&
      !listedTeams.some((team) => String(team.id) === teamId),
    retry: false,
  });
  const availableTeams = [
    ...listedTeams,
    ...(selectedTeam.data &&
    !listedTeams.some((team) => String(team.id) === teamId)
      ? [selectedTeam.data.item]
      : []),
  ];
  const tokens = usePagedList('/account/tokens', tokenSchema);
  const [secret, setSecret] = useState('');
  const [copied, setCopied] = useState('');
  const [name, setName] = useState('');
  const [confirm, setConfirm] = useState<number | null>(null);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: ['/account/tokens'] });
    void client.invalidateQueries({ queryKey: ['account'] });
  };
  const create = useMutation({
    mutationFn: async (name: string) => {
      const data = await request(
        '/account/tokens',
        z.object({
          token: z.string().regex(/^ldt_[A-Za-z0-9_-]+$/),
          item: tokenSchema,
        }),
        {
          method: 'POST',
          body: JSON.stringify({
            name,
            ...(teamId ? { team_id: Number(teamId) } : {}),
          }),
        },
      );
      setSecret(data.token);
      setCopied('');
      setName('');
      // Keep the one-time secret out of both Query and Mutation caches.
    },
    onSuccess: refresh,
  });
  const revoke = useMutation({
    mutationFn: (id: number) =>
      request(`/account/tokens/${id}`, z.unknown(), { method: 'DELETE' }),
    onSuccess: () => {
      setConfirm(null);
      refresh();
    },
  });
  return (
    <>
      <Heading title={t('网关令牌')} artwork="tokens">
        {t('为不同设备分别命名。令牌只显示一次，撤销后立即失效。')}
      </Heading>
      <SidePanel
        title={t('创建令牌')}
        trigger={t('创建令牌')}
        locked={create.isPending || Boolean(secret)}
      >
        <form
          className="inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            create.mutate(name);
          }}
        >
          <FieldGroup className="token-fields grid min-w-0 flex-1 gap-4">
            <Field className="field">
              <FieldLabel htmlFor="token-name">{t('令牌名称')}</FieldLabel>
              <Input
                id="token-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t('例如：工作电脑')}
                required
                maxLength={80}
                disabled={create.isPending}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="token-wallet">{t('扣费钱包')}</FieldLabel>
              <Select
                id="token-wallet"
                value={teamId}
                onValueChange={setTeamId}
                disabled={create.isPending}
                options={[
                  { value: '', label: t('个人钱包') },
                  ...availableTeams.map((team) => ({
                    value: String(team.id),
                    label: team.name,
                  })),
                ]}
              />
            </Field>
            <More
              hasNext={teams.hasNextPage}
              pending={teams.isFetchingNextPage}
              onClick={() => void teams.fetchNextPage()}
            />
          </FieldGroup>
          <Button
            type="submit"
            disabled={
              create.isPending ||
              Boolean(secret) ||
              (Boolean(teamId) &&
                !availableTeams.some((team) => String(team.id) === teamId))
            }
          >
            {create.isPending ? t('正在创建…') : t('创建令牌')}
          </Button>
        </form>
        {teamId && (
          <ErrorNotice
            error={selectedTeam.error}
            retry={() => void selectedTeam.refetch()}
          />
        )}
        {teamId && (
          <ErrorNotice error={teams.error} retry={() => void teams.refetch()} />
        )}
        <ErrorNotice error={create.error} />
        {secret && (
          <section className="secret-panel" aria-label={t('新令牌')}>
            <h2>{t('请立即保存令牌')}</h2>
            <p>{t('离开此页面或隐藏后，将无法再次查看明文。')}</p>
            <code>{secret}</code>
            <div className="action-row">
              <Button
                variant="outline"
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(secret);
                    setCopied('令牌已复制');
                  } catch {
                    setCopied('无法自动复制，请选中令牌并手动复制。');
                  }
                }}
              >
                {t('复制令牌')}
              </Button>
              <Button
                onClick={() => {
                  setSecret('');
                  setCopied('');
                }}
              >
                {t('我已保存，隐藏令牌')}
              </Button>
            </div>
            {copied && <p role="status">{t(copied)}</p>}
          </section>
        )}
      </SidePanel>
      <ErrorNotice error={tokens.error} retry={() => void tokens.retry()} />
      {tokens.isPending && <Loading />}
      {tokens.isSuccess && tokens.items.length === 0 && (
        <section className="empty-state">
          <h2>{t('还没有令牌')}</h2>
          <p>{t('创建第一个令牌，让本地客户端连接网关。')}</p>
        </section>
      )}
      <ul className="record-list" ref={tokens.listRef} tabIndex={-1}>
        {tokens.items.map((token) => (
          <li key={token.id}>
            <div className="record-main">
              <h2>{token.name}</h2>
              <code>{token.prefix}…</code>
              <p>
                {t('创建于 {created} · 最近使用：{lastUsed}', {
                  created: date(token.created_at, locale),
                  lastUsed: date(token.last_used_at, locale),
                })}
              </p>
            </div>
            {token.revoked_at ? (
              <span className="status-badge" data-tone="neutral">
                {t('已撤销')}
              </span>
            ) : confirm === token.id ? (
              <div className="confirm-actions">
                <p>{t('撤销后，使用此令牌的客户端将无法连接。')}</p>
                <Button
                  variant="outline"
                  disabled={revoke.isPending}
                  onClick={() => {
                    setConfirm(null);
                    revoke.reset();
                  }}
                >
                  {t('取消')}
                </Button>
                <Button
                  disabled={revoke.isPending}
                  onClick={() => revoke.mutate(token.id)}
                >
                  {revoke.isPending ? t('正在撤销…') : t('确认撤销')}
                </Button>
                <ErrorNotice error={revoke.error} />
              </div>
            ) : (
              <Button
                variant="outline"
                aria-label={t('撤销 {value1}', { value1: token.name })}
                onClick={() => setConfirm(token.id)}
              >
                {t('撤销')}
              </Button>
            )}
          </li>
        ))}
      </ul>
      <Pagination label={t('令牌')} {...tokens.pagination} />
    </>
  );
}
