import { ChevronRight, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import { type ReactNode, useState } from 'react';
import { useI18n } from '@/i18n';
import { Button } from './ui/button';
import './SidePanel.css';

export function SidePanel({
  title,
  children,
  trigger,
  onClose,
  locked = false,
}: {
  title: string;
  children: ReactNode;
  trigger?: string;
  onClose?: () => void;
  locked?: boolean;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(!trigger);
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
          className="side-panel"
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
            <Dialog.Close asChild>
              <Button
                variant="ghost"
                disabled={locked}
                aria-label={t('关闭面板')}
              >
                <X aria-hidden="true" />
              </Button>
            </Dialog.Close>
          </header>
          <div className="side-panel-body">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
