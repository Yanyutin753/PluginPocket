import { useMutation, useQuery } from '@tanstack/react-query';
import {
  BookOpen,
  Copy,
  Download,
  RefreshCw,
  Search,
  ShieldCheck,
} from 'lucide-react';
import { useState } from 'react';
import { api } from './api';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';

const actions: Record<string, string> = {
  login: '账号登录',
  logout: '退出登录',
  apply: '配置客户端',
  remove: '移除配置',
  install: '安装装备',
  update: '更新装备',
  uninstall: '卸载装备',
  update_installed: '更新装备',
  uninstall_installed: '卸载装备',
  diagnostics: '连接诊断',
  doctor: '连接检查',
  bridge: 'Bridge',
};
export function LogsPage({ native }: { native: boolean }) {
  const logs = useQuery({
    queryKey: ['logs'],
    queryFn: api.logs,
    enabled: native,
  });
  const [search, setSearch] = useState('');
  const [level, setLevel] = useState('all');
  const [feedback, setFeedback] = useState('');
  const [failed, setFailed] = useState(false);
  const [copying, setCopying] = useState(false);
  const [exportPath, setExportPath] = useState('');
  const exportLogs = useMutation({
    mutationFn: api.exportLogs,
    onMutate: () => {
      setFeedback('');
      setFailed(false);
      setExportPath('');
    },
    onSuccess: (result) => {
      setFeedback(`${result.count} 条日志已保存到`);
      setExportPath(result.path);
    },
    onError: () => {
      setFailed(true);
      setFeedback('导出失败，请刷新日志后重试，或复制日志');
    },
  });
  const visible = (logs.isError ? [] : (logs.data ?? []))
    .filter(
      (item) =>
        (level === 'all' || item.level === level) &&
        `${item.message} ${actions[item.action] ?? item.action}`
          .toLowerCase()
          .includes(search.trim().toLowerCase()),
    )
    .sort((a, b) => b.timestamp - a.timestamp);
  const text = visible
    .map(
      (item) =>
        `${new Date(item.timestamp).toISOString()} [${item.level.toUpperCase()}] ${item.action}: ${item.message}`,
    )
    .join('\n');
  async function copy() {
    setFeedback('');
    setFailed(false);
    setExportPath('');
    setCopying(true);
    try {
      await navigator.clipboard.writeText(text);
      setFeedback(`已复制 ${visible.length} 条日志`);
    } catch {
      setFailed(true);
      setFeedback('复制失败，请重试或导出日志');
    } finally {
      setCopying(false);
    }
  }
  return (
    <div>
      <div className="page-toolbar">
        <label className="search-field" htmlFor="logs-search">
          <Search size={18} aria-hidden="true" />
          <Input
            id="logs-search"
            type="search"
            aria-label="搜索日志"
            placeholder="搜索操作或日志内容…"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setFeedback('');
            }}
          />
        </label>
        <label className="filter-select">
          <span className="sr-only">日志级别</span>
          <select
            aria-label="日志级别"
            value={level}
            onChange={(event) => {
              setLevel(event.target.value);
              setFeedback('');
            }}
          >
            <option value="all">全部级别</option>
            <option value="info">信息</option>
            <option value="error">错误</option>
          </select>
        </label>
        <Button
          variant="outline"
          onClick={() => void logs.refetch()}
          disabled={!native || logs.isFetching}
        >
          <RefreshCw aria-hidden="true" />
          刷新日志
        </Button>
      </div>
      <div className="log-panel">
        <div className="log-panel-heading">
          <div>
            <strong>本机运行记录</strong>
            <span className="small muted">
              {native && logs.isSuccess
                ? `${visible.length} 条 · 最新在前`
                : '登录 · 配置 · 装备 · Bridge'}
            </span>
          </div>
          <div className="log-actions">
            <Button
              variant="ghost"
              onClick={() => void copy()}
              disabled={visible.length === 0 || copying}
            >
              <Copy aria-hidden="true" />
              复制日志
            </Button>
            <Button
              variant="outline"
              onClick={() => exportLogs.mutate(visible)}
              disabled={visible.length === 0 || exportLogs.isPending}
            >
              <Download aria-hidden="true" />
              {exportLogs.isPending ? '导出中…' : '导出日志'}
            </Button>
          </div>
        </div>
        {native && logs.isPending && (
          <p role="status" className="page-loading">
            正在读取运行日志…
          </p>
        )}
        {native && logs.isError && (
          <div className="page-error" role="alert">
            <p>无法读取运行日志，请重试</p>
            <Button variant="outline" onClick={() => void logs.refetch()}>
              重试读取日志
            </Button>
          </div>
        )}
        {(!native || (logs.isSuccess && visible.length === 0)) && (
          <div className="empty-state">
            <BookOpen size={34} aria-hidden="true" />
            <h2>
              {!native
                ? '运行记录，随时可查'
                : logs.data?.length
                  ? '没有匹配的日志'
                  : '还没有运行日志'}
            </h2>
            <p>
              {!native
                ? '在桌面应用中读取这台电脑的真实操作记录。关闭窗口或重启后，记录依然保留。'
                : logs.data?.length
                  ? '更换关键词或日志级别，再试一次。'
                  : '登录、配置或管理装备后，可以在这里查看结果。'}
            </p>
          </div>
        )}
        {visible.length > 0 && (
          <div className="log-table-wrap">
            <table className="log-table">
              <caption className="sr-only">筛选后的本机运行日志</caption>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>级别</th>
                  <th>操作</th>
                  <th>记录</th>
                </tr>
              </thead>
              <tbody>
                {visible.map((item, index) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: immutable log rows can share a timestamp and content; they carry no component state.
                  <tr key={`${item.timestamp}:${index}`}>
                    <td>
                      <time dateTime={new Date(item.timestamp).toISOString()}>
                        {new Date(item.timestamp).toLocaleString('zh-CN', {
                          month: '2-digit',
                          day: '2-digit',
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                          hour12: false,
                        })}
                      </time>
                    </td>
                    <td>
                      <span className={`log-level ${item.level}`}>
                        {item.level === 'error' ? '错误' : '信息'}
                      </span>
                    </td>
                    <td>{actions[item.action] ?? item.action}</td>
                    <td>{item.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <div className="feedback" aria-live="polite">
        {feedback && (
          <p
            role={failed ? 'alert' : 'status'}
            className={failed ? 'error' : undefined}
          >
            {feedback}
          </p>
        )}
        {exportPath && (
          <p className="export-path">
            <code>{exportPath}</code>
          </p>
        )}
      </div>
      <p className="page-footnote">
        <ShieldCheck size={16} aria-hidden="true" />
        仅记录安全的操作摘要，不保存令牌或工具输入输出。复制与导出遵循当前筛选。
      </p>
    </div>
  );
}
