import { useSettingsStore } from '@/store/settings-store';

export type Unit = 'bytes' | 'bytesPerSec' | 'reqps' | 'opsps' | 'ms' | 'seconds' | 'percent' | 'short' | 'count';

const BYTE_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB'];

function locale() {
  return useSettingsStore.getState().language;
}

function num(value: number, digits: number) {
  return new Intl.NumberFormat(locale(), { maximumFractionDigits: digits, minimumFractionDigits: 0 }).format(value);
}

function adaptiveDigits(v: number) {
  const a = Math.abs(v);
  if (a === 0) return 0;
  if (a >= 100) return 0;
  if (a >= 10) return 1;
  return 2;
}

export function formatBytesValue(bytes: number): string {
  if (!Number.isFinite(bytes)) return '—';
  let v = Math.abs(bytes);
  let i = 0;
  while (v >= 1024 && i < BYTE_UNITS.length - 1) {
    v /= 1024;
    i++;
  }
  const signed = bytes < 0 ? -v : v;
  return `${num(signed, i === 0 ? 0 : adaptiveDigits(v))} ${BYTE_UNITS[i]}`;
}

export function formatShort(value: number): string {
  if (!Number.isFinite(value)) return '—';
  const a = Math.abs(value);
  const units: [number, string][] = [[1e12, 'T'], [1e9, 'B'], [1e6, 'M'], [1e3, 'K']];
  for (const [div, suffix] of units) {
    if (a >= div) return `${num(value / div, adaptiveDigits(a / div))}${suffix}`;
  }
  return num(value, adaptiveDigits(a));
}

export function formatDurationMs(ms: number): string {
  if (!Number.isFinite(ms)) return '—';
  const a = Math.abs(ms);
  if (a === 0) return '0 ms';
  if (a < 1) return `${num(ms * 1000, adaptiveDigits(a * 1000))} µs`;
  if (a < 1000) return `${num(ms, adaptiveDigits(a))} ms`;
  if (a < 60_000) return `${num(ms / 1000, adaptiveDigits(a / 1000))} s`;
  return `${num(ms / 60_000, 1)} min`;
}

export function formatValue(value: number | null | undefined, unit: Unit): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '—';
  switch (unit) {
    case 'bytes':
      return formatBytesValue(value);
    case 'bytesPerSec':
      return `${formatBytesValue(value)}/s`;
    case 'reqps':
      return `${formatShort(value)} req/s`;
    case 'opsps':
      return `${formatShort(value)} ops/s`;
    case 'ms':
      return formatDurationMs(value);
    case 'seconds':
      return formatDurationMs(value * 1000);
    case 'percent':
      return `${num(value, Math.abs(value) >= 10 ? 1 : 2)}%`;
    case 'count':
      return num(Math.round(value), 0);
    default:
      return formatShort(value);
  }
}

/** Compact axis labels: fewer digits, no spaces for bytes. */
export function formatAxis(value: number, unit: Unit): string {
  if (unit === 'bytes' || unit === 'bytesPerSec') return formatBytesValue(value).replace(' ', '');
  if (unit === 'ms') return formatDurationMs(value).replace(' ', '');
  if (unit === 'seconds') return formatDurationMs(value * 1000).replace(' ', '');
  if (unit === 'percent') return `${num(value, 0)}%`;
  return formatShort(value);
}

export function formatTime(ts: number, withSeconds: boolean, withDate: boolean): string {
  const { timezone, language, hour12 } = useSettingsStore.getState();
  return new Intl.DateTimeFormat(language, {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    ...(withSeconds ? { second: '2-digit' } : {}),
    ...(withDate ? { month: 'short', day: 'numeric' } : {}),
    hour12,
  }).format(new Date(ts * 1000));
}

export function formatDate(ts: number): string {
  const { timezone, language } = useSettingsStore.getState();
  return new Intl.DateTimeFormat(language, { timeZone: timezone, month: 'short', day: 'numeric' }).format(new Date(ts * 1000));
}

export function formatRelative(date: Date | string | number | undefined): string {
  if (date === undefined) return '—';
  const d = typeof date === 'number' ? new Date(date * 1000) : new Date(date);
  const diff = (d.getTime() - Date.now()) / 1000;
  const rtf = new Intl.RelativeTimeFormat(locale(), { numeric: 'auto' });
  const a = Math.abs(diff);
  if (a < 60) return rtf.format(Math.round(diff), 'second');
  if (a < 3600) return rtf.format(Math.round(diff / 60), 'minute');
  if (a < 86400) return rtf.format(Math.round(diff / 3600), 'hour');
  return rtf.format(Math.round(diff / 86400), 'day');
}
