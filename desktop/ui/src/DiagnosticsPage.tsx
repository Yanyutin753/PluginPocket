import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Activity,
  AlertCircle,
  CheckCircle2,
  CircleDashed,
  Play,
  ShieldCheck,
} from 'lucide-react';
import { useState } from 'react';
import { api } from './api';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { useI18n } from './i18n';

const pendingChecks = [
  { id: 'network', label: '服务连接', detail: '检查服务是否可达' },
  { id: 'auth', label: '账号凭证', detail: '验证已保存凭证是否有效' },
  {
    id: 'client-codex',
    label: 'Codex 配置',
    detail: '检查配置格式与 PluginPocket 接入状态',
  },
  {
    id: 'client-claude',
    label: 'Claude Code 配置',
    detail: '检查配置格式与 PluginPocket 接入状态',
  },
  {
    id: 'client-cursor',
    label: 'Cursor 配置',
    detail: '检查配置格式与 PluginPocket 接入状态',
  },
  {
    id: 'bridge',
    label: 'Bridge 可执行文件',
    detail: '检查本机程序是否存在，不执行工具调用',
  },
];
export function DiagnosticsPage({ native }: { native: boolean }) {
  const { t } = useI18n();
  const [server, setServer] = useState('');
  const cache = useQueryClient();
  const diagnosis = useMutation({
    mutationFn: () => api.diagnostics(server.trim() || null),
    onSettled: () => cache.invalidateQueries({ queryKey: ['logs'] }),
  });
  const checks =
    diagnosis.data && !diagnosis.isPending && !diagnosis.isError
      ? diagnosis.data.checks
      : pendingChecks.map((check) => ({ ...check, status: 'pending' }));
  return (
    <div>
      <form
        className="diagnostic-start"
        onSubmit={(event) => {
          event.preventDefault();
          if (native && !diagnosis.isPending) diagnosis.mutate();
        }}
      >
        <div className="diagnostic-start-icon">
          <Activity size={26} aria-hidden="true" />
        </div>
        <div className="diagnostic-input">
          <label htmlFor="diagnostic-server">{t('检查服务地址')}</label>
          <Input
            type="url"
            id="diagnostic-server"
            placeholder={t('留空使用本机已保存的服务')}
            value={server}
            onChange={(event) => {
              setServer(event.target.value);
              diagnosis.reset();
            }}
            disabled={!native || diagnosis.isPending}
          />
          <p className="muted small">
            只检查状态，不修改配置，也不产生工具调用费用。
          </p>
        </div>
        <Button type="submit" disabled={!native || diagnosis.isPending}>
          <Play aria-hidden="true" />
          {diagnosis.isPending ? '正在诊断…' : '运行诊断'}
        </Button>
      </form>
      <div className="list-section-heading">
        <h2>{t('检查项目')}</h2>
        <span className="muted small">
          {diagnosis.isPending
            ? '正在检查，请稍候'
            : diagnosis.isSuccess
              ? '本次诊断结果'
              : '运行后显示实际结果'}
        </span>
      </div>
      {diagnosis.isPending && (
        <p role="status" className="sr-only">
          正在检查服务、凭证和本地配置
        </p>
      )}
      {diagnosis.isError && (
        <div className="page-error" role="alert">
          <p>{t('诊断未完成，请检查本机环境后重新运行')}</p>
          <Button variant="outline" onClick={() => diagnosis.mutate()}>
            重新诊断
          </Button>
        </div>
      )}
      <ul className="diagnostic-list" aria-busy={diagnosis.isPending}>
        {checks.map((check) => (
          <li key={check.id}>
            <span
              className={`diagnostic-status-icon ${check.status}`}
              aria-hidden="true"
            >
              {check.status === 'ok' ? (
                <CheckCircle2 size={22} />
              ) : check.status === 'pending' ? (
                <CircleDashed size={22} />
              ) : (
                <AlertCircle size={22} />
              )}
            </span>
            <div>
              <h3>{check.label}</h3>
              <p>{check.detail}</p>
            </div>
            <span className={`check-result ${check.status}`}>
              {check.status === 'ok'
                ? '通过'
                : check.status === 'error'
                  ? '需要处理'
                  : check.status === 'warning'
                    ? '待完善'
                    : '待检查'}
            </span>
          </li>
        ))}
      </ul>
      <p className="page-footnote">
        <ShieldCheck size={16} aria-hidden="true" />
        每项结果独立显示。检查通过不代表已完成实际 MCP
        工具调用；客户端仍需在配置后重启。
      </p>
    </div>
  );
}
