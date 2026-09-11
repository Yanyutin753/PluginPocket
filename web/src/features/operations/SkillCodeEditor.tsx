import { closeCompletion } from '@codemirror/autocomplete';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { closeSearchPanel } from '@codemirror/search';
import { Compartment, EditorState, StateEffect } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { basicSetup } from 'codemirror';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { usePanelFullscreen } from '@/components/SidePanel';
import { useI18n } from '@/i18n';

const highlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: tags.heading, color: 'var(--code-heading)', fontWeight: '700' },
    { tag: tags.keyword, color: 'var(--code-keyword)' },
    { tag: [tags.string, tags.url], color: 'var(--code-string)' },
    {
      tag: [tags.number, tags.bool, tags.propertyName],
      color: 'var(--code-number)',
    },
    { tag: [tags.comment, tags.meta], color: 'var(--muted-foreground)' },
    { tag: tags.strong, fontWeight: '700' },
    { tag: tags.emphasis, fontStyle: 'italic' },
  ]),
);

export function SkillCodeEditor({
  path,
  value,
  label,
  disabled,
  wrap,
  focus,
  onChange,
  onCursor,
  sessions,
}: {
  path: string;
  value: string;
  label: string;
  disabled: boolean;
  wrap: boolean;
  focus: boolean;
  onChange: (value: string) => void;
  onCursor: (line: number, column: number) => void;
  sessions: Map<string, EditorState>;
}) {
  const { t } = useI18n();
  const { registerEscape } = usePanelFullscreen();
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const latest = useRef({ value, disabled, wrap, focus, onChange, onCursor });
  latest.current = { value, disabled, wrap, focus, onChange, onCursor };
  const options = useRef(new Compartment());
  const language = useRef(new Compartment());
  const [highlightError, setHighlightError] = useState(false);
  const [languageAttempt, setLanguageAttempt] = useState(0);
  useLayoutEffect(() => {
    if (!host.current) return;
    const extensions = [
      basicSetup,
      language.current.of([]),
      highlight,
      EditorView.contentAttributes.of({
        'aria-label': label,
        'aria-multiline': 'true',
        spellcheck: 'false',
      }),
      options.current.of([
        EditorState.readOnly.of(latest.current.disabled),
        EditorView.editable.of(!latest.current.disabled),
        ...(latest.current.wrap ? [EditorView.lineWrapping] : []),
      ]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged)
          latest.current.onChange(update.state.doc.toString());
        if (update.docChanged || update.selectionSet) {
          const position = update.state.selection.main.head;
          const line = update.state.doc.lineAt(position);
          latest.current.onCursor(line.number, position - line.from + 1);
        }
      }),
    ];
    const previous = sessions.get(path);
    const state =
      previous && previous.doc.toString() === latest.current.value
        ? previous.update({ effects: StateEffect.reconfigure.of(extensions) })
            .state
        : EditorState.create({ doc: latest.current.value, extensions });
    const instance = new EditorView({
      parent: host.current,
      state,
    });
    view.current = instance;
    if (latest.current.focus) instance.focus();
    return () => {
      sessions.set(path, instance.state);
      instance.destroy();
      view.current = null;
    };
  }, [path, label, sessions]);
  useEffect(
    () =>
      registerEscape(() => {
        const instance = view.current;
        if (!instance?.dom.contains(document.activeElement)) return false;
        return closeCompletion(instance) || closeSearchPanel(instance);
      }),
    [registerEscape],
  );
  // biome-ignore lint/correctness/useExhaustiveDependencies: Retry token and label recreation must reload the grammar into the current view.
  useEffect(() => {
    let cancelled = false;
    setHighlightError(false);
    const load = /\.(md|mdx|markdown)$/i.test(path)
      ? import('@codemirror/lang-markdown').then((module) => module.markdown())
      : /\.(js|jsx|ts|tsx|mjs|cjs)$/i.test(path)
        ? import('@codemirror/lang-javascript').then((module) =>
            module.javascript({
              typescript: /tsx?$/i.test(path),
              jsx: /[jt]sx$/i.test(path),
            }),
          )
        : /\.json$/i.test(path)
          ? import('@codemirror/lang-json').then((module) => module.json())
          : Promise.resolve([]);
    void load
      .then((extension) => {
        if (!cancelled)
          view.current?.dispatch({
            effects: language.current.reconfigure(extension),
          });
      })
      .catch(() => {
        if (!cancelled) setHighlightError(true);
      });
    return () => {
      cancelled = true;
    };
  }, [path, label, languageAttempt]);
  useEffect(() => {
    const instance = view.current;
    if (instance && instance.state.doc.toString() !== value)
      instance.dispatch({
        changes: { from: 0, to: instance.state.doc.length, insert: value },
      });
  }, [value]);
  useEffect(() => {
    view.current?.dispatch({
      effects: options.current.reconfigure([
        EditorState.readOnly.of(disabled),
        EditorView.editable.of(!disabled),
        ...(wrap ? [EditorView.lineWrapping] : []),
      ]),
    });
  }, [disabled, wrap]);
  return (
    <>
      {highlightError && (
        <button
          type="button"
          className="skill-highlight-retry"
          onClick={() => setLanguageAttempt((attempt) => attempt + 1)}
        >
          {t('语法高亮加载失败，点击重试')}
        </button>
      )}
      <div className="skill-code-editor" ref={host} />
    </>
  );
}
