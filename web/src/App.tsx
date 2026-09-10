import { useQuery } from '@tanstack/react-query';
import { cn } from 'cn';
import {
  CircleCheck,
  CircleDashed,
  CircleX,
  Layers,
  RefreshCw,
  Terminal,
} from 'lucide-react';
import { healthQuery } from './api';
import { Button } from './components/ui/button';
import { Separator } from './components/ui/separator';

export default function App() {
  const health = useQuery(healthQuery);
  const pending = health.isFetching || health.isPending;
  const connected = !pending && health.isSuccess;
  const StateIcon = pending ? CircleDashed : connected ? CircleCheck : CircleX;
  const title = pending
    ? '正在检查服务'
    : connected
      ? '服务已连接'
      : '暂时无法连接';

  return (
    <div className="min-h-screen">
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-5 py-5 sm:px-8">
          <div className="flex items-center gap-3 font-semibold tracking-tight">
            <Layers aria-hidden="true" className="size-5 text-primary" />
            <span className="text-lg">Loadout</span>
          </div>
          <span className="text-sm text-muted-foreground">接入控制台</span>
        </div>
      </header>

      <main className="mx-auto flex max-w-5xl flex-col gap-10 px-5 py-10 sm:px-8 sm:py-14">
        <div className="flex flex-col gap-3">
          <h1 className="text-3xl font-semibold tracking-tight">接入状态</h1>
          <p className="max-w-prose text-base leading-7 text-muted-foreground">
            让你的 AI 工具，准备就绪。从这里确认 Loadout 服务是否可用。
          </p>
        </div>

        <section
          aria-label="服务连接"
          className="overflow-hidden rounded-lg border border-border bg-card"
        >
          <div className="flex flex-col gap-6 p-6 sm:flex-row sm:items-center sm:justify-between sm:p-8">
            <div
              role="status"
              aria-live="polite"
              className="flex items-start gap-4"
            >
              <StateIcon
                aria-hidden="true"
                className={cn(
                  'mt-1 size-6 shrink-0',
                  pending
                    ? 'text-muted-foreground'
                    : connected
                      ? 'text-success'
                      : 'text-destructive',
                )}
              />
              <div className="flex flex-col gap-2">
                <h2 className="text-xl font-medium">{title}</h2>
                <p className="max-w-prose text-sm leading-6 text-muted-foreground">
                  {pending
                    ? '正在与网关建立连接，请稍候。'
                    : connected
                      ? '网关可以响应请求。你可以继续在本地检查连接。'
                      : '确认服务已启动，再检查连接。'}
                </p>
              </div>
            </div>
            <Button
              type="button"
              onClick={() => {
                void health.refetch();
              }}
              disabled={pending}
              className="self-start sm:shrink-0"
            >
              <RefreshCw aria-hidden="true" data-icon="inline-start" />
              检查连接
            </Button>
          </div>
          <Separator />
          <dl className="grid grid-cols-2 gap-6 px-6 py-5 text-sm sm:px-8">
            <div className="flex flex-col gap-2">
              <dt className="text-muted-foreground">服务版本</dt>
              <dd className="font-mono break-all">
                {connected ? health.data.version : '—'}
              </dd>
            </div>
            <div className="flex flex-col gap-2">
              <dt className="text-muted-foreground">当前阶段</dt>
              <dd>基础连接验证</dd>
            </div>
          </dl>
        </section>

        <section
          aria-labelledby="connect-title"
          className="flex flex-col gap-6"
        >
          <div className="flex flex-col gap-2">
            <h2 id="connect-title" className="text-lg font-medium">
              一套工具，连接你的工作流
            </h2>
            <p className="text-sm leading-6 text-muted-foreground">
              以下接入流程正在开发，当前可先检查服务连接。
            </p>
          </div>
          <ol className="grid gap-6 sm:grid-cols-3 sm:gap-8">
            <li className="flex flex-col gap-2">
              <h3 className="font-medium">网页开通</h3>
              <p className="text-sm leading-6 text-muted-foreground">
                注册账号，创建专属网关令牌。
              </p>
            </li>
            <li className="flex flex-col gap-2">
              <h3 className="font-medium">本地配置</h3>
              <p className="text-sm leading-6 text-muted-foreground">
                通过 CLI 登录，一次配置多个 AI 客户端。
              </p>
            </li>
            <li className="flex flex-col gap-2">
              <h3 className="font-medium">工具就绪</h3>
              <p className="text-sm leading-6 text-muted-foreground">
                使用预设 MCP 工具，统一查看用量与额度。
              </p>
            </li>
          </ol>
        </section>

        <section
          aria-labelledby="doctor-title"
          className="flex min-w-0 flex-col gap-4"
        >
          <h2
            id="doctor-title"
            className="flex items-center gap-2 text-base font-medium"
          >
            <Terminal
              aria-hidden="true"
              className="size-4 text-muted-foreground"
            />
            在本地检查连接
          </h2>
          <p className="text-sm leading-6 text-muted-foreground">
            安装 CLI 后，用下面的命令检查同一台服务。将地址替换为你的服务地址。
          </p>
          <section
            aria-label="CLI 检查命令"
            // biome-ignore lint/a11y/noNoninteractiveTabindex: Scrollable code needs keyboard access (WCAG 2.1.1).
            tabIndex={0}
            className="overflow-x-auto rounded-lg border border-border bg-card p-5 text-sm leading-6"
          >
            <pre>
              <code>loadout doctor --server http://127.0.0.1:8787</code>
            </pre>
          </section>
        </section>

        <footer className="border-t border-border pt-6 text-sm text-muted-foreground">
          Your AI, fully loaded.
        </footer>
      </main>
    </div>
  );
}
