import type { Tone } from '@/components/dashboard/panels';

// Below this a percentage misleads: one failure out of one request reads as 100%.
export const MIN_REQUESTS_FOR_PCT = 20;

export interface ErrorCounts {
  requests: number;
  errors: number;
  serverErrors: number;
}

export function errorRateTone({ requests, errors, serverErrors }: ErrorCounts, warnPct: number): Tone {
  if (requests <= 0) return 'neutral';
  const enough = requests >= MIN_REQUESTS_FOR_PCT;
  if (enough && (serverErrors / requests) * 100 >= warnPct) return 'crit';
  // A normal 404 on HeadObject is not a problem, so a few 4xx stay green.
  if (serverErrors > 0 || (enough && (errors / requests) * 100 >= warnPct)) return 'warn';
  return 'ok';
}
