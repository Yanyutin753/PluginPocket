import { useInfiniteQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import type { z } from 'zod';
import { listOptions } from './api';

export function usePagedList<T extends z.ZodType>(
  path: string,
  schema: T,
  enabled = true,
) {
  const [pageSize, setPageSize] = useState(10);
  const key = `${path}:${pageSize}`;
  const [view, setView] = useState({ key, index: 0 });
  if (view.key !== key) setView({ key, index: 0 });
  const currentKey = useRef(key);
  currentKey.current = key;
  const fetchingNext = useRef(false);
  const query = useInfiniteQuery({
    ...listOptions(path, schema, pageSize),
    enabled,
  });
  const pages = query.data?.pages ?? [];
  const index =
    view.key === key ? Math.min(view.index, Math.max(0, pages.length - 1)) : 0;
  const items = pages[index]?.items ?? [];
  const content = useRef<HTMLElement | null>(null);
  const previousPage = useRef(`${key}:${index}`);
  useEffect(() => {
    const current = `${key}:${index}`;
    if (previousPage.current !== current && content.current) {
      content.current.scrollTop = 0;
      content.current.scrollIntoView({ block: 'start' });
      content.current.focus({ preventScroll: true });
    }
    previousPage.current = current;
  }, [key, index]);
  const offset = pages
    .slice(0, index)
    .reduce((sum, page) => sum + page.items.length, 0);
  const next = async () => {
    if (query.isFetching || fetchingNext.current) return;
    if (index < pages.length - 1) {
      setView({ key, index: index + 1 });
      return;
    }
    if (!query.hasNextPage) return;
    fetchingNext.current = true;
    try {
      const result = await query.fetchNextPage();
      if (
        currentKey.current === key &&
        !result.isError &&
        result.data &&
        result.data.pages.length > index + 1
      ) {
        setView({ key, index: index + 1 });
      }
    } finally {
      fetchingNext.current = false;
    }
  };
  return {
    ...query,
    items,
    listRef: (node: HTMLElement | null) => {
      content.current = node;
    },
    retry: () => (query.isFetchNextPageError ? next() : query.refetch()),
    pagination: {
      page: index + 1,
      pageSize,
      start: items.length ? offset + 1 : 0,
      end: offset + items.length,
      hasData: Boolean(query.data),
      hasNext: index < pages.length - 1 || query.hasNextPage,
      pending: query.isFetching || query.fetchStatus === 'paused',
      previous: () => setView({ key, index: Math.max(0, index - 1) }),
      next,
      setPageSize: (size: number) => {
        setView({ key: `${path}:${size}`, index: 0 });
        setPageSize(size);
      },
    },
  };
}
