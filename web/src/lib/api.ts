// The typed client for docs/API.md. Sessions live in HttpOnly cookies, so no
// token is ever readable here.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fix = ''
  ) {
    super(message);
  }
}

// Required by the server on cookie-authenticated changes (CSRF protection).
const CSRF = { 'X-Orbit-CSRF': '1' };

function send(method: string, path: string, body?: unknown): Promise<Response> {
  const headers: Record<string, string> = method === 'GET' ? {} : { ...CSRF };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  return fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body)
  }).catch(() => {
    throw new ApiError(0, 'offline', "Couldn't reach Orbit.", 'Check your connection, then retry.');
  });
}

let refreshing: Promise<boolean> | null = null;

// A refresh token works once: using it twice signs the session out. So only one
// refresh may run at a time, across tabs too.
async function renew(): Promise<boolean> {
  const run = async () => {
    // Another tab may have renewed the cookies while this one waited.
    if ((await send('GET', '/api/me')).ok) return true;
    return (await send('POST', '/api/auth/token', { grant_type: 'refresh_token' })).ok;
  };
  return navigator.locks ? navigator.locks.request('orbit-refresh', run) : run();
}

function refresh(): Promise<boolean> {
  refreshing ??= renew().finally(() => (refreshing = null));
  return refreshing;
}

export async function api<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  let res = await send(method, path, body);
  if (res.status === 401 && (await refresh())) res = await send(method, path, body);
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    const error = data?.error;
    throw new ApiError(
      res.status,
      error?.code ?? 'error',
      error?.message ?? 'Something went wrong.',
      error?.fix ?? 'Try again.'
    );
  }
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

export function asApiError(e: unknown): ApiError {
  return e instanceof ApiError ? e : new ApiError(0, 'error', 'Something went wrong.', 'Try again.');
}
