// Temporary visual QA entry. Remove with qa-loading.html after inspection.
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { LoadingSkeleton } from './components/LoadingSkeleton';
import { PreferencesControls } from './components/Preferences';
import { PreferencesProvider } from './i18n';
import './styles.css';

function Preview() {
  const [variant, setVariant] = useState<'page' | 'form' | 'summary' | 'list'>(
    'page',
  );
  return (
    <PreferencesProvider>
      <div style={{ maxWidth: 1040, margin: '0 auto', padding: '16px' }}>
        <header
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 16,
            marginBottom: 24,
          }}
        >
          <label style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            骨架布局
            <select
              value={variant}
              onChange={(event) => {
                const value = event.target.value;
                if (
                  value === 'page' ||
                  value === 'form' ||
                  value === 'summary' ||
                  value === 'list'
                )
                  setVariant(value);
              }}
            >
              <option value="page">整页准备</option>
              <option value="list">列表</option>
              <option value="summary">指标</option>
              <option value="form">表单</option>
            </select>
          </label>
          <PreferencesControls />
        </header>
        <main>
          <LoadingSkeleton variant={variant} />
        </main>
      </div>
    </PreferencesProvider>
  );
}

const root = document.getElementById('root');
if (!root) throw new Error('Missing preview root');
createRoot(root).render(<Preview />);
