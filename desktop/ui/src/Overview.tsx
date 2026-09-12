import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Link, LogOut, RefreshCw, Unplug } from 'lucide-react';
import { useState } from 'react';
import workshopDesktop from '../../../web/public/images/workshop-desktop.webp';
import { api, type ClientKind } from './api';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { UpdatePanel } from './UpdatePanel';

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
export function Overview({ native }: { native: boolean }) {
  const cache = useQueryClient();
  const status = useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    enabled: native,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const clients = useQuery({
    queryKey: ['clients'],
    queryFn: api.clients,
    enabled: native,
    retry: false,
    refetchOnWindowFocus: false,
  });
  const [server, setServer] = useState('');
  const [token, setToken] = useState('');
  const [selected, setSelected] = useState<ClientKind[]>([]);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const clearFeedback = () => {
    setMessage('');
    setError('');
  };
  const login = useMutation({
    onMutate: clearFeedback,
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
    onMutate: clearFeedback,
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
    onMutate: clearFeedback,
    mutationFn: (remove: boolean) =>
      remove ? api.remove(selected) : api.apply(selected),
    onSuccess: async (_result, remove) => {
      await cache.invalidateQueries({ queryKey: ['clients'] });
      setMessage(
        remove
          ? '已移除所选客户端的 PluginPocket 配置'
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
    onMutate: clearFeedback,
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
  const clientsUnavailable =
    clients.isPending || clients.isFetching || clients.isError;
  const configurationDisabled =
    pending || status.isFetching || clientsUnavailable || selected.length === 0;
  const refresh = () => {
    clearFeedback();
    void cache.invalidateQueries({ queryKey: ['status'] });
    void cache.invalidateQueries({ queryKey: ['clients'] });
  };
  return (
    <div className="overview">
      <div className="overview-toolbar">
        <p className="muted">账号、工具与这台电脑的接入状态</p>
        <Button
          variant="outline"
          onClick={refresh}
          disabled={
            !native || pending || status.isFetching || clients.isFetching
          }
        >
          <RefreshCw aria-hidden="true" data-icon="inline-start" />
          刷新状态
        </Button>
      </div>
      <div className="desktop-intro">
        <div>
          <h2>一处接入，随时开工。</h2>
          <p>
            把常用工具带进 Codex、Claude Code 和 Cursor。
            <br />
            安装、排障和运行记录，都在这个工作台。
          </p>
        </div>
        <img src={workshopDesktop} alt="" width="900" height="600" />
      </div>
      <div className="desktop-grid">
        {native && status.isPending && (
          <span className="sr-only" role="status" aria-label="正在读取本地状态">
            正在读取本地状态…
          </span>
        )}
        {native && clients.isPending && (
          <span className="sr-only" role="status" aria-label="正在检测客户端">
            正在检测客户端…
          </span>
        )}
        <section
          className="account-panel"
          aria-labelledby="account-title"
          aria-busy={native && status.isPending}
        >
          <h2 id="account-title">账号与连接</h2>
          {!native ? (
            <div className="account-content">
              <p className="muted">
                在桌面应用中连接你的 PluginPocket 服务，即可查看真实账号与额度。
              </p>
              <label htmlFor="preview-server">服务地址</label>
              <Input
                id="preview-server"
                placeholder="https://api.example.com"
                disabled
              />
              <Button disabled>请在桌面应用中登录</Button>
              <p className="small muted">
                凭证仅由本机保存，客户端配置不写入密钥。
              </p>
            </div>
          ) : status.isPending ? (
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
              <label htmlFor="token">PluginPocket 令牌</label>
              <Input
                id="token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="ppt_…"
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
          aria-busy={native && clients.isPending}
        >
          <h2 id="clients-title">这台电脑的客户端</h2>
          <p className="muted">
            勾选要接入的客户端；也可以为尚未启动过的客户端创建配置
          </p>
          {native && clients.isPending && <LoadingPlaceholder />}
          {!native && (
            <div className="preview-clients">
              {Object.values(labels).map((label) => (
                <div className="preview-client" key={label}>
                  <strong>{label}</strong>
                  <span className="muted small">等待本机检测</span>
                </div>
              ))}
            </div>
          )}
          {clients.isError && (
            <p role="alert" className="error">
              无法读取客户端配置，请检查文件内容后刷新重试
            </p>
          )}
          <fieldset
            disabled={pending || clientsUnavailable}
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
                      ? '已配置 PluginPocket'
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
              disabled={configurationDisabled || !account}
            >
              <Link aria-hidden="true" data-icon="inline-start" />
              配置所选客户端
            </Button>
            <Button
              variant="outline"
              onClick={() => configure.mutate(true)}
              disabled={configurationDisabled}
            >
              <Unplug aria-hidden="true" data-icon="inline-start" />
              移除所选配置
            </Button>
          </div>
          <p className="muted small">
            已有配置会先备份；遇到手写的 PluginPocket 条目时会停止，供你确认处理
          </p>
        </section>
      </div>
      {native && <UpdatePanel />}
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
    </div>
  );
}
