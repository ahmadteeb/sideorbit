// A small query cache: pages show the last data at once and refresh it.
import { goto } from '$app/navigation';
import { ApiError, asApiError } from './api';

const cache = new Map<string, unknown>();

export function clearCache() {
  cache.clear();
}

export interface Resource<T> {
  readonly data: T | undefined;
  readonly error: ApiError | undefined;
  readonly loading: boolean;
  reload(): Promise<void>;
}

export function resource<T>(key: string, load: () => Promise<T>): Resource<T> {
  let data = $state.raw<T | undefined>(cache.get(key) as T | undefined);
  let error = $state.raw<ApiError | undefined>();
  let loading = $state(true);

  async function reload() {
    loading = true;
    try {
      const next = await load();
      cache.set(key, next);
      data = next;
      error = undefined;
    } catch (e) {
      const failure = asApiError(e);
      if (failure.status === 401) {
        clearCache();
        await goto('/login');
        return;
      }
      error = failure;
    } finally {
      loading = false;
    }
  }
  void reload();

  return {
    get data() {
      return data;
    },
    get error() {
      return error;
    },
    get loading() {
      return loading;
    },
    reload
  };
}
