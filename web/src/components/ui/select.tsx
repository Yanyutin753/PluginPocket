import { Check, ChevronDown, ChevronUp } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';
import { type ReactNode, useEffect, useRef, useState } from 'react';
import './select.css';

export type SelectProps = {
  id?: string;
  name?: string;
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  options: Array<{ value: string; label: string; disabled?: boolean }>;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
  icon?: ReactNode;
  'aria-label'?: string;
  'aria-describedby'?: string;
};

export function Select({
  options,
  value,
  defaultValue = '',
  onValueChange,
  placeholder,
  disabled,
  name,
  className,
  icon,
  ...triggerProps
}: SelectProps) {
  const [localValue, setLocalValue] = useState(defaultValue);
  const [pointerFocus, setPointerFocus] = useState(false);
  const selected = value ?? localValue;
  const input = useRef<HTMLInputElement>(null);
  const open = useRef(false);
  useEffect(() => {
    const form = input.current?.form;
    if (value !== undefined || !form) return;
    const reset = () => setLocalValue(defaultValue);
    form.addEventListener('reset', reset);
    return () => form.removeEventListener('reset', reset);
  }, [defaultValue, value]);
  // Encode every option, so even a real "value:" option cannot collide with ''.
  const internalValue =
    selected === '' && !options.some((option) => option.value === '')
      ? ''
      : `value:${selected}`;
  return (
    <>
      <input
        ref={input}
        type="hidden"
        name={name}
        value={selected}
        disabled={disabled}
      />
      <SelectPrimitive.Root
        onOpenChange={(next) => {
          open.current = next;
        }}
        value={internalValue}
        disabled={disabled}
        onValueChange={(next) => {
          const decoded = next.slice('value:'.length);
          if (value === undefined) setLocalValue(decoded);
          onValueChange?.(decoded);
        }}
      >
        <SelectPrimitive.Trigger
          {...triggerProps}
          data-slot="select-trigger"
          data-pointer-focus={pointerFocus || undefined}
          onPointerDown={() => setPointerFocus(true)}
          onKeyDown={() => setPointerFocus(false)}
          onBlur={() => {
            if (!open.current) setPointerFocus(false);
          }}
          className={
            className ? `select-trigger ${className}` : 'select-trigger'
          }
        >
          {icon && (
            <span className="select-leading-icon" aria-hidden="true">
              {icon}
            </span>
          )}
          <span className="select-label">
            <SelectPrimitive.Value placeholder={placeholder} />
          </span>
          <SelectPrimitive.Icon asChild>
            <ChevronDown aria-hidden="true" />
          </SelectPrimitive.Icon>
        </SelectPrimitive.Trigger>
        <SelectPrimitive.Portal>
          <SelectPrimitive.Content
            className="select-content"
            data-pointer-focus={pointerFocus || undefined}
            onPointerDownCapture={() => setPointerFocus(true)}
            onKeyDownCapture={() => setPointerFocus(false)}
            position="popper"
            sideOffset={6}
            collisionPadding={12}
          >
            <SelectPrimitive.ScrollUpButton className="select-scroll">
              <ChevronUp aria-hidden="true" />
            </SelectPrimitive.ScrollUpButton>
            <SelectPrimitive.Viewport className="select-viewport">
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={`value:${option.value}`}
                  disabled={option.disabled}
                  className="select-option"
                >
                  <SelectPrimitive.ItemText>
                    {option.label}
                  </SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className="select-check">
                    <Check aria-hidden="true" />
                  </SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.Viewport>
            <SelectPrimitive.ScrollDownButton className="select-scroll">
              <ChevronDown aria-hidden="true" />
            </SelectPrimitive.ScrollDownButton>
          </SelectPrimitive.Content>
        </SelectPrimitive.Portal>
      </SelectPrimitive.Root>
    </>
  );
}
