import { useState, useEffect, useCallback, useRef } from "react";

/**
 * State for async data operations
 */
export interface AsyncDataState<T> {
  /** The fetched data, null if not yet loaded or on error */
  data: T | null;
  /** True while the initial fetch or a refetch is in progress */
  loading: boolean;
  /** Error object if the fetch failed, null otherwise */
  error: Error | null;
}

/**
 * Return type for useAsyncData hook
 */
export interface UseAsyncDataResult<T> extends AsyncDataState<T> {
  /** Manually trigger a refetch of the data */
  refetch: () => Promise<void>;
  /** Update the data state directly (useful for optimistic updates) */
  setData: (data: T | null | ((prev: T | null) => T | null)) => void;
  /** Clear the error state */
  clearError: () => void;
  /** True if data has been successfully loaded at least once */
  isLoaded: boolean;
}

/**
 * Options for useAsyncData hook
 */
export interface UseAsyncDataOptions {
  /** If false, the fetch will not run automatically on mount. Default: true */
  enabled?: boolean;
  /** Callback when fetch succeeds */
  onSuccess?: () => void;
  /** Callback when fetch fails */
  onError?: (error: Error) => void;
  /** Keep previous data while refetching. Default: false */
  keepPreviousData?: boolean;
}

/**
 * Generic hook for fetching async data with loading, error, and refetch support.
 *
 * @example Basic usage
 * ```tsx
 * const { data, loading, error, refetch } = useAsyncData(
 *   () => api.getUsers(),
 *   []
 * );
 *
 * if (loading) return <Spinner />;
 * if (error) return <ErrorMessage error={error} />;
 * return <UserList users={data} />;
 * ```
 *
 * @example With dependencies
 * ```tsx
 * const { data } = useAsyncData(
 *   () => api.getUser(userId),
 *   [userId]
 * );
 * ```
 *
 * @example Disabled until ready
 * ```tsx
 * const { data } = useAsyncData(
 *   () => api.getDetails(id),
 *   [id],
 *   { enabled: !!id }
 * );
 * ```
 *
 * @param fetcher - Async function that returns the data
 * @param deps - Dependency array that triggers refetch when changed
 * @param options - Optional configuration
 */
export function useAsyncData<T>(
  fetcher: () => Promise<T>,
  deps: React.DependencyList = [],
  options: UseAsyncDataOptions = {}
): UseAsyncDataResult<T> {
  const {
    enabled = true,
    onSuccess,
    onError,
    keepPreviousData = false,
  } = options;

  const [state, setState] = useState<AsyncDataState<T>>({
    data: null,
    loading: enabled,
    error: null,
  });
  const [isLoaded, setIsLoaded] = useState(false);

  // Track if component is mounted to avoid state updates after unmount
  const isMountedRef = useRef(true);
  // Track the current fetch to handle race conditions
  const fetchIdRef = useRef(0);

  const fetchData = useCallback(async () => {
    if (!enabled) return;

    const fetchId = ++fetchIdRef.current;

    setState((prev) => ({
      ...prev,
      loading: true,
      error: null,
      // Optionally keep previous data while loading
      data: keepPreviousData ? prev.data : null,
    }));

    try {
      const result = await fetcher();

      // Only update state if this is still the latest fetch and component is mounted
      if (isMountedRef.current && fetchId === fetchIdRef.current) {
        setState({
          data: result,
          loading: false,
          error: null,
        });
        setIsLoaded(true);
        onSuccess?.();
      }
    } catch (err) {
      // Only update state if this is still the latest fetch and component is mounted
      if (isMountedRef.current && fetchId === fetchIdRef.current) {
        const error = err instanceof Error ? err : new Error(String(err));
        setState((prev) => ({
          data: keepPreviousData ? prev.data : null,
          loading: false,
          error,
        }));
        onError?.(error);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, keepPreviousData, ...deps]);

  // Initial fetch and refetch on dependency change
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Cleanup on unmount
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  const setData = useCallback(
    (dataOrUpdater: T | null | ((prev: T | null) => T | null)) => {
      setState((prev) => ({
        ...prev,
        data:
          typeof dataOrUpdater === "function"
            ? (dataOrUpdater as (prev: T | null) => T | null)(prev.data)
            : dataOrUpdater,
      }));
    },
    []
  );

  const clearError = useCallback(() => {
    setState((prev) => ({ ...prev, error: null }));
  }, []);

  return {
    ...state,
    refetch: fetchData,
    setData,
    clearError,
    isLoaded,
  };
}

/**
 * Hook for fetching multiple async data sources in parallel.
 *
 * @example
 * ```tsx
 * const { data, loading, error } = useAsyncDataAll({
 *   users: () => api.getUsers(),
 *   posts: () => api.getPosts(),
 * }, []);
 *
 * if (loading) return <Spinner />;
 * // data.users and data.posts are now available
 * ```
 */
export function useAsyncDataAll<T extends Record<string, () => Promise<unknown>>>(
  fetchers: T,
  deps: React.DependencyList = [],
  options: UseAsyncDataOptions = {}
): UseAsyncDataResult<{ [K in keyof T]: Awaited<ReturnType<T[K]>> }> {
  type ResultType = { [K in keyof T]: Awaited<ReturnType<T[K]>> };

  const combinedFetcher = useCallback(async () => {
    const keys = Object.keys(fetchers) as (keyof T)[];
    const promises = keys.map((key) => fetchers[key]());
    const results = await Promise.all(promises);

    return keys.reduce((acc, key, index) => {
      acc[key] = results[index] as Awaited<ReturnType<T[typeof key]>>;
      return acc;
    }, {} as ResultType);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return useAsyncData<ResultType>(combinedFetcher, deps, options);
}
