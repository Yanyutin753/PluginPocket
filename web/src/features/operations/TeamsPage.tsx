import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { z } from 'zod';
import { Pagination } from '@/components/Pagination';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { accountQuery, date, number, request } from '../account/api';
import { ErrorNotice, Heading, Loading } from '../account/shared';
import { usePagedList } from '../account/usePagedList';
import { memberSchema, type Team, teamSchema } from './api';
import { OneTimeCode } from './OneTimeCode';
import { TeamSettings } from './TeamSettings';

function TeamList() {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const teams = usePagedList('/account/teams', teamSchema);
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const refresh = () => {
    void client.invalidateQueries({ queryKey: ['/account/teams'] });
  };
  const create = useMutation({
    mutationFn: () =>
      request('/account/teams', z.object({ item: teamSchema }), {
        method: 'POST',
        body: JSON.stringify({ name }),
      }),
    onSuccess: () => {
      setName('');
      refresh();
    },
  });
  const join = useMutation({
    mutationFn: () =>
      request('/account/team-invites/accept', z.object({ item: teamSchema }), {
        method: 'POST',
        body: JSON.stringify({ code }),
      }),
    onSuccess: () => {
      setCode('');
      refresh();
    },
  });
  return (
    <>
      <Heading title={t('我的团队')} artwork="teams">
        {t('让成员共用额度，并按成员查看工具调用。')}
      </Heading>
      <div className="two-column">
        <form
          className="editor-panel"
          onSubmit={(event) => {
            event.preventDefault();
            create.mutate();
          }}
        >
          <h2>{t('新建团队')}</h2>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="team-name">{t('团队名称')}</FieldLabel>
              <Input
                id="team-name"
                required
                maxLength={80}
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <ErrorNotice error={create.error} />
            <Button disabled={create.isPending} type="submit">
              {t('创建团队')}
            </Button>
          </FieldGroup>
        </form>
        <form
          className="editor-panel"
          onSubmit={(event) => {
            event.preventDefault();
            join.mutate();
          }}
        >
          <h2>{t('接受邀请')}</h2>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="team-invite">{t('团队邀请码')}</FieldLabel>
              <Input
                id="team-invite"
                required
                value={code}
                onChange={(event) => setCode(event.target.value)}
                autoComplete="off"
              />
            </Field>
            <ErrorNotice error={join.error} />
            <Button disabled={join.isPending} type="submit">
              {t('加入团队')}
            </Button>
          </FieldGroup>
        </form>
      </div>
      <ErrorNotice error={teams.error} retry={() => void teams.retry()} />
      {teams.isPending && <Loading />}
      {teams.isSuccess && !teams.items.length && (
        <p className="empty-state">{t('还没有加入团队')}</p>
      )}
      <ul className="record-list" ref={teams.listRef} tabIndex={-1}>
        {teams.items.map((team) => (
          <li key={team.id}>
            <div className="record-main">
              <h2>{team.name}</h2>
              <p>
                {t('{role} · 共享额度 {credits} · {seats} 个席位', {
                  role: team.role === 'owner' ? t('所有者') : t('成员'),
                  credits: number(team.balance, locale),
                  seats: number(team.seat_limit, locale),
                })}
              </p>
            </div>
            <Link
              className="inline-action"
              aria-label={t('管理 {value1}', { value1: team.name })}
              to={`/teams/${team.id}`}
            >
              {t('管理团队')}
            </Link>
          </li>
        ))}
      </ul>
      <Pagination label={t('团队')} {...teams.pagination} />
    </>
  );
}
function Funding({ team }: { team: Team }) {
  const { t, locale } = useI18n();
  const client = useQueryClient();
  const [credits, setCredits] = useState('');
  const [operation, setOperation] = useState<{
    credits: number;
    idempotency_key: string;
  } | null>(null);
  const fund = useMutation({
    mutationFn: (body: { credits: number; idempotency_key: string }) =>
      request(
        `/account/teams/${team.id}/fund`,
        z.object({ item: teamSchema }),
        { method: 'POST', body: JSON.stringify(body) },
      ),
    onSuccess: (data) => {
      client.setQueryData([`/account/teams/${team.id}`], data);
      void client.invalidateQueries({ queryKey: ['/account/teams'] });
      void client.invalidateQueries({ queryKey: ['account'] });
      void client.invalidateQueries({ queryKey: ['/account/ledger'] });
    },
  });
  return (
    <section className="editor-panel">
      <h2>{t('转入团队额度')}</h2>
      <p>{t('从你的个人钱包转入。转入后由团队共同使用。')}</p>
      {fund.isSuccess ? (
        <>
          <p role="status">
            {t('已转入 {credits} 额度。', {
              credits: number(operation?.credits ?? 0, locale),
            })}
          </p>
          <Button
            variant="outline"
            onClick={() => {
              fund.reset();
              setOperation(null);
              setCredits('');
            }}
          >
            {t('再转入一笔')}
          </Button>
        </>
      ) : (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            const next = operation ?? {
              credits: Number(credits),
              idempotency_key: crypto.randomUUID(),
            };
            setOperation(next);
            fund.mutate(next);
          }}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="team-fund">{t('转入额度')}</FieldLabel>
              <Input
                id="team-fund"
                type="number"
                min="1"
                max="1000000000000"
                step="1"
                required
                disabled={Boolean(operation)}
                value={credits}
                onChange={(event) => setCredits(event.target.value)}
              />
            </Field>
            <ErrorNotice error={fund.error} />
            {fund.isError && (
              <p>{t('重试沿用同一个操作编号，避免重复转入。')}</p>
            )}
            <Button disabled={fund.isPending} type="submit">
              {t('确认转入')}
            </Button>
          </FieldGroup>
        </form>
      )}
    </section>
  );
}
function TeamDetail({ id }: { id: string }) {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const client = useQueryClient();
  const account = useQuery(accountQuery);
  const team = useQuery({
    queryKey: [`/account/teams/${id}`],
    queryFn: ({ signal }) =>
      request(`/account/teams/${id}`, z.object({ item: teamSchema }), {
        signal,
      }),
    retry: false,
  });
  const members = usePagedList(
    `/account/teams/${id}/members`,
    memberSchema,
    team.isSuccess,
  );
  const [invite, setInvite] = useState<{
    code: string;
    expires_at: string;
  } | null>(null);
  const [transferred, setTransferred] = useState(false);
  const [removing, setRemoving] = useState<number | null>(null);
  const invitation = useMutation({
    mutationFn: async () => {
      setInvite(
        await request(
          `/account/teams/${id}/invites`,
          z.object({
            code: z.string(),
            expires_at: z.iso.datetime({ offset: true }),
          }),
          { method: 'POST', body: '{}' },
        ),
      );
    },
  });
  const remove = useMutation({
    mutationFn: (userId: number) =>
      request(`/account/teams/${id}/members/${userId}`, z.unknown(), {
        method: 'DELETE',
      }),
    onSuccess: (_data, userId) => {
      setRemoving(null);
      void client.invalidateQueries({
        queryKey: [`/account/teams/${id}/members`],
      });
      void client.invalidateQueries({ queryKey: ['/account/teams'] });
      if (userId === account.data?.user.id) {
        client.removeQueries({ queryKey: [`/account/teams/${id}`] });
        navigate('/teams');
      }
    },
  });
  if (team.isPending) return <Loading variant="page" />;
  if (team.error)
    return <ErrorNotice error={team.error} retry={() => void team.refetch()} />;
  const item = team.data.item;
  return (
    <>
      <Heading title={item.name} translateTitle={false} artwork="team-detail">
        {t('共享额度 {credits} · {seats} 个席位', {
          credits: number(item.balance, locale),
          seats: number(item.seat_limit, locale),
        })}
      </Heading>
      <div className="action-row">
        <Link className="inline-action" to="/teams">
          {t('所有团队')}
        </Link>
        <Link className="inline-action" to={`/teams/${id}/usage`}>
          {t('团队用量与导出')}
        </Link>
        <Link className="inline-action" to={`/tokens?team_id=${id}`}>
          {t('创建团队令牌')}
        </Link>
      </div>
      {item.role === 'owner' && (
        <div className="two-column">
          <Funding team={item} />
          <section className="editor-panel">
            <h2>{t('邀请成员')}</h2>
            <p>{t('邀请码仅可使用一次，24 小时内有效。')}</p>
            <Button
              disabled={invitation.isPending || Boolean(invite)}
              onClick={() => invitation.mutate()}
            >
              {t('生成邀请')}
            </Button>
            <ErrorNotice error={invitation.error} />
            {invite && (
              <OneTimeCode
                title={t('邀请码')}
                code={invite.code}
                expires={date(invite.expires_at, locale)}
                hide={() => setInvite(null)}
              />
            )}
          </section>
        </div>
      )}
      {transferred && <p role="status">{t('团队所有权已交接')}</p>}
      {item.role === 'owner' && (
        <TeamSettings
          team={item}
          members={members.data?.pages.flatMap((page) => page.items) ?? []}
          selfId={account.data?.user.id}
          transferred={() => setTransferred(true)}
        />
      )}
      <section className="section-stack">
        <h2>{t('团队成员')}</h2>
        <ErrorNotice error={members.error} retry={() => void members.retry()} />
        <ErrorNotice error={remove.error} />
        {members.isPending && <Loading />}
        <ul className="record-list" ref={members.listRef} tabIndex={-1}>
          {members.items.map((member) => {
            const self = member.user_id === account.data?.user.id;
            return (
              <li key={member.id}>
                <div className="record-main">
                  <h3>
                    {member.username}
                    {self ? t('（你）') : ''}
                  </h3>
                  <p>{member.role === 'owner' ? t('所有者') : t('成员')}</p>
                </div>
                {(item.role === 'owner' || self) &&
                  (removing === member.user_id ? (
                    <div className="confirm-actions">
                      <p>
                        {self
                          ? t(
                              '退出后将无法访问团队额度与工具。最后一位所有者需要先交接团队。',
                            )
                          : t('移除后，此成员的团队令牌将无法使用团队额度。')}
                      </p>
                      <Button
                        disabled={remove.isPending}
                        onClick={() => remove.mutate(member.user_id)}
                      >
                        {self ? t('确认退出') : t('确认移除')}
                      </Button>
                      <Button
                        variant="outline"
                        onClick={() => setRemoving(null)}
                        disabled={remove.isPending}
                      >
                        {t('取消')}
                      </Button>
                    </div>
                  ) : (
                    <Button
                      variant="outline"
                      aria-label={
                        self
                          ? t('退出团队')
                          : t('移除 {value1}', { value1: member.username })
                      }
                      onClick={() => {
                        remove.reset();
                        setRemoving(member.user_id);
                      }}
                    >
                      {self ? t('退出团队') : t('移除成员')}
                    </Button>
                  ))}
              </li>
            );
          })}
        </ul>
        <Pagination label={t('成员')} {...members.pagination} />
      </section>
    </>
  );
}
export default function TeamsPage() {
  const { t } = useI18n();
  const { id } = useParams();
  return id ? (
    /^\d+$/.test(id) ? (
      <TeamDetail key={id} id={id} />
    ) : (
      <Heading title={t('团队不存在')} />
    )
  ) : (
    <TeamList />
  );
}
