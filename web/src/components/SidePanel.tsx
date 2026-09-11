import { ChevronRight, Maximize2, Minimize2, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useRef,
  useState,
} from 'react';
import { useI18n } from '@/i18n';
import { Button } from './ui/button';
import './SidePanel.css';

const PanelFullscreen = createContext({
  fullscreen: false,
  toggle: () => {},
  registerEscape: (_handler: () => boolean) => () => {},
});
export const usePanelFullscreen = () => useContext(PanelFullscreen);

export function SidePanel({
  title,
  children,
  trigger,
  onClose,
  locked = false,
  className = '',
}: {
  title: string;
  children: ReactNode;
  trigger?: string;
  onClose?: () => void;
  locked?: boolean;
  className?: string;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(!trigger);
  const [fullscreen, setFullscreen] = useState(false);
  const localEscape = useRef<(() => boolean) | null>(null);
  const registerEscape = useCallback((handler: () => boolean) => {
    localEscape.current = handler;
    return () => {
      if (localEscape.current === handler) localEscape.current = null;
    };
  }, []);
  const [returnFocus] = useState(() => document.activeElement);
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next && locked) return;
        setOpen(next);
        if (!next) onClose?.();
      }}
    >
      {trigger && (
        <Dialog.Trigger asChild>
          <Button variant="outline" className="panel-trigger">
            {trigger}
            <ChevronRight aria-hidden="true" />
          </Button>
        </Dialog.Trigger>
      )}
      <Dialog.Portal>
        <Dialog.Overlay className="side-panel-overlay" />
        <Dialog.Content
          className={`side-panel ${className}`}
          data-fullscreen={fullscreen}
          onEscapeKeyDown={(event) => {
            if (localEscape.current?.()) {
              event.preventDefault();
              return;
            }
            if (fullscreen) {
              event.preventDefault();
              setFullscreen(false);
            }
          }}
          aria-describedby={undefined}
          onCloseAutoFocus={
            trigger
              ? undefined
              : (event) => {
                  event.preventDefault();
                  if (
                    returnFocus instanceof HTMLElement &&
                    returnFocus.isConnected
                  )
                    returnFocus.focus();
                }
          }
        >
          <header className="side-panel-heading">
            <Dialog.Title>{title}</Dialog.Title>
            <div className="side-panel-actions">
              <Button
                type="button"
                variant="ghost"
                aria-label={t(fullscreen ? '退出全屏' : '全屏')}
                title={t(fullscreen ? '退出全屏' : '全屏')}
                aria-pressed={fullscreen}
                onClick={() => setFullscreen(!fullscreen)}
              >
                {fullscreen ? (
                  <Minimize2 aria-hidden="true" />
                ) : (
                  <Maximize2 aria-hidden="true" />
                )}
              </Button>
              <Dialog.Close asChild>
                <Button
                  variant="ghost"
                  disabled={locked}
                  aria-label={t('关闭面板')}
                >
                  <X aria-hidden="true" />
                </Button>
              </Dialog.Close>
            </div>
          </header>
          <PanelFullscreen
            value={{
              fullscreen,
              toggle: () => setFullscreen(!fullscreen),
              registerEscape,
            }}
          >
            <div className="side-panel-body">{children}</div>
          </PanelFullscreen>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
