import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { z } from 'zod';
import { SidePanel } from '@/components/SidePanel';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useI18n } from '@/i18n';
import { request } from '../account/api';
import { ErrorNotice } from '../account/shared';
import { type Team, teamSchema } from './api';
export function TeamSettings({
  team,
  members,
  selfId,
  transferred,
}: {
  team: Team;
  members: Array<{ user_id: number; username: string }>;
  selfId: number | undefined;
  transferred: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [owner, setOwner] = useState('');
  const [confirm, setConfirm] = useState(false);
  const save = useMutation({
    mutationFn: (body: {
      name?: string;
      seat_limit?: number;
      owner_user_id?: number;
    }) =>
      request(`/account/teams/${team.id}`, z.object({ item: teamSchema }), {
        method: 'PATCH',
        body: JSON.stringify(body),
      }),
    onSuccess: (data, variables) => {
      if (variables.owner_user_id) transferred();
      client.setQueryData([`/account/teams/${team.id}`], data);
      void client.invalidateQueries({ queryKey: ['/account/teams'] });
      void client.invalidateQueries({
        queryKey: [`/account/teams/${team.id}/members`],
      });
    },
  });
  return (
    <SidePanel
      title={t('团队设置')}
      trigger={t('团队设置')}
      locked={save.isPending}
    >
      <section className="editor-panel">
        <h2>{t('团队设置')}</h2>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            save.mutate({
              name: String(data.get('name')),
              seat_limit: Number(data.get('seats')),
            });
          }}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="edit-team-name">
                {t('团队显示名称')}
              </FieldLabel>
              <Input
                id="edit-team-name"
                name="name"
                defaultValue={team.name}
                required
                maxLength={80}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="team-seats">{t('团队席位')}</FieldLabel>
              <Input
                id="team-seats"
                name="seats"
                type="number"
                min="1"
                max="1000"
                step="1"
                defaultValue={team.seat_limit}
                required
              />
            </Field>
            <Button type="submit" disabled={save.isPending}>
              {t('保存团队设置')}
            </Button>
          </FieldGroup>
        </form>
        <ErrorNotice error={save.error} />
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="team-owner">{t('新的所有者')}</FieldLabel>
            <Select
              id="team-owner"
              value={owner}
              onValueChange={(value) => {
                setOwner(value);
                setConfirm(false);
              }}
              options={[
                { value: '', label: t('请选择成员') },
                ...members
                  .filter((member) => member.user_id !== selfId)
                  .map((member) => ({
                    value: String(member.user_id),
                    label: member.username,
                  })),
              ]}
            />
          </Field>
          {confirm ? (
            <div className="confirm-actions">
              <p>{t('交接后你将成为普通成员，团队管理权限会立即转移。')}</p>
              <Button
                disabled={save.isPending}
                onClick={() => save.mutate({ owner_user_id: Number(owner) })}
              >
                {t('确认交接')}
              </Button>
              <Button
                variant="outline"
                disabled={save.isPending}
                onClick={() => setConfirm(false)}
              >
                {t('取消')}
              </Button>
            </div>
          ) : (
            <Button
              variant="outline"
              disabled={!owner || save.isPending}
              onClick={() => setConfirm(true)}
            >
              {t('交接所有权')}
            </Button>
          )}
        </FieldGroup>
      </section>
    </SidePanel>
  );
}
