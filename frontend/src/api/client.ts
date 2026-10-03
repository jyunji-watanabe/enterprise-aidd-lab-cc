import type { FieldError } from './types';

/** Error returned by the API, carrying the HTTP status and field errors (422). */
export class ApiError extends Error {
  readonly status: number;
  readonly fields: FieldError[];

  constructor(status: number, message: string, fields: FieldError[] = []) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.fields = fields;
  }
}

interface ErrorBody {
  error?: string;
  fields?: FieldError[];
}

async function parseError(res: Response): Promise<ApiError> {
  let body: ErrorBody = {};
  try {
    body = (await res.json()) as ErrorBody;
  } catch {
    // non-JSON error body
  }
  return new ApiError(res.status, body.error ?? `HTTP ${res.status}`, body.fields ?? []);
}

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return '予期しないエラーが発生しました';
}

/** Listeners notified on 401 so the app can return to the login screen. */
const unauthorizedListeners = new Set<() => void>();
export function onUnauthorized(fn: () => void): () => void {
  unauthorizedListeners.add(fn);
  return () => unauthorizedListeners.delete(fn);
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin', headers: {} };
  if (body instanceof FormData) {
    init.body = body;
  } else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers = { 'Content-Type': 'application/json' };
  }
  const res = await fetch(path, init);
  if (!res.ok) {
    const err = await parseError(res);
    if (res.status === 401 && path !== '/api/auth/login') {
      unauthorizedListeners.forEach((fn) => {
        fn();
      });
    }
    throw err;
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export const get = <T>(path: string) => request<T>('GET', path);
export const post = <T>(path: string, body?: unknown) => request<T>('POST', path, body);
export const put = <T>(path: string, body?: unknown) => request<T>('PUT', path, body);
export const del = (path: string) => request<undefined>('DELETE', path);
