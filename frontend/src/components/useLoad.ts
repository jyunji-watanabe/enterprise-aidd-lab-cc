import { useCallback, useEffect, useState } from 'react';

export interface Loaded<T> {
  data: T | undefined;
  error: unknown;
  loading: boolean;
  reload: () => void;
}

/** Runs an async loader on mount and whenever `key` changes. */
export function useLoad<T>(loader: () => Promise<T>, key: string): Loaded<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    loader()
      .then((d) => {
        if (!cancelled) {
          setData(d);
          setError(null);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `key` captures loader inputs
  }, [key, tick]);

  const reload = useCallback(() => {
    setTick((t) => t + 1);
  }, []);
  return { data, error, loading, reload };
}
