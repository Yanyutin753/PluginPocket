import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Activity,
  ArrowUpRight,
  ChartNoAxesCombined,
  CreditCard,
  House,
  KeyRound,
  Layers,
  LogOut,
  Menu,
  Monitor,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  ShieldCheck,
  Users,
} from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { lazy, type ReactNode, Suspense, useEffect, useState } from 'react';
import {
  BrowserRouter,
  Link,
  Navigate,
  NavLink,
  Outlet,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from 'react-router';
import { z } from 'zod';
import { PreferencesControls } from './components/Preferences';
import { Button } from './components/ui/button';
import { ApiError, accountQuery, request } from './features/account/api';
import { ErrorNotice, Loading } from './features/account/shared';
import { PreferencesProvider, useI18n } from './i18n';
import { SessionBoundary } from './SessionBoundary';

const LandingPage = lazy(() => import('./features/LandingPage'));
const HealthPage = lazy(() => import('./HealthPage'));
const AuthPage = lazy(() => import('./features/account/AuthPage'));
const OverviewPage = lazy(() => import('./features/account/OverviewPage'));
const TokensPage = lazy(() => import('./features/account/TokensPage'));
const UsagePage = lazy(() => import('./features/account/UsagePage'));
const AdminPage = lazy(() => import('./features/account/AdminPage'));
const ToolsPage = lazy(() => import('./features/operations/ToolsPage'));
const BillingPage = lazy(() => import('./features/operations/BillingPage'));
const PlansPage = lazy(() => import('./features/operations/PlansPage'));
const CodesPage = lazy(() => import('./features/operations/CodesPage'));
const TeamsPage = lazy(() => import('./features/operations/TeamsPage'));
const DevicePage = lazy(() => import('./features/operations/DevicePage'));
const LedgerPage = lazy(() => import('./features/operations/LedgerPage'));
const SettingsPage = lazy(() => import('./features/operations/SettingsPage'));
const SystemSettingsPage = lazy(
  () => import('./features/operations/SystemSettingsPage'),
);
const VerifyEmailPage = lazy(
  () => import('./features/operations/VerifyEmailPage'),
);
function AdminOnly({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const account = useQuery(accountQuery);
  return account.data?.user.role === 'admin' ? (
    children
  ) : (
    <header className="page-heading">
      <h1>{t('没有管理员权限')}</h1>
      <p>{t('请使用有权限的账号访问此页面。')}</p>
    </header>
  );
}
function SessionEvents() {
  const location = useLocation();
  const client = useQueryClient();
  const navigate = useNavigate();
  useEffect(() => {
    let handled = false;
    const expired = (error: unknown) => {
      if (
        !handled &&
        error instanceof ApiError &&
        error.status === 401 &&
        error.code !== 'invalid_credentials'
      ) {
        handled = true;
        client.clear();
        if (location.pathname !== '/login') {
          navigate('/login', {
            replace: true,
            state: { from: location.pathname + location.search },
          });
        }
        return true;
      }
      return false;
    };
    const queries = client.getQueryCache().subscribe((event) => {
      if (event.type === 'updated') expired(event.query.state.error);
    });
    const mutations = client.getMutationCache().subscribe((event) => {
      if (event.type === 'updated') expired(event.mutation.state.error);
    });
    // A fast response can arrive before this effect subscribes.
    for (const entry of [
      ...client.getQueryCache().getAll(),
      ...client.getMutationCache().getAll(),
    ]) {
      if (expired(entry.state.error)) break;
    }
    return () => {
      queries();
      mutations();
    };
  }, [client, navigate, location.pathname, location.search]);
  return null;
}
function Shell() {
  const { t } = useI18n();
  const location = useLocation();
  const account = useQuery(accountQuery);
  const client = useQueryClient();
  const navigate = useNavigate();
  const [expanded, setExpanded] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const logout = useMutation({
    mutationFn: () => request('/auth/logout', z.unknown(), { method: 'POST' }),
    onSuccess: () => {
      client.clear();
      navigate('/login', { replace: true });
    },
  });
  if (account.isPending)
    return (
      <main className="standalone">
        <Loading variant="page" />
      </main>
    );
  if (account.error)
    return (
      <main className="standalone">
        <ErrorNotice
          error={account.error}
          retry={() => void account.refetch()}
        />
      </main>
    );
  const links = [
    { to: '/overview', title: '概览', icon: House },
    { to: '/tokens', title: '网关令牌', icon: KeyRound },
    { to: '/usage', title: '用量明细', icon: ChartNoAxesCombined },
    { to: '/tools', title: '工具目录', icon: Layers },
    { to: '/billing', title: '额度与账单', icon: CreditCard },
    { to: '/teams', title: '我的团队', icon: Users },
    { to: '/device', title: '设备授权', icon: Monitor },
    { to: '/settings', title: '个人设置', icon: Settings },
    ...(account.data.user.role === 'admin'
      ? [
          { to: '/admin/users', title: '用户管理', icon: Users },
          { to: '/admin/tools', title: '工具管理', icon: Layers },
          { to: '/admin/plans', title: '套餐管理', icon: ChartNoAxesCombined },
          { to: '/admin/codes', title: '兑换码管理', icon: KeyRound },
          { to: '/admin/usage', title: '全局用量', icon: ChartNoAxesCombined },
          { to: '/admin/ledger', title: '额度审计', icon: ChartNoAxesCombined },
          { to: '/admin/settings', title: '系统配置', icon: Settings },
        ]
      : []),
    { to: '/health', title: '服务状态', icon: Activity },
  ];
  return (
    <div className={`app-shell${collapsed ? ' sidebar-collapsed' : ''}`}>
      <a className="skip-link" href="#main-content">
        {t('跳到主内容')}
      </a>
      <aside className="app-sidebar">
        <div className="brand-row">
          <Link to="/" className="brand" aria-label={t('Loadout 首页')}>
            <img
              className="workshop-mark"
              src="/images/workshop-mark.webp"
              alt=""
              width="48"
              height="48"
            />
            <span>
              <span className="brand-wordmark">Loadout</span>
              <small>{t('AI 装备工坊')}</small>
            </span>
          </Link>
          <Button
            variant="ghost"
            className="sidebar-collapse"
            aria-label={t(collapsed ? '展开侧栏' : '收起侧栏')}
            title={t(collapsed ? '展开侧栏' : '收起侧栏')}
            aria-expanded={!collapsed}
            aria-controls="main-nav"
            onClick={() => setCollapsed((value) => !value)}
          >
            {collapsed ? (
              <PanelLeftOpen aria-hidden="true" />
            ) : (
              <PanelLeftClose aria-hidden="true" />
            )}
          </Button>
          <Button
            variant="ghost"
            className="nav-toggle min-[900px]:hidden"
            aria-expanded={expanded}
            aria-controls="main-nav"
            onClick={() => setExpanded(!expanded)}
          >
            <Menu aria-hidden="true" />
            {t('导航')}
          </Button>
        </div>
        <nav
          id="main-nav"
          aria-label={t('主导航')}
          className={expanded ? 'main-nav expanded' : 'main-nav'}
        >
          {['工作空间', '管理后台', '账号与支持'].map((group) => (
            <fieldset aria-label={t(group)} className="nav-group" key={group}>
              <legend className="nav-section-label">{t(group)}</legend>
              {links
                .filter(({ to }) =>
                  group === '管理后台'
                    ? to.startsWith('/admin')
                    : group === '账号与支持'
                      ? ['/settings', '/health'].includes(to)
                      : !to.startsWith('/admin') &&
                        !['/settings', '/health'].includes(to),
                )
                .map(({ to, title, icon: Icon }) => (
                  <NavLink
                    key={to}
                    to={to}
                    end={to === '/overview'}
                    aria-label={t(title)}
                    title={collapsed ? t(title) : undefined}
                    onClick={() => setExpanded(false)}
                  >
                    <Icon aria-hidden="true" />
                    <span>{t(title)}</span>
                  </NavLink>
                ))}
            </fieldset>
          ))}
        </nav>
        <div className="sidebar-note">
          <ShieldCheck aria-hidden="true" />
          <p>{t('一处接入，你的整个 AI 工作流。')}</p>
          <Link to="/device">
            {t('连接客户端')}
            <ArrowUpRight aria-hidden="true" />
          </Link>
        </div>
        <Link
          className="sidebar-user"
          to="/settings"
          aria-label={t('账号设置')}
          title={account.data.user.username}
        >
          <span className="account-avatar" aria-hidden="true">
            <img
              src="/images/workshop-avatar.webp"
              alt=""
              width="32"
              height="32"
            />
          </span>
          <span className="sidebar-user-copy">
            <strong>{account.data.user.username}</strong>
            <small>{t('个人账号')}</small>
          </span>
          <Settings aria-hidden="true" />
        </Link>
      </aside>
      <div className="workspace-body">
        <header className="workspace-header">
          <div className="breadcrumb">
            <span>{t('个人工作空间')}</span>
            <span aria-hidden="true">/</span>
            <strong>
              {t(
                links.find((link) => link.to === location.pathname)?.title ??
                  '你的工作台',
              )}
            </strong>
          </div>
          <div className="workspace-header-actions">
            <PreferencesControls />
            <DropdownMenu.Root>
              <DropdownMenu.Trigger asChild>
                <Button
                  variant="ghost"
                  className="account-trigger"
                  aria-label={t('账号菜单')}
                >
                  <span className="account-avatar" aria-hidden="true">
                    <img
                      src="/images/workshop-avatar.webp"
                      alt=""
                      width="32"
                      height="32"
                    />
                  </span>
                  <span className="account-trigger-name">
                    {account.data.user.username}
                  </span>
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content
                  className="account-popover"
                  align="end"
                  sideOffset={8}
                  collisionPadding={12}
                >
                  <DropdownMenu.Label className="account-identity">
                    <span className="account-avatar" aria-hidden="true">
                      <img
                        src="/images/workshop-avatar.webp"
                        alt=""
                        width="32"
                        height="32"
                      />
                    </span>
                    <div>
                      <strong>{account.data.user.username}</strong>
                      <p>{t('个人账号')}</p>
                    </div>
                  </DropdownMenu.Label>
                  <DropdownMenu.Separator className="account-divider" />
                  <DropdownMenu.Item
                    className="account-logout"
                    onSelect={() => logout.mutate()}
                    disabled={logout.isPending}
                  >
                    <LogOut aria-hidden="true" />
                    {t(logout.isPending ? '正在退出…' : '退出登录')}
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>
          </div>
        </header>
        <main id="main-content" className="app-main">
          <ErrorNotice error={logout.error} />
          <Suspense fallback={<Loading variant="page" />}>
            <Outlet />
          </Suspense>
        </main>
        <footer className="workspace-footer">
          <div className="workspace-footer-inner">
            <div className="footer-brand">
              <img
                src="/images/workshop-mark.webp"
                alt=""
                width="28"
                height="28"
              />
              <span className="brand-wordmark">Loadout</span>
              <span className="footer-tagline">Your AI, fully loaded.</span>
            </div>
            <Link to="/health" className="footer-status">
              <Activity aria-hidden="true" />
              {t('服务状态')}
            </Link>
          </div>
        </footer>
      </div>
    </div>
  );
}
function AppRoutes() {
  const { t } = useI18n();
  return (
    <SessionBoundary>
      <BrowserRouter>
        <SessionEvents />
        <Suspense
          fallback={
            <main className="standalone">
              <Loading variant="page" />
            </main>
          }
        >
          <Routes>
            <Route path="/" element={<LandingPage />} />
            <Route path="/health" element={<HealthPage />} />
            <Route path="/login" element={<AuthPage />} />
            <Route path="/register" element={<AuthPage register />} />
            <Route path="/verify-email" element={<VerifyEmailPage />} />
            <Route element={<Shell />}>
              <Route path="overview" element={<OverviewPage />} />
              <Route path="tokens" element={<TokensPage />} />
              <Route path="usage" element={<UsagePage />} />
              <Route path="admin/users" element={<AdminPage />} />
              <Route path="tools" element={<ToolsPage />} />
              <Route path="billing" element={<BillingPage />} />
              <Route path="teams" element={<TeamsPage />} />
              <Route path="teams/:id" element={<TeamsPage />} />
              <Route path="teams/:id/usage" element={<UsagePage team />} />
              <Route path="device" element={<DevicePage />} />
              <Route path="devices" element={<DevicePage />} />
              <Route path="settings" element={<SettingsPage />} />
              <Route
                path="admin/settings"
                element={
                  <AdminOnly>
                    <SystemSettingsPage />
                  </AdminOnly>
                }
              />
              <Route
                path="admin/ledger"
                element={
                  <AdminOnly>
                    <LedgerPage />
                  </AdminOnly>
                }
              />
              <Route
                path="admin/tools"
                element={
                  <AdminOnly>
                    <ToolsPage admin />
                  </AdminOnly>
                }
              />
              <Route
                path="admin/plans"
                element={
                  <AdminOnly>
                    <PlansPage />
                  </AdminOnly>
                }
              />
              <Route
                path="admin/codes"
                element={
                  <AdminOnly>
                    <CodesPage />
                  </AdminOnly>
                }
              />
              <Route
                path="admin/usage"
                element={
                  <AdminOnly>
                    <UsagePage admin />
                  </AdminOnly>
                }
              />
            </Route>
            <Route
              path="*"
              element={
                <main className="standalone not-found">
                  <img
                    src="/images/workshop-empty.webp"
                    alt=""
                    width="900"
                    height="600"
                  />
                  <h1>{t('页面不存在')}</h1>
                  <Button asChild>
                    <Link to="/overview">{t('返回概览')}</Link>
                  </Button>
                </main>
              }
            />
            <Route
              path="/dashboard"
              element={<Navigate to="/overview" replace />}
            />
          </Routes>
        </Suspense>
      </BrowserRouter>
    </SessionBoundary>
  );
}

export default function App() {
  return (
    <PreferencesProvider>
      <AppRoutes />
    </PreferencesProvider>
  );
}
