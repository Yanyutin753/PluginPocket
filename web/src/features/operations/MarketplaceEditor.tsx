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
import { type MarketplaceItem, marketplaceItemSchema } from './api';

export function MarketplaceEditor({
  kind,
  item,
  items,
  close,
  saved,
}: {
  kind: 'skill' | 'bundle';
  item?: MarketplaceItem;
  items: MarketplaceItem[];
  close: () => void;
  saved: () => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [name, setName] = useState(item?.name ?? '');
  const [slug, setSlug] = useState(item?.slug ?? '');
  const [description, setDescription] = useState(item?.description ?? '');
  const [source, setSource] = useState(item?.spec?.source ?? 'inline');
  const [repo, setRepo] = useState(item?.spec?.repo ?? '');
  const [path, setPath] = useState(item?.spec?.path ?? '');
  const [content, setContent] = useState(item?.spec?.files?.['SKILL.md'] ?? '');
  const [includes, setIncludes] = useState(item?.spec?.includes ?? []);
  const save = useMutation({
    mutationKey: ['marketplace'],
    mutationFn: () =>
      request(
        `/admin/marketplace/${kind === 'skill' ? 'skills' : 'bundles'}`,
        z.object({ item: marketplaceItemSchema }),
        {
          method: 'POST',
          body: JSON.stringify({
            name: name.trim(),
            slug: slug.trim(),
            description,
            ...(kind === 'bundle'
              ? { includes }
              : {
                  source,
                  ...(source === 'github'
                    ? { repo: repo.trim(), path: path.trim() }
                    : { files: { ...item?.spec?.files, 'SKILL.md': content } }),
                }),
          }),
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
      void client.invalidateQueries({ queryKey: ['public-plugins'] });
      saved();
      close();
    },
  });
  return (
    <SidePanel
      title={t(
        item ? '编辑市场条目' : kind === 'skill' ? '创建技能' : '创建装备组',
      )}
      onClose={close}
      locked={save.isPending}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          save.mutate();
        }}
      >
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="entry-name">{t('名称')}</FieldLabel>
            <Input
              id="entry-name"
              required
              maxLength={80}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="entry-slug">{t('标识')}</FieldLabel>
            <Input
              id="entry-slug"
              required
              pattern="[a-zA-Z0-9_-]{3,32}"
              minLength={3}
              maxLength={32}
              disabled={!!item}
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
            />
            <p className="hint-text">
              {t('3–32 位字母、数字、短横线或下划线，用于安装命令。')}
            </p>
          </Field>
          <Field>
            <FieldLabel htmlFor="entry-description">{t('描述')}</FieldLabel>
            <textarea
              id="entry-description"
              rows={3}
              maxLength={2000}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </Field>
          {kind === 'skill' ? (
            <>
              <Field>
                <FieldLabel htmlFor="entry-source">{t('技能来源')}</FieldLabel>
                <Select
                  id="entry-source"
                  value={source}
                  onValueChange={(value) =>
                    setSource(value === 'github' ? 'github' : 'inline')
                  }
                  options={[
                    { value: 'inline', label: t('自定义内容') },
                    { value: 'github', label: 'GitHub' },
                  ]}
                />
              </Field>
              {source === 'inline' ? (
                <Field>
                  <FieldLabel htmlFor="entry-content">
                    {t('SKILL.md 内容')}
                  </FieldLabel>
                  <textarea
                    id="entry-content"
                    required
                    rows={14}
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                  />
                  <p className="hint-text">
                    {t('使用 Markdown 编写技能说明；编辑时保留已有附属文件。')}
                  </p>
                </Field>
              ) : (
                <>
                  <Field>
                    <FieldLabel htmlFor="entry-repo">
                      {t('GitHub 仓库')}
                    </FieldLabel>
                    <Input
                      id="entry-repo"
                      required
                      placeholder="owner/repo"
                      value={repo}
                      onChange={(e) => setRepo(e.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="entry-path">
                      {t('技能目录')}
                    </FieldLabel>
                    <Input
                      id="entry-path"
                      required
                      placeholder="skills/review"
                      value={path}
                      onChange={(e) => setPath(e.target.value)}
                    />
                    <p className="hint-text">
                      {t('填写包含 SKILL.md 的目录；同步失败时保留原条目。')}
                    </p>
                  </Field>
                </>
              )}
            </>
          ) : (
            <fieldset>
              <legend>{t('选择组件（最多 32 项）')}</legend>
              <p className="hint-text">
                {t('未提供 HTTP 端点的 MCP 暂不可加入装备组。')}
              </p>
              <div className="market-components">
                {items
                  .filter((entry) => entry.kind !== 'bundle')
                  .map((entry) => (
                    <label key={entry.slug} className="market-component">
                      <input
                        type="checkbox"
                        checked={includes.includes(entry.slug)}
                        disabled={
                          !includes.includes(entry.slug) &&
                          (includes.length >= 32 ||
                            (entry.kind === 'mcp' &&
                              entry.transport !== 'gateway' &&
                              !(entry.transport === 'http' && entry.endpoint)))
                        }
                        onChange={(e) =>
                          setIncludes(
                            e.target.checked
                              ? [...includes, entry.slug]
                              : includes.filter((slug) => slug !== entry.slug),
                          )
                        }
                      />
                      <span>
                        {entry.name}
                        <small>
                          {entry.kind === 'skill' ? t('技能') : 'MCP'} ·{' '}
                          {entry.slug}
                        </small>
                      </span>
                    </label>
                  ))}
              </div>
              {!items.some((entry) => entry.kind !== 'bundle') && (
                <p>{t('先创建技能或同步 MCP，再组合装备组。')}</p>
              )}
            </fieldset>
          )}
          <ErrorNotice error={save.error} />
          <div className="action-row">
            <Button
              disabled={
                save.isPending || (kind === 'bundle' && includes.length === 0)
              }
              type="submit"
            >
              {save.isPending
                ? t('正在保存…')
                : t(kind === 'skill' ? '保存技能' : '保存装备组')}
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
    </SidePanel>
  );
}

const recommendations = [
  {
    slug: 'superpowers-debugging',
    name: 'Systematic Debugging',
    repo: 'obra/superpowers',
    path: 'skills/systematic-debugging',
    description: '系统排障：定位根因、验证假设。',
  },
  {
    slug: 'superpowers-tdd',
    name: 'Test Driven Development',
    repo: 'obra/superpowers',
    path: 'skills/test-driven-development',
    description: '先写失败测试，再实现与重构。',
  },
  {
    slug: 'anthropic-frontend-design',
    name: 'Frontend Design',
    repo: 'anthropics/skills',
    path: 'skills/frontend-design',
    description: '构建有辨识度的前端界面。',
  },
];

export function RecommendedSkills({ close }: { close: () => void }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const sync = useMutation({
    mutationKey: ['marketplace'],
    mutationFn: (entry: (typeof recommendations)[number]) =>
      request(
        '/admin/marketplace/skills',
        z.object({ item: marketplaceItemSchema }),
        {
          method: 'POST',
          body: JSON.stringify({ ...entry, source: 'github' }),
        },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['/admin/marketplace'] });
    },
  });
  const syncAll = useMutation({
    mutationKey: ['marketplace'],
    mutationFn: async () => {
      sync.reset();
      const failed: string[] = [];
      for (const entry of recommendations) {
        try {
          await sync.mutateAsync(entry);
        } catch {
          failed.push(entry.name);
        }
      }
      sync.reset();
      return failed;
    },
  });
  return (
    <SidePanel
      title={t('精选 GitHub 技能')}
      onClose={close}
      locked={sync.isPending || syncAll.isPending}
    >
      <p>
        {t(
          '从维护者仓库同步完整技能目录；可重复同步。源码与许可请查看原仓库。',
        )}
      </p>
      <Button
        disabled={sync.isPending || syncAll.isPending}
        onClick={() => syncAll.mutate()}
      >
        {syncAll.isPending ? t('正在同步…') : t('一键同步全部')}
      </Button>
      {syncAll.isSuccess && (
        <div role="status">
          <p>
            {t('同步完成：成功 {success}，失败 {failed}', {
              success: recommendations.length - syncAll.data.length,
              failed: syncAll.data.length,
            })}
          </p>
          {syncAll.data.map((name) => (
            <p key={name}>
              {name} · {t('同步失败，请重试')}
            </p>
          ))}
        </div>
      )}
      <ErrorNotice error={sync.error} />
      {sync.isSuccess && (
        <p role="status">
          {t('已同步')} · {sync.variables.name}
        </p>
      )}
      <ul className="record-list">
        {recommendations.map((entry) => (
          <li key={entry.slug}>
            <div className="record-main">
              <h3>{entry.name}</h3>
              <p>{t(entry.description)}</p>
              <a
                href={`https://github.com/${entry.repo}/tree/HEAD/${entry.path}`}
                target="_blank"
                rel="noreferrer"
              >
                {entry.repo}
              </a>
            </div>
            <Button
              disabled={sync.isPending || syncAll.isPending}
              aria-label={t('同步 {value1}', { value1: entry.name })}
              onClick={() => {
                syncAll.reset();
                sync.mutate(entry);
              }}
            >
              {sync.isPending && sync.variables.slug === entry.slug
                ? t('正在同步…')
                : t('同步')}
            </Button>
          </li>
        ))}
      </ul>
    </SidePanel>
  );
}
