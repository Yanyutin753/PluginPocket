import { RefreshCw } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { Button } from './components/ui/button';
import { type UpdateInfo, type UpdateProgress, updater } from './updater';

type Phase =
  | 'idle'
  | 'checking'
  | 'available'
  | 'uptodate'
  | 'downloading'
  | 'ready'
  | 'error';

export function UpdatePanel() {
  const [phase, setPhase] = useState<Phase>('idle');
  const [version, setVersion] = useState('');
  const [info, setInfo] = useState<UpdateInfo | null>(null);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const installRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    let cancelled = false;
    updater
      .currentVersion()
      .then((value) => !cancelled && setVersion(value))
      .catch(() => {});
    // 启动静默检查：失败不打扰，手动检查才展示错误。
    updater
      .check()
      .then((result) => {
        if (cancelled) return;
        if (result.available) {
          setInfo(result.info);
          setVersion(result.info.currentVersion);
          setPhase('available');
        } else {
          setVersion(result.currentVersion);
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (phase === 'available') installRef.current?.focus();
  }, [phase]);

  async function check() {
    setPhase('checking');
    try {
      const result = await updater.check();
      if (result.available) {
        setInfo(result.info);
        setPhase('available');
      } else {
        setVersion(result.currentVersion);
        setInfo(null);
        setPhase('uptodate');
      }
    } catch {
      setPhase('error');
    }
  }

  async function install() {
    setPhase('downloading');
    setProgress(null);
    try {
      await updater.downloadAndInstall(setProgress);
      setPhase('ready');
    } catch {
      setPhase('error');
    }
  }

  async function relaunch() {
    try {
      await updater.relaunch();
    } catch {
      // Windows 由 passive 安装器负责重启；重启调用失败时停留在就绪态供用户手动处理。
    }
  }

  const percent =
    progress?.total && progress.total > 0
      ? Math.min(100, Math.floor((progress.received / progress.total) * 100))
      : null;

  return (
    <section className="update-panel" aria-labelledby="update-title">
      <h2 id="update-title">应用更新</h2>
      <div aria-live="polite">
        {phase === 'idle' && (
          <p className="update-status">
            当前版本 v{version || '…'}
            <Button onClick={check}>检查更新</Button>
          </p>
        )}
        {phase === 'checking' && (
          <p className="update-status" role="status">
            <RefreshCw aria-hidden="true" data-icon="inline-start" />
            正在检查更新…
          </p>
        )}
        {phase === 'available' && info && (
          <div className="update-available">
            <p>
              发现新版本 v{info.version}（当前 v{info.currentVersion || version}
              ）
            </p>
            {info.notes && <p className="small muted">{info.notes}</p>}
            <div className="update-actions">
              <Button ref={installRef} onClick={install}>
                下载并安装
              </Button>
              <Button variant="ghost" onClick={() => setPhase('idle')}>
                暂不更新
              </Button>
            </div>
          </div>
        )}
        {phase === 'uptodate' && (
          <p className="update-status" role="status">
            已是最新版本 v{version || '…'}
            <Button variant="outline" onClick={check}>
              再检查一次
            </Button>
          </p>
        )}
        {phase === 'downloading' && (
          <div className="update-downloading">
            <p role="status">
              正在下载更新
              {percent === null ? '…' : ` ${percent}%`}
            </p>
            <progress
              aria-label="下载进度"
              value={percent ?? undefined}
              max={100}
            />
          </div>
        )}
        {phase === 'ready' && (
          <div className="update-available">
            <p role="status">更新已安装，重启后生效。</p>
            <div className="update-actions">
              <Button onClick={relaunch}>立即重启</Button>
            </div>
          </div>
        )}
        {phase === 'error' && (
          <div className="update-available">
            <p role="alert" className="error">
              {info ? '更新未完成，请稍后重试' : '检查更新失败，请稍后重试'}
            </p>
            <div className="update-actions">
              {info ? (
                <Button onClick={install}>重试</Button>
              ) : (
                <Button onClick={check}>重试</Button>
              )}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
