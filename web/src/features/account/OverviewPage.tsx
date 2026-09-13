import { useQuery } from '@tanstack/react-query';
import {
  Activity,
  ArrowRight,
  ArrowUpRight,
  ChartNoAxesCombined,
  Circle,
  Copy,
  CreditCard,
  KeyRound,
  Terminal,
} from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n';
import { accountQuery, number } from './api';
export default function OverviewPage() {
  const { t, locale } = useI18n();
  const { data } = useQuery(accountQuery);
  const [copyMessage, setCopyMessage] = useState('');
  if (!data) return null;
  const stats = [
    ['可用额度', data.user.balance, CreditCard],
    ['今日调用', data.summary.today_calls, Activity],
    ['本月消耗', data.summary.month_cost, ChartNoAxesCombined],
    ['有效令牌', data.summary.token_count, KeyRound],
  ] as const;
  return (
    <>
      <section className="workshop-hero overview-hero">
        <div className="workshop-intro">
          <div className="workshop-note" aria-hidden="true">
            {t('工欲善其事，')}
            <br />
            {t('必先利其器。')}
          </div>
          <h1>{t('把 AI 的超能力，装进口袋。')}</h1>
          <p>{t('一次接入，让工具进入你的工作流。')}</p>
          <Button asChild className="workshop-action" size="lg">
            <Link to="/tokens">
              <KeyRound aria-hidden="true" />
              {t('创建网关令牌')}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
          <div className="client-wordmarks">
            <span>Codex</span>
            <span>Claude Code</span>
            <span>Cursor</span>
          </div>
        </div>
        <img
          className="workshop-illustration"
          src="/images/workshop-hero.webp"
          alt=""
          width="1200"
          height="900"
          fetchPriority="high"
        />
      </section>
      <section className="workshop-account" aria-labelledby="account-overview">
        <h2 id="account-overview">{t('账号概览')}</h2>
        <dl className="stats-grid">
          {stats.map(([label, value, Icon]) => (
            <div key={label} className="stat-card">
              <dt>
                {t(label)}
                <Icon aria-hidden="true" className="stat-icon" />
              </dt>
              <dd>{number(value, locale)}</dd>
              {value === 0 && (
                <p className="stat-note">
                  {t(
                    label === '今日调用'
                      ? '今日暂无调用'
                      : label === '本月消耗'
                        ? '本月暂无消耗'
                        : '当前暂无有效令牌',
                  )}
                </p>
              )}
            </div>
          ))}
        </dl>
      </section>
      <section className="workshop-setup" aria-label={t('接入指南')}>
        <ol className="workshop-steps" aria-label={t('接入指南')}>
          <li>
            <span className="step-number" aria-hidden="true">
              1
            </span>
            <div>
              <h2>{t('创建令牌')}</h2>
              <p>{t('独立管理每台设备的访问权限。')}</p>
            </div>
          </li>
          <li>
            <span className="step-number" aria-hidden="true">
              2
            </span>
            <div>
              <h2>{t('连接客户端')}</h2>
              <p>{t('用两条命令接入你的工作流。')}</p>
            </div>
          </li>
        </ol>
        <div className="command-list">
          <div className="terminal-dots" aria-hidden="true">
            <Circle />
            <Circle />
            <Circle />
          </div>
          <img
            className="terminal-mascot"
            src="/images/workshop-mark.webp"
            alt=""
            width="256"
            height="256"
          />
          <div className="command-heading">
            <Terminal aria-hidden="true" />
            <span>{t('在本地终端运行')}</span>
            <span className="terminal-language">CLI</span>
            <Button
              variant="ghost"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(
                    `pluginpocket login --server ${window.location.origin}\npluginpocket apply`,
                  );
                  setCopyMessage('接入命令已复制');
                } catch {
                  setCopyMessage('无法自动复制，请选中命令并手动复制。');
                }
              }}
            >
              <Copy aria-hidden="true" />
              {t('复制接入命令')}
            </Button>
          </div>
          <div className="command-lines">
            <code>
              <span aria-hidden="true">$ </span>pluginpocket login --server{' '}
              {window.location.origin}
            </code>
            <code>
              <span aria-hidden="true">$ </span>pluginpocket apply
            </code>
          </div>
          <p>{t('登录时粘贴令牌。不要把令牌写在命令行参数中。')}</p>
          {copyMessage && <p role="status">{t(copyMessage)}</p>}
        </div>
      </section>
      <div className="overview-links">
        <Link to="/tools">
          {t('探索工具')}
          <ArrowUpRight aria-hidden="true" />
        </Link>
        <Link to="/billing">
          {t('管理额度')}
          <ArrowUpRight aria-hidden="true" />
        </Link>
        <Link to="/usage">
          {t('查看用量明细')}
          <ArrowUpRight aria-hidden="true" />
        </Link>
        <Link to="/device">
          {t('设备授权')}
          <ArrowUpRight aria-hidden="true" />
        </Link>
      </div>
    </>
  );
}
