// Temporary development-only visual fixture. Delete after visual review.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot } from 'react-dom/client';
import { Link, MemoryRouter, Route, Routes } from 'react-router';
import { PreferencesControls } from './components/Preferences';
import UsagePage from './features/account/UsagePage';
import TeamsPage from './features/operations/TeamsPage';
import { PreferencesProvider } from './i18n';
import './styles.css';

const team = {
  id: 1,
  name: '研发组 · 视觉测试',
  role: 'owner',
  balance: 100,
  seat_limit: 5,
};
const members = [
  { id: 1, user_id: 1, username: 'alice', role: 'owner' },
  { id: 2, user_id: 2, username: 'bob', role: 'member' },
  { id: 3, user_id: 3, username: 'chen_design', role: 'member' },
];
const usage = ['ok', 'error', 'recovered', 'pending', 'denied'].map(
  (status, index) => ({
    id: index + 1,
    user_id: (index % 3) + 1,
    tool: index % 2 ? 'search_documents' : 'echo',
    cost: status === 'ok' ? 2 : 0,
    status,
    duration_ms: 120 + index * 40,
    created_at: `2026-09-10T10:0${index}:00Z`,
  }),
);

// Never retain or delegate to the real fetch: every request stays inside this fixture.
window.fetch = async (input, init = {}) => {
  const method = (
    init.method ?? (input instanceof Request ? input.method : 'GET')
  ).toUpperCase();
  if (method !== 'GET')
    return Response.json({ error: 'forbidden' }, { status: 403 });
  const url = new URL(
    input instanceof Request ? input.url : String(input),
    window.location.origin,
  );
  const path = url.pathname.replace(/^\/api\/v1/, '');
  if (path === '/account/me')
    return Response.json({
      user: {
        id: 1,
        username: 'alice',
        role: 'user',
        balance: 500,
        enabled: true,
      },
      summary: { today_calls: 5, month_cost: 2, token_count: 1 },
    });
  if (path === '/account/teams/1') return Response.json({ item: team });
  if (path === '/account/teams')
    return Response.json({ items: [team], next_cursor: '' });
  if (path === '/account/teams/1/members')
    return Response.json({ items: members, next_cursor: '' });
  if (path === '/account/teams/1/usage/summary')
    return Response.json({
      next_cursor: '',
      items: members.map((member, index) => ({
        user_id: member.user_id,
        username: member.username,
        calls: 12 + index,
        cost: 8 + index,
        errors: index,
      })),
    });
  if (path === '/account/teams/1/usage')
    return Response.json({
      items: usage.filter(
        (row) =>
          (!url.searchParams.get('tool') ||
            row.tool === url.searchParams.get('tool')) &&
          (!url.searchParams.get('status') ||
            row.status === url.searchParams.get('status')) &&
          (!url.searchParams.get('user_id') ||
            String(row.user_id) === url.searchParams.get('user_id')),
      ),
      next_cursor: '',
    });
  return Response.json({ error: 'forbidden' }, { status: 403 });
};

const root = document.getElementById('root');
if (!root) throw new Error('Missing fixture root');
const client = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});
createRoot(root).render(
  <QueryClientProvider client={client}>
    <PreferencesProvider>
      <MemoryRouter initialEntries={['/teams/1']}>
        <div
          style={{
            maxWidth: '96rem',
            margin: '0 auto',
            padding: 'clamp(1rem, 3vw, 3rem)',
          }}
        >
          <aside className="editor-panel" aria-label="视觉测试说明">
            <strong>仅视觉 QA · 以下全部为测试数据</strong>
            <p>
              真实页面组件，所有请求均由本页拦截。写入和导出被拒绝，不连接真实
              API。
            </p>
            <div className="action-row">
              <Link className="inline-action" to="/teams/1">
                返回团队详情
              </Link>
              <Link className="inline-action" to="/teams/1/usage">
                查看团队用量
              </Link>
              <PreferencesControls />
            </div>
          </aside>
          <main className="app-main">
            <Routes>
              <Route path="/teams" element={<TeamsPage />} />
              <Route path="/teams/:id" element={<TeamsPage />} />
              <Route path="/teams/:id/usage" element={<UsagePage team />} />
              <Route
                path="*"
                element={
                  <p>此临时入口仅覆盖团队详情和用量，请使用上方返回按钮。</p>
                }
              />
            </Routes>
          </main>
        </div>
      </MemoryRouter>
    </PreferencesProvider>
  </QueryClientProvider>,
);
