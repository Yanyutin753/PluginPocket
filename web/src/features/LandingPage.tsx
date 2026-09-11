import { ArrowRight, Layers3, Terminal } from 'lucide-react';
import { Button } from '../components/ui/button';
import { PublicHeader } from './PublicHeader';
import './LandingPage.css';
import { useI18n } from '../i18n';

export default function LandingPage() {
  const { t } = useI18n();
  return (
    <div className="landing">
      <a className="skip-link" href="#landing-main">
        {t('跳至主要内容')}
      </a>
      <PublicHeader />
      <main id="landing-main">
        <section className="landing-hero">
          <div className="landing-hero-copy">
            <h1>{t('把 AI 的超能力，装进口袋。')}</h1>
            <p className="landing-lead">
              {t(
                '把工具接入一个 MCP 网关。让你熟悉的 AI 客户端，带着合适的装备开始工作。',
              )}
            </p>
            <div className="landing-actions">
              <Button asChild size="lg">
                <a href="/register">
                  {t('创建账号，开始装备')}
                  <ArrowRight aria-hidden="true" />
                </a>
              </Button>
              <a className="inline-action" href="#how-it-works">
                {t('看看如何连接')}
                <ArrowRight aria-hidden="true" />
              </a>
            </div>
            <p className="landing-clients">
              {t('支持 Codex · Claude Code · Cursor')}
            </p>
          </div>
          <div className="landing-hero-art">
            <img
              src="/images/workshop-welcome.webp"
              alt={t('薄荷绿色工具箱伙伴打开装满 AI 工具的工作台')}
              width="1200"
              height="900"
              fetchPriority="high"
            />
          </div>
        </section>

        <section
          className="landing-capabilities"
          aria-labelledby="capabilities-title"
        >
          <div className="landing-section-heading">
            <h2 id="capabilities-title">{t('连接更简单，使用更清楚。')}</h2>
            <p>{t('从个人探索到团队协作，在一个地方管理 AI 的工具入口。')}</p>
          </div>
          <div className="landing-feature-grid">
            <article className="landing-feature landing-feature-tools">
              <img
                src="/images/workshop-tools.webp"
                alt=""
                width="900"
                height="600"
                loading="lazy"
              />
              <div>
                <Layers3 aria-hidden="true" />
                <h3>{t('一个网关，连接工具')}</h3>
                <p>
                  {t(
                    '通过 MCP 为客户端提供工具入口，集中查看可用工具与调用成本。',
                  )}
                </p>
              </div>
            </article>
            <article className="landing-feature">
              <img
                className="landing-card-art"
                src="/images/workshop-security-v2.webp"
                alt=""
                width="900"
                height="600"
                loading="lazy"
              />
              <h3>{t('令牌集中管理')}</h3>
              <p>
                {t(
                  '为不同客户端创建网关令牌。随时查看使用情况，需要时撤销访问。',
                )}
              </p>
            </article>
            <article className="landing-feature">
              <img
                className="landing-card-art"
                src="/images/workshop-insights-v2.webp"
                alt=""
                width="900"
                height="600"
                loading="lazy"
              />
              <h3>{t('每次调用，都有记录')}</h3>
              <p>{t('查看执行结果、耗时与实际消耗，让用量清晰可见。')}</p>
            </article>
            <article className="landing-feature landing-feature-team">
              <img
                className="landing-team-art"
                src="/images/workshop-team-v2.webp"
                alt=""
                width="900"
                height="600"
                loading="lazy"
              />
              <h3>{t('把装备分享给团队')}</h3>
              <p>
                {t(
                  '邀请成员、共享团队额度，按成员查看用量。协作时，也能各自保持清楚。',
                )}
              </p>
            </article>
          </div>
        </section>

        <section
          id="how-it-works"
          className="landing-setup"
          aria-labelledby="setup-title"
        >
          <div className="landing-section-heading">
            <h2 id="setup-title">{t('三步，进入工作状态。')}</h2>
          </div>
          <ol className="landing-steps" aria-label={t('连接步骤')}>
            <li>
              <span>01</span>
              <h3>{t('创建账号')}</h3>
              <p>{t('进入你的 PluginPocket 工作空间。')}</p>
            </li>
            <li>
              <span>02</span>
              <h3>{t('创建网关令牌')}</h3>
              <p>{t('在令牌页创建专属连接凭证。')}</p>
            </li>
            <li>
              <span>03</span>
              <h3>{t('连接客户端')}</h3>
              <p>{t('通过 CLI 或桌面端配置 Codex、Claude Code、Cursor。')}</p>
            </li>
          </ol>
          <div className="landing-terminal">
            <div>
              <Terminal aria-hidden="true" />
              <span>{t('已安装 PluginPocket CLI？从终端连接。')}</span>
            </div>
            <pre>
              <code>{`pluginpocket login --server ${window.location.origin}\npluginpocket apply`}</code>
            </pre>
            <p>{t('登录会引导你完成授权；也可以使用桌面端配置客户端。')}</p>
          </div>
        </section>

        <section className="landing-close">
          <img
            src="/images/workshop-credits.webp"
            alt=""
            width="900"
            height="600"
            loading="lazy"
          />
          <div>
            <h2>{t('下一次灵感，带上更好的装备。')}</h2>
            <p>{t('让工具准备好，把注意力留给你想做的事。')}</p>
            <Button asChild size="lg">
              <a href="/register">
                {t('创建账号')}
                <ArrowRight aria-hidden="true" />
              </a>
            </Button>
          </div>
        </section>
      </main>
      <footer className="landing-footer">
        <span>{t('PluginPocket · AI 装备工坊')}</span>
        <a className="inline-action" href="/overview">
          {t('进入工作空间')}
        </a>
        <span>{t('登录后管理工具与用量')}</span>
      </footer>
    </div>
  );
}
