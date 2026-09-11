import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Eye, EyeOff } from 'lucide-react';
import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router';
import { PreferencesControls } from '@/components/Preferences';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n';
import { metaQuery, request, resultSchema } from './api';
import { ErrorNotice } from './shared';

export default function AuthPage({ register = false }: { register?: boolean }) {
  const { t } = useI18n();
  const [showPassword, setShowPassword] = useState(false);
  const client = useQueryClient();
  const location = useLocation();
  const meta = useQuery(metaQuery);
  const navigate = useNavigate();
  const auth = useMutation({
    mutationFn: (data: { username: string; password: string }) =>
      request(`/auth/${register ? 'register' : 'login'}`, resultSchema, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      client.clear();
      const from = location.state?.from;
      navigate(
        typeof from === 'string' &&
          from.startsWith('/') &&
          !from.startsWith('//') &&
          !['/login', '/register'].includes(from)
          ? from
          : '/overview',
        { replace: true },
      );
    },
  });
  return (
    <main className="auth-page">
      <header className="auth-topbar">
        <Link className="brand" to="/">
          <img
            className="workshop-mark"
            src="/images/workshop-mark.webp"
            alt=""
            width="40"
            height="40"
          />
          <span className="brand-wordmark">Loadout</span>
        </Link>
        <PreferencesControls />
      </header>
      <div className="auth-card">
        <section className="auth-welcome">
          <div className="auth-welcome-copy">
            <h2>{t('给你的 AI，装上超能力。')}</h2>
            <p>{t('统一管理 MCP 工具，接入 Codex、Claude Code 和 Cursor。')}</p>
          </div>
          <img
            src={
              register
                ? '/images/workshop-register.webp'
                : '/images/workshop-login.webp'
            }
            alt=""
            width="900"
            height="600"
            fetchPriority="high"
          />
          <ul
            className="auth-client-list"
            aria-label={t('支持主流 AI 开发客户端')}
          >
            <li>Codex</li>
            <li>Claude Code</li>
            <li>Cursor</li>
          </ul>
        </section>
        <section
          className="auth-form-panel"
          aria-label={t(register ? '注册表单' : '登录表单')}
        >
          <div className="auth-form-heading">
            <img
              className="workshop-mark"
              src="/images/workshop-mark.webp"
              alt=""
              width="80"
              height="80"
            />
            <h1>{t(register ? '创建 Loadout 账号' : '登录 Loadout')}</h1>
            <p>{t('少一点配置，多一点创造。')}</p>
          </div>
          <form
            className="form-stack"
            onSubmit={(event) => {
              event.preventDefault();
              const data = new FormData(event.currentTarget);
              auth.mutate({
                username: String(data.get('username')),
                password: String(data.get('password')),
              });
            }}
          >
            <FieldGroup className="min-w-0 flex-1">
              <Field className="field">
                <FieldLabel htmlFor="username">{t('用户名')}</FieldLabel>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  required
                  minLength={3}
                  maxLength={32}
                  pattern="[A-Za-z0-9_-]+"
                  disabled={auth.isPending}
                  aria-describedby={register ? 'username-hint' : undefined}
                />
                {register && (
                  <p id="username-hint">
                    {t('3–32 位字母、数字、下划线或横线。')}
                  </p>
                )}
              </Field>
              <Field className="field">
                <FieldLabel htmlFor="password">{t('密码')}</FieldLabel>
                <div className="password-field">
                  <Input
                    id="password"
                    name="password"
                    spellCheck={false}
                    autoCapitalize="none"
                    autoCorrect="off"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete={
                      register ? 'new-password' : 'current-password'
                    }
                    required
                    minLength={register ? 12 : 1}
                    maxLength={1024}
                    disabled={auth.isPending}
                    aria-describedby={register ? 'password-hint' : undefined}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    className="password-toggle"
                    aria-label={t(showPassword ? '隐藏密码' : '显示密码')}
                    aria-pressed={showPassword}
                    aria-controls="password"
                    disabled={auth.isPending}
                    onClick={() => setShowPassword((value) => !value)}
                  >
                    {showPassword ? (
                      <EyeOff aria-hidden="true" />
                    ) : (
                      <Eye aria-hidden="true" />
                    )}
                  </Button>
                </div>
                {register && (
                  <p id="password-hint">
                    {t('至少 12 个字符，建议使用密码管理器生成。')}
                  </p>
                )}
              </Field>
            </FieldGroup>
            <ErrorNotice error={auth.error} />
            <Button type="submit" disabled={auth.isPending}>
              {t(auth.isPending ? '正在提交…' : register ? '创建账号' : '登录')}
              <ArrowRight aria-hidden="true" />
            </Button>
          </form>
          {meta.data?.github && (
            <a className="inline-action" href="/api/v1/auth/github/start">
              {t('通过 GitHub 登录')}
            </a>
          )}
          <p className="auth-switch">
            {t(register ? '已有账号？' : '第一次使用？')}
            <Link
              state={location.state}
              to={register ? '/login' : '/register'}
              onClick={() => auth.reset()}
            >
              {t(register ? '去登录' : '创建账号')}
            </Link>
          </p>
          <Link className="auth-help" to="/health">
            {t('检查服务连接')}
          </Link>
        </section>
      </div>
      <p className="auth-tagline">Your AI, fully loaded.</p>
    </main>
  );
}
