import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, onUnauthorized, request } from './client';

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  vi.stubGlobal('fetch', fn);
  return fn;
}

describe('request', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends JSON and returns the parsed body', async () => {
    const fn = mockFetch(200, { ok: true });
    await expect(request('POST', '/api/x', { a: 1 })).resolves.toEqual({ ok: true });
    const [, init] = fn.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe('{"a":1}');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });
  });

  it('turns 422 responses into ApiError with field errors', async () => {
    mockFetch(422, { error: '入力内容に誤りがあります', fields: [{ field: 'comment', message: '必須' }] });
    const err = await request('POST', '/api/x').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(422);
    expect((err as ApiError).fields).toEqual([{ field: 'comment', message: '必須' }]);
  });

  it('notifies listeners on 401', async () => {
    mockFetch(401, { error: '認証が必要です' });
    const listener = vi.fn();
    const off = onUnauthorized(listener);
    await expect(request('GET', '/api/auth/me')).rejects.toThrow('認証が必要です');
    expect(listener).toHaveBeenCalledOnce();
    off();
  });

  it('returns undefined for 204', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    await expect(request('DELETE', '/api/x')).resolves.toBeUndefined();
  });
});
