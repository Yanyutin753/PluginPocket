import { ArrowDownToLine, ExternalLink, Monitor, Terminal } from 'lucide-react';
import { Link } from 'react-router';
import { Button } from '../components/ui/button';
import { useI18n } from '../i18n';
import { PublicHeader } from './PublicHeader';
import './DownloadPage.css';

const releases = 'https://github.com/Yanyutin753/PluginPocket/releases/latest';

export default function DownloadPage() {
  const { t } = useI18n();
  return (
    <div className="download-page">
      <PublicHeader />
      <main className="download-main">
        <section className="download-hero" aria-labelledby="download-title">
          <div>
            <span className="download-eyebrow"><Monitor aria-hidden="true" />{t('PluginPocket 客户端')}</span>
            <h1 id="download-title">{t('把工作台带到你的桌面。')}</h1>
            <p>{t('使用桌面客户端管理装备、额度、日志与连接诊断；安装包从 GitHub Releases 获取。')}</p>
            <div className="download-actions">
              <Button asChild size="lg"><a href={releases} target="_blank" rel="noreferrer"><ArrowDownToLine aria-hidden="true" />{t('查看最新安装包')}</a></Button>
              <Link className="inline-action" to="/device">{t('查看连接说明')}<ExternalLink aria-hidden="true" /></Link>
            </div>
          </div>
          <img src="/images/workshop-desktop.webp" alt="" width="900" height="600" />
        </section>
        <section className="download-options" aria-labelledby="download-options-title">
          <div><Terminal aria-hidden="true" /><h2 id="download-options-title">{t('按你的平台开始')}</h2></div>
          <div className="download-grid">
            <a href={releases} target="_blank" rel="noreferrer"><strong>macOS</strong><span>{t('Apple Silicon 与 Intel')}</span></a>
            <a href={releases} target="_blank" rel="noreferrer"><strong>Windows</strong><span>{t('MSI 安装包')}</span></a>
            <a href={releases} target="_blank" rel="noreferrer"><strong>Linux</strong><span>{t('AppImage 与 DEB')}</span></a>
          </div>
        </section>
      </main>
    </div>
  );
}
