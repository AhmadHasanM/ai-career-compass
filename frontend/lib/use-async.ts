"use client";

import { useCallback, useEffect, useState } from "react";

export type AsyncState<T> = {
  data: T | undefined;
  error: unknown;
  loading: boolean;
  reload: () => void;
};

// Muat data saat mount dan setiap kali deps (nilai primitif) berubah. Selama memuat ulang, data lama
// tetap tersedia agar layar tidak berkedip; hasil request lama yang telat datang diabaikan.
export function useAsync<T>(fn: () => Promise<T>, deps: (string | number | boolean | null | undefined)[] = []): AsyncState<T> {
  const [tick, setTick] = useState(0);
  const key = JSON.stringify([...deps, tick]);
  const [state, setState] = useState<{ key: string | null; data?: T; error?: unknown }>({ key: null });

  useEffect(() => {
    let alive = true;
    fn().then(
      (data) => alive && setState({ key, data }),
      (error) => alive && setState((s) => ({ key, data: s.data, error })),
    );
    return () => {
      alive = false;
    };
    // fn sengaja tidak menjadi dependency: permintaan hanya diulang saat deps atau reload berubah.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const loading = state.key !== key;
  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data: state.data, error: loading ? undefined : state.error, loading, reload };
}
