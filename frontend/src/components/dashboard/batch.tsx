import * as React from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useClusterStore } from '@/store/cluster-store';
import { resolveRange, telemetryKeys, type TimeRange } from '@/hooks/useTelemetry';
import { telemetryApi, type MetricQuery, type RangeResult } from '@/lib/telemetry';

// Every panel on the dashboard registers its queries here, and the dashboard
// fetches them all with one /telemetry/query request per refresh instead of
// one per panel. Panels that are not mounted (collapsed rows, hidden panels)
// or that pass enabled=false are simply not part of the request.

const SEP = '~';

interface BatchResult {
  result: RangeResult;
  /** Panel id → the short prefix its query ids carried in this request. */
  panels: Map<string, string>;
}

interface BatchCtx {
  register: (panel: string, queries: MetricQuery[]) => () => void;
  data: BatchResult | undefined;
  isFetching: boolean;
  isError: boolean;
}

const BatchContext = React.createContext<BatchCtx | null>(null);

export function QueryBatchProvider({ range, refetchInterval, children }: { range: TimeRange; refetchInterval: number | false; children: React.ReactNode }) {
  const clusterId = useClusterStore((s) => s.activeClusterId);
  const [registry, setRegistry] = React.useState<Record<string, MetricQuery[]>>({});

  const register = React.useCallback((panel: string, queries: MetricQuery[]) => {
    setRegistry((r) => (r[panel] === queries ? r : { ...r, [panel]: queries }));
    return () =>
      setRegistry((r) => {
        if (!(panel in r)) return r;
        const next = { ...r };
        delete next[panel];
        return next;
      });
  }, []);

  // Short numeric prefixes keep the request (sent as a GET query string) small.
  const { queries, panels } = React.useMemo(() => {
    const ids = Object.keys(registry).sort();
    const panels = new Map(ids.map((p, i) => [p, i.toString(36)]));
    return {
      panels,
      queries: ids.flatMap((p) => registry[p].map((q) => ({ ...q, id: `${panels.get(p)}${SEP}${q.id}` }))),
    };
  }, [registry]);

  const q = useQuery({
    queryKey: [...telemetryKeys.range(clusterId, queries, range), 'batch'],
    queryFn: async ({ signal }): Promise<BatchResult> => ({
      result: await telemetryApi.query({ ...resolveRange(range), queries }, signal),
      panels,
    }),
    enabled: !!clusterId && queries.length > 0,
    refetchInterval: range.kind === 'relative' ? refetchInterval : false,
    refetchIntervalInBackground: false,
    staleTime: 2000,
    placeholderData: keepPreviousData,
    retry: false,
  });

  const value = React.useMemo<BatchCtx>(
    () => ({ register, data: q.data, isFetching: q.isFetching, isError: q.isError }),
    [register, q.data, q.isFetching, q.isError],
  );
  return <BatchContext.Provider value={value}>{children}</BatchContext.Provider>;
}

export interface PanelData {
  data: RangeResult | undefined;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
}

/** A panel's slice of the dashboard batch. Returns null outside a QueryBatchProvider. */
export function useBatchedRange(queries: MetricQuery[], enabled = true): PanelData | null {
  const batch = React.useContext(BatchContext);
  const panel = React.useId();
  const register = batch?.register;

  React.useEffect(() => {
    if (!register || !enabled || queries.length === 0) return;
    return register(panel, queries);
  }, [register, panel, queries, enabled]);

  const data = batch?.data;
  const slice = React.useMemo(() => {
    const short = data?.panels.get(panel);
    if (!data || short === undefined) return undefined;
    const prefix = short + SEP;
    return {
      ...data.result,
      series: (data.result.series ?? []).filter((s) => s.query.startsWith(prefix)).map((s) => ({ ...s, query: s.query.slice(prefix.length) })),
    };
  }, [data, panel]);

  if (!batch) return null;
  return {
    data: slice,
    isLoading: enabled && queries.length > 0 && !slice,
    isFetching: enabled && batch.isFetching,
    isError: enabled && batch.isError,
  };
}
