import { useQuery } from '@tanstack/react-query';
import {
  ArrowLeft,
  ArrowUpRight,
  BookOpen,
  Boxes,
  ChevronLeft,
  ChevronRight,
  Copy,
  Plug,
  Search,
  Terminal,
} from 'lucide-react';
import { useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { ApiError, request } from './account/api';
import { ErrorNotice, Heading, Loading } from './account/shared';
import {
  publicPluginDetailSchema,
  publicPluginsSchema,
} from './operations/api';
import { PublicHeader } from './PublicHeader';
import './PluginsPage.css';

function InstallCommand({ command }: { command: string }) {
  const { t } = useI18n();
  const [message, setMessage] = useState('');
  return (
    <section className="command-list">
      <div className="command-heading">
        <Terminal aria-hidden="true" />
        <span>{t('在本地终端运行')}</span>
        <Button
          variant="ghost"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(command);
              setMessage('已复制');
            } catch {
              setMessage('复制失败，请手动选择命令复制。');
            }
          }}
        >
          <Copy aria-hidden="true" />
          {t('复制命令')}
        </Button>
      </div>
      {/* biome-ignore lint/a11y/noNoninteractiveTabindex: Scrollable commands must be reachable with a keyboard. */}
      <pre tabIndex={0}>
        <code>{command}</code>
      </pre>
      {message && <p role="status">{t(message)}</p>}
    </section>
  );
}

function Catalog() {
  const { t } = useI18n();
  const results = useRef<HTMLHeadingElement>(null);
  const [params, setParams] = useSearchParams();
  const query = useQuery({
    queryKey: ['public-plugins'],
    queryFn: ({ signal }) =>
      request('/plugins', publicPluginsSchema, { signal }),
  });
  const q = params.get('q') ?? '';
  const kind = ['mcp', 'skill', 'bundle'].includes(params.get('kind') ?? '')
    ? (params.get('kind') ?? '')
    : '';
  const kinds = [
    ['', '全部'],
    ['mcp', 'MCP'],
    ['skill', '技能'],
    ['bundle', '装备组'],
  ] as const;
  const change = (key: string, value: string) =>
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        if (value) next.set(key, value);
        else next.delete(key);
        if (key !== 'page') next.delete('page');
        return next;
      },
      { replace: true },
    );
  const items =
    query.data?.items.filter(
      (item) =>
        (!kind || item.kind === kind) &&
        `${item.name} ${item.slug} ${item.description}`
          .toLocaleLowerCase()
          .includes(q.trim().toLocaleLowerCase()),
    ) ?? [];
  const pageSize = 12;
  const pageCount = Math.max(1, Math.ceil(items.length / pageSize));
  const requestedPage = Number(params.get('page') ?? 1);
  const page =
    Number.isSafeInteger(requestedPage) && requestedPage > 0
      ? Math.min(requestedPage, pageCount)
      : 1;
  const offset = (page - 1) * pageSize;
  const pages = Array.from(
    { length: pageCount },
    (_, index) => index + 1,
  ).filter(
    (number) =>
      number === 1 || number === pageCount || Math.abs(number - page) <= 1,
  );
  const goToPage = (number: number) => {
    change('page', number === 1 ? '' : String(number));
    results.current?.focus({ preventScroll: true });
    results.current?.scrollIntoView?.({ block: 'start' });
  };
  return (
    <>
      <section className="catalog-hero">
        <div className="catalog-hero-content">
          <Heading title="插件市场">
            {t('发现 MCP 插件、技能与装备组，按需装进你的 AI 工作流。')}
          </Heading>
          <div className="catalog-search">
            <Search aria-hidden="true" />
            <Input
              type="search"
              aria-label={t('搜索插件')}
              placeholder={t('搜索工具、技能或你想完成的任务')}
              value={q}
              onChange={(event) => change('q', event.target.value)}
            />
          </div>
        </div>
        <img
          className="catalog-hero-art"
          src="/images/workshop-marketplace.webp"
          alt=""
          width="1200"
          height="900"
          fetchPriority="high"
        />
      </section>
      <div className="catalog-toolbar">
        <fieldset className="catalog-filters" aria-label={t('插件类型')}>
          {kinds.map(([value, label]) => (
            <Button
              key={value}
              variant={kind === value ? 'default' : 'ghost'}
              aria-pressed={kind === value}
              aria-label={t(label)}
              onClick={() => change('kind', value)}
            >
              {t(label)}
              {query.data && (
                <span className="catalog-filter-count" aria-hidden="true">
                  {
                    query.data.items.filter(
                      (item) => !value || item.kind === value,
                    ).length
                  }
                </span>
              )}
            </Button>
          ))}
        </fieldset>
      </div>
      <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      {query.isPending && <Loading />}
      {query.data && (
        <>
          <div className="catalog-results-heading">
            <h2 ref={results} tabIndex={-1}>
              {t(
                kind
                  ? (kinds.find(([value]) => value === kind)?.[1] ?? '全部插件')
                  : '全部插件',
              )}
            </h2>
            <span role="status">
              {items.length
                ? t('显示 {start}–{end} 项，共 {total} 项', {
                    start: offset + 1,
                    end: Math.min(offset + pageSize, items.length),
                    total: items.length,
                  })
                : t('共 {total} 项', { total: 0 })}
            </span>
          </div>
          <ul className="catalog-records" aria-label={t('插件列表')}>
            {items.slice(offset, offset + pageSize).map((item) => {
              const Icon =
                item.kind === 'skill'
                  ? BookOpen
                  : item.kind === 'bundle'
                    ? Boxes
                    : Plug;
              return (
                <li
                  key={item.slug}
                  className="catalog-card"
                  data-kind={item.kind}
                >
                  <div className="catalog-card-top">
                    <span className="catalog-monogram" aria-hidden="true">
                      {Array.from(item.name.trim()).slice(0, 2).join('')}
                    </span>
                    <span className="catalog-kind">
                      <Icon aria-hidden="true" />
                      {t(
                        kinds.find(([value]) => value === item.kind)?.[1] ??
                          'MCP',
                      )}
                    </span>
                  </div>
                  <h3>
                    <Link to={`/plugins/${encodeURIComponent(item.slug)}`}>
                      {item.name}
                      <ArrowUpRight aria-hidden="true" />
                    </Link>
                  </h3>
                  <span className="catalog-slug">{item.slug}</span>
                  <p className="catalog-description">{item.description}</p>
                  <div className="catalog-meta">
                    <span>v{item.version}</span>
                    {item.gateway && (
                      <span className="status-badge" data-tone="success">
                        {t('网关托管')}
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
          {items.length > pageSize && (
            <nav className="catalog-pagination" aria-label={t('插件分页')}>
              <Button
                variant="outline"
                disabled={page === 1}
                onClick={() => goToPage(page - 1)}
              >
                <ChevronLeft aria-hidden="true" />
                {t('上一页')}
              </Button>
              <div className="catalog-page-numbers">
                {pages.map((number, index) => (
                  <span key={number}>
                    {index > 0 && number - pages[index - 1] > 1 && (
                      <span className="catalog-page-gap" aria-hidden="true">
                        …
                      </span>
                    )}
                    <Button
                      variant={page === number ? 'default' : 'ghost'}
                      aria-label={t('第 {page} 页', { page: number })}
                      aria-current={page === number ? 'page' : undefined}
                      onClick={() => goToPage(number)}
                    >
                      {number}
                    </Button>
                  </span>
                ))}
              </div>
              <span className="catalog-mobile-page">
                {page} / {pageCount}
              </span>
              <Button
                variant="outline"
                disabled={page === pageCount}
                onClick={() => goToPage(page + 1)}
              >
                {t('下一页')}
                <ChevronRight aria-hidden="true" />
              </Button>
            </nav>
          )}
          {!items.length && (
            <div className="empty-state">
              <h2>
                {t(
                  query.data.items.length
                    ? '没有匹配的插件'
                    : '暂时没有市场条目。',
                )}
              </h2>
              {(q || kind) && (
                <Button variant="outline" onClick={() => setParams({})}>
                  {t('清除筛选')}
                </Button>
              )}
            </div>
          )}
          <div className="section-stack catalog-connect">
            <h2>{t('接入整个市场')}</h2>
            <InstallCommand
              command={`codex plugin marketplace add ${query.data.origin || window.location.origin}/marketplace.git`}
            />
          </div>
        </>
      )}
    </>
  );
}

function Detail({ slug }: { slug: string }) {
  const { t } = useI18n();
  const query = useQuery({
    queryKey: ['public-plugin', slug],
    queryFn: ({ signal }) =>
      request(
        `/plugins/${encodeURIComponent(slug)}`,
        publicPluginDetailSchema,
        { signal },
      ),
  });
  return (
    <>
      <Link className="inline-action" to="/plugins">
        <ArrowLeft aria-hidden="true" />
        {t('返回插件市场')}
      </Link>
      {query.isPending && <Loading />}
      {query.error instanceof ApiError && query.error.status === 404 ? (
        <Heading title="插件不存在">
          {t('返回市场查看当前可用的插件。')}
        </Heading>
      ) : (
        <ErrorNotice error={query.error} retry={() => void query.refetch()} />
      )}
      {query.data && (
        <>
          <Heading title={query.data.item.name} translateTitle={false}>
            {query.data.item.description}
          </Heading>
          <div className="catalog-meta">
            <span className="status-badge">
              {t(
                query.data.item.kind === 'skill'
                  ? '技能'
                  : query.data.item.kind === 'bundle'
                    ? '装备组'
                    : 'MCP',
              )}
            </span>
            <span>v{query.data.item.version}</span>
            {query.data.item.gateway && (
              <span className="status-badge" data-tone="success">
                {t('网关托管')}
              </span>
            )}
          </div>
          {query.data.item.gateway && (
            <p className="hint-text">
              {t(
                '浏览无需登录；使用网关托管工具前，请先登录 PluginPocket CLI。',
              )}
            </p>
          )}
          <div className="section-stack">
            <h2>{t('接入整个市场')}</h2>
            <InstallCommand
              key={`codex-${slug}`}
              command={`codex plugin marketplace add ${query.data.origin || window.location.origin}/marketplace.git\ncodex plugin add ${query.data.item.slug} --marketplace pluginpocket`}
            />
          </div>
          <div className="section-stack">
            <h2>{t('使用 PluginPocket CLI 安装')}</h2>
            <InstallCommand
              key={`cli-${slug}`}
              command={`pluginpocket install ${query.data.item.slug}`}
            />
          </div>
        </>
      )}
    </>
  );
}

export default function PluginsPage({
  embedded = false,
}: {
  embedded?: boolean;
}) {
  const { slug } = useParams();
  const { t } = useI18n();
  if (embedded)
    return (
      <div className="workspace-catalog">
        {slug ? <Detail slug={slug} /> : <Catalog />}
      </div>
    );
  return (
    <div className="public-catalog">
      <a className="skip-link" href="#catalog-main">
        {t('跳到主内容')}
      </a>
      <PublicHeader />
      <main id="catalog-main" className="app-main">
        {slug ? <Detail slug={slug} /> : <Catalog />}
      </main>
      <footer className="landing-footer">
        <span className="brand-wordmark">PluginPocket</span>
        <span>{t('把 AI 的超能力，装进口袋。')}</span>
        <Link className="inline-action" to="/health">
          {t('服务状态')}
        </Link>
      </footer>
    </div>
  );
}
