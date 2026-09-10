import {
  QueryClient,
  QueryClientProvider,
  useQueryClient,
} from '@tanstack/react-query';
import {
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react';
import { accountQuery } from './features/account/api';

export function SessionBoundary({ children }: { children: ReactNode }) {
  const initialClient = useQueryClient();
  const [session, setSession] = useState({
    client: initialClient,
    identity: undefined as number | undefined,
    generation: 0,
  });
  const subscribe = useCallback(
    (notify: () => void) => session.client.getQueryCache().subscribe(notify),
    [session.client],
  );
  const snapshot = useCallback(
    () => session.client.getQueryData(accountQuery.queryKey),
    [session.client],
  );
  const account = useSyncExternalStore(subscribe, snapshot);
  const previousClient = useRef(session.client);
  useEffect(() => {
    if (previousClient.current !== session.client) {
      previousClient.current.clear();
      previousClient.current = session.client;
    }
  }, [session.client]);
  if (account && account.user.id !== session.identity) {
    if (session.identity === undefined) {
      setSession({ ...session, identity: account.user.id });
    } else {
      // Retired mutations can finish only against their retired client.
      const client = new QueryClient({
        defaultOptions: session.client.getDefaultOptions(),
      });
      client.setQueryData(accountQuery.queryKey, account);
      setSession({
        client,
        identity: account.user.id,
        generation: session.generation + 1,
      });
    }
  }
  return (
    <QueryClientProvider key={session.generation} client={session.client}>
      {children}
    </QueryClientProvider>
  );
}
