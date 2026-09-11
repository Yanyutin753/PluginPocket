import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Link, LogOut, RefreshCw, SunMoon, Unplug } from 'lucide-react';
import { useEffect, useState } from 'react';
import workshopDesktop from '../../../web/public/images/workshop-desktop.webp';
import workshopMark from '../../../web/public/images/workshop-mark.webp';
import { Select } from '../../../web/src/components/ui/select';
import { api, type ClientKind } from './api';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';

const labels: Record<ClientKind, string> = {
  codex: 'Codex',
  claude: 'Claude Code',
  cursor: 'Cursor',
};

function LoadingPlaceholder() {
  return (
    <div className="loading-placeholder">
      <div aria-hidden="true" className="skeleton-content">
        <span className="skeleton skeleton-short" />
        <span className="skeleton skeleton-field" />
        <span className="skeleton skeleton-field" />
        <span className="skeleton skeleton-short" />
      </div>
    </div>
  );
}
export function App() {
  const [theme, setTheme] = useState('system');
  useEffect(() => {
    document.documentElement.style.colorScheme =
      theme === 'system' ? 'light dark' : theme;
  }, [theme]);
  const cache = useQueryClient();
  const status = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const clients = useQuery({
    queryKey: ['clients'],
    queryFn: api.clients,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const [server, setServer] = useState('');
  const [token, setToken] = useState('');
  const [selected, setSelected] = useState<ClientKind[]>([]);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const login = useMutation({
    mutationFn: () => api.login(server, token),
    onSuccess: async (account) => {
      await cache.cancelQueries({ queryKey: ['status'] });
      cache.setQueryData(['status'], { account, clients: clients.data ?? [] });
      setMessage('已登录 选择客户端完成接入');
      setError('');
    },
    onError: () => setError('登录失败，请检查服务地址和令牌后重试'),
    onSettled: () => setToken(''),
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      await cache.cancelQueries({ queryKey: ['status'] });
      cache.setQueryData(['status'], null);
      login.reset();
      setToken('');
      setMessage('已退出登录 已配置客户端的 bridge 将停止使用凭证');
      setError('');
    },
    onError: () => setError('退出登录失败，请重试'),
  });
  const configure = useMutation({
    mutationFn: (remove: boolean) =>
      remove ? api.remove(selected) : api.apply(selected),
    onSuccess: async (_result, remove) => {
      await cache.invalidateQueries({ queryKey: ['clients'] });
      setMessage(
        remove
          ? '已移除所选客户端的 Loadout 配置'
          : '配置已写入 重启客户端后即可使用',
      );
      setError('');
    },
    onError: () =>
      setError(
        '配置未完成 请检查客户端配置是否有手写冲突或无效内容，处理后重试',
      ),
  });
  const doctor = useMutation({
    mutationFn: () => api.doctor(server || null),
    onSuccess: (result) => {
      setMessage(
        result.authenticated ? '服务可达 · 凭证有效' : '服务可达 · 尚未登录',
      );
      setError('');
    },
    onError: () => setError('连接检查失败，请检查服务地址、网络和凭证后重试'),
  });
  const pending =
    login.isPending ||
    logout.isPending ||
    configure.isPending ||
    doctor.isPending;
  const account = status.isError ? undefined : status.data?.account;
  const refresh = () => {
    setMessage('');
    setError('');
    void cache.invalidateQueries({ queryKey: ['status'] });
    void cache.invalidateQueries({ queryKey: ['clients'] });
  };
  return (
    <main className="desktop-shell">
      <header className="desktop-header">
        <div className="desktop-brand">
          <img src={workshopMark} alt="" width="48" height="48" />
          <span>
            <span className="brand-wordmark">Loadout</span>
            <small>AI 装备工坊</small>
          </span>
        </div>
        <div className="desktop-controls">
          <label className="theme-control" htmlFor="desktop-theme">
            <span className="sr-only">外观</span>
            <Select
              icon={<SunMoon />}
              id="desktop-theme"
              aria-label="外观"
              value={theme}
              onValueChange={setTheme}
              options={[
                { value: 'light', label: '浅色' },
                { value: 'dark', label: '深色' },
                { value: 'system', label: '跟随系统' },
              ]}
            />
          </label>
          <Button
            variant="outline"
            onClick={refresh}
            disabled={pending || status.isFetching || clients.isFetching}
          >
            <RefreshCw aria-hidden="true" data-icon="inline-start" />
            刷新状态
          </Button>
        </div>
      </header>
      <div className="desktop-intro">
        <div>
          <h1>让你的 AI，准备就绪</h1>
          <p>登录一次，将工具接入这台电脑上的 AI 客户端</p>
        </div>
        <img src={workshopDesktop} alt="" width="900" height="600" />
      </div>
      <div className="desktop-grid">
        {status.isPending && (
          <span className="sr-only" role="status" aria-label="正在读取本地状态">
            正在读取本地状态…
          </span>
        )}
        {clients.isPending && (
          <span className="sr-only" role="status" aria-label="正在检测客户端">
            正在检测客户端…
          </span>
        )}
        <section
          className="account-panel"
          aria-labelledby="account-title"
          aria-busy={status.isPending}
        >
          <h2 id="account-title">账号与连接</h2>
          {status.isPending ? (
            <LoadingPlaceholder />
          ) : account ? (
            <div className="account-content">
              <div className="account-identity">
                <Check aria-hidden="true" />
                <strong>{account.username}</strong>
              </div>
              <p className="balance">{account.balance} credits</p>
              <p>{account.tools.length} 个可用工具</p>
              <div className="action-row">
                <Button
                  variant="outline"
                  onClick={() => doctor.mutate()}
                  disabled={pending}
                >
                  检查连接
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => logout.mutate()}
                  disabled={pending}
                >
                  <LogOut aria-hidden="true" data-icon="inline-start" />
                  退出登录
                </Button>
              </div>
            </div>
          ) : (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                if (!pending) {
                  setError('');
                  login.mutate();
                }
              }}
              className="login-form"
            >
              <p className="muted">从网页控制台创建令牌，然后在这里登录</p>
              {status.isError && (
                <p className="muted">尚未登录或凭证不可用，请登录或刷新重试</p>
              )}
              <label htmlFor="server">服务地址</label>
              <Input
                id="server"
                type="url"
                value={server}
                onChange={(e) => setServer(e.target.value)}
                placeholder="https://api.example.com"
                required
                disabled={pending}
                autoComplete="url"
              />
              <label htmlFor="token">Loadout 令牌</label>
              <Input
                id="token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="ldt_…"
                required
                disabled={pending}
                autoComplete="off"
                spellCheck={false}
              />
              <p className="muted small">
                令牌保存在本机，客户端配置中只写入 bridge
              </p>
              <div className="action-row">
                <Button type="submit" disabled={pending}>
                  {login.isPending ? '登录中…' : '登录'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => doctor.mutate()}
                  disabled={pending}
                >
                  检查连接
                </Button>
              </div>
            </form>
          )}
        </section>
        <section
          className="clients-panel"
          aria-labelledby="clients-title"
          aria-busy={clients.isPending}
        >
          <h2 id="clients-title">这台电脑的客户端</h2>
          <p className="muted">
            勾选要接入的客户端；也可以为尚未启动过的客户端创建配置
          </p>
          {clients.isPending && <LoadingPlaceholder />}
          {clients.isError && (
            <p role="alert" className="error">
              无法读取客户端配置，请检查文件内容后刷新重试
            </p>
          )}
          <fieldset
            disabled={pending || clients.isPending || clients.isError}
            className="client-list"
          >
            <legend className="sr-only">选择客户端</legend>
            {clients.data?.map((client) => (
              <label key={client.client} className="client-option">
                <input
                  type="checkbox"
                  checked={selected.includes(client.client)}
                  onChange={(e) =>
                    setSelected((current) =>
                      e.target.checked
                        ? [...current, client.client]
                        : current.filter((kind) => kind !== client.client),
                    )
                  }
                />
                <span>
                  <strong>{labels[client.client]}</strong>
                  <small>
                    {client.configured
                      ? '已配置 Loadout'
                      : client.detected
                        ? '已检测到 · 尚未配置'
                        : '未检测到 · 可创建配置'}
                  </small>
                </span>
                {client.configured && <Check aria-hidden="true" />}
              </label>
            ))}
          </fieldset>
          <div className="client-actions">
            <Button
              onClick={() => configure.mutate(false)}
              disabled={pending || !account || selected.length === 0}
            >
              <Link aria-hidden="true" data-icon="inline-start" />
              配置所选客户端
            </Button>
            <Button
              variant="outline"
              onClick={() => configure.mutate(true)}
              disabled={pending || selected.length === 0}
            >
              <Unplug aria-hidden="true" data-icon="inline-start" />
              移除所选配置
            </Button>
          </div>
          <p className="muted small">
            已有配置会先备份；遇到手写的 Loadout 条目时会停止，供你确认处理
          </p>
        </section>
      </div>
      <div className="feedback" aria-live="polite">
        {message && <p role="status">{message}</p>}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
      </div>
      <footer>
        关闭窗口后可从系统托盘重新打开 退出应用不会删除已保存的本地凭证
      </footer>
    </main>
  );
}
