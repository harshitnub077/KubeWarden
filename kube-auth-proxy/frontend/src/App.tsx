import { useEffect, useState } from 'react';

interface Permission {
  resource: string;
  verb: string;
  namespace: string;
  allowed: boolean;
}

interface PermissionMatrix {
  user: string;
  permissions: Permission[];
}

const VERBS = ['get', 'list', 'create', 'delete'];

// Group permissions by resource for the table view
function groupByResource(perms: Permission[]): Record<string, Record<string, boolean>> {
  const map: Record<string, Record<string, boolean>> = {};
  for (const p of perms) {
    if (!map[p.resource]) map[p.resource] = {};
    map[p.resource][p.verb] = p.allowed;
  }
  return map;
}

export default function App() {
  const [matrix, setMatrix] = useState<PermissionMatrix | null>(null);
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const fetchPermissions = async () => {
    setLoading(true); setError('');
    try {
      const res = await fetch('http://localhost:8888/api/permissions', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(await res.text());
      setMatrix(await res.json());
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const grouped = matrix ? groupByResource(matrix.permissions) : {};

  return (
    <div style={{ fontFamily: 'system-ui', maxWidth: 900, margin: '40px auto', padding: '0 20px' }}>
      <h1 style={{ fontWeight: 700 }}>kube-auth-proxy</h1>
      <p style={{ color: '#555' }}>OIDC/RBAC Permission Matrix — Kubernetes API</p>

      <div style={{ display: 'flex', gap: 12, marginBottom: 24 }}>
        <input
          style={{ flex: 1, padding: '8px 12px', border: '1px solid #ccc', borderRadius: 4 }}
          placeholder="Paste your OIDC Bearer token here"
          value={token}
          onChange={e => setToken(e.target.value)}
        />
        <button
          onClick={fetchPermissions}
          disabled={loading || !token}
          style={{ padding: '8px 20px', background: '#000', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
        >
          {loading ? 'Evaluating…' : 'Evaluate RBAC'}
        </button>
      </div>

      {error && <p style={{ color: 'red' }}>{error}</p>}

      {matrix && (
        <>
          <p>User: <strong>{matrix.user}</strong></p>
          <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 16 }}>
            <thead>
              <tr style={{ background: '#f5f5f5' }}>
                <th style={th}>Resource</th>
                {VERBS.map(v => <th key={v} style={th}>{v}</th>)}
              </tr>
            </thead>
            <tbody>
              {Object.entries(grouped).map(([resource, perms]) => (
                <tr key={resource}>
                  <td style={{ ...td, fontFamily: 'monospace' }}>{resource}</td>
                  {VERBS.map(v => (
                    <td key={v} style={{ ...td, textAlign: 'center' }}>
                      {perms[v] ? '✅' : '❌'}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
}

const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', border: '1px solid #e0e0e0' };
const td: React.CSSProperties = { padding: '8px 14px', border: '1px solid #e0e0e0' };
