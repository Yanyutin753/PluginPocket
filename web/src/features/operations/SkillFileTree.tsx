import {
  ChevronDown,
  ChevronRight,
  FileCode2,
  FileImage,
  FileText,
  Folder,
  FolderOpen,
} from 'lucide-react';
import { useState } from 'react';
import { useI18n } from '@/i18n';
import { imageType } from '@/lib/files';

export function SkillFileIcon({ path }: { path: string }) {
  const Icon = imageType(path)
    ? FileImage
    : /\.(md|txt|markdown)$/i.test(path)
      ? FileText
      : FileCode2;
  return (
    <Icon
      className={
        imageType(path)
          ? 'skill-icon-image'
          : /\.md$/i.test(path)
            ? 'skill-icon-markdown'
            : 'skill-icon-code'
      }
      aria-hidden="true"
      size={16}
    />
  );
}

export function SkillFileTree({
  paths,
  selected,
  modified,
  select,
  disabled,
  prefix = '',
}: {
  paths: string[];
  selected: string;
  modified: Set<string>;
  select: (path: string) => void;
  disabled: boolean;
  prefix?: string;
}) {
  const { t } = useI18n();
  const [closed, setClosed] = useState<string[]>([]);
  const names = [
    ...new Set(
      paths
        .filter((path) => path.startsWith(prefix))
        .map((path) => path.slice(prefix.length).split('/')[0]),
    ),
  ].sort((a, b) =>
    a === 'SKILL.md' ? -1 : b === 'SKILL.md' ? 1 : a.localeCompare(b),
  );
  return (
    <ul className="skill-tree-list">
      {names.map((name) => {
        const path = `${prefix}${name}`;
        const folder = paths.some((entry) => entry.startsWith(`${path}/`));
        const expanded = !closed.includes(path);
        return (
          <li key={path}>
            {folder ? (
              <>
                <button
                  type="button"
                  className="skill-tree-folder"
                  aria-label={t('文件夹 {path}', { path })}
                  aria-expanded={expanded}
                  onClick={() =>
                    setClosed(
                      expanded
                        ? [...closed, path]
                        : closed.filter((entry) => entry !== path),
                    )
                  }
                >
                  {expanded ? (
                    <ChevronDown aria-hidden="true" size={14} />
                  ) : (
                    <ChevronRight aria-hidden="true" size={14} />
                  )}
                  {expanded ? (
                    <FolderOpen aria-hidden="true" size={16} />
                  ) : (
                    <Folder aria-hidden="true" size={16} />
                  )}
                  <span>{name}</span>
                </button>
                {expanded && (
                  <SkillFileTree
                    paths={paths}
                    prefix={`${path}/`}
                    selected={selected}
                    modified={modified}
                    select={select}
                    disabled={disabled}
                  />
                )}
              </>
            ) : (
              <button
                type="button"
                className="skill-tree-file"
                aria-label={path}
                aria-pressed={selected === path}
                disabled={disabled}
                onClick={() => select(path)}
                title={path}
              >
                <SkillFileIcon path={path} />
                <span>{name}</span>
                {modified.has(path) && (
                  <span
                    className="skill-modified-dot"
                    title={t('未保存')}
                    aria-hidden="true"
                  />
                )}
              </button>
            )}
          </li>
        );
      })}
    </ul>
  );
}
