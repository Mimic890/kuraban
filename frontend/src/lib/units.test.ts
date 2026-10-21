import { describe, expect, it } from 'vitest';
import { formatAxis, formatBytesValue, formatDurationMs, formatShort, formatValue } from './units';
import { refreshMs, resolveRange, rangeSeconds } from '@/hooks/useTelemetry';
import { withAlpha } from './chart-theme';

describe('units', () => {
  it('formats bytes with binary prefixes', () => {
    expect(formatBytesValue(0)).toBe('0 B');
    expect(formatBytesValue(1536)).toBe('1.5 KiB');
    expect(formatBytesValue(5 * 1024 ** 3)).toBe('5 GiB');
    expect(formatValue(2048, 'bytesPerSec')).toBe('2 KiB/s');
  });

  it('formats durations across units', () => {
    expect(formatDurationMs(0.25)).toBe('250 µs');
    expect(formatDurationMs(12.34)).toBe('12.3 ms');
    expect(formatDurationMs(2500)).toBe('2.5 s');
  });

  it('shortens large numbers and handles missing values', () => {
    expect(formatShort(1234)).toBe('1.23K');
    expect(formatShort(2_500_000)).toBe('2.5M');
    expect(formatValue(null, 'reqps')).toBe('—');
    expect(formatValue(12.345, 'percent')).toBe('12.3%');
    expect(formatValue(3.6, 'count')).toBe('4');
    expect(formatAxis(1024, 'bytes')).toBe('1KiB');
  });
});

describe('time range helpers', () => {
  it('maps keys to seconds and refresh options to intervals', () => {
    expect(rangeSeconds('6h')).toBe(21600);
    expect(rangeSeconds('nope')).toBe(3600);
    expect(refreshMs('off', 15)).toBe(false);
    expect(refreshMs('auto', 10)).toBe(10_000);
    expect(refreshMs('auto', undefined)).toBe(15_000);
    expect(refreshMs('1m', 15)).toBe(60_000);
  });

  it('resolves relative and absolute ranges', () => {
    expect(resolveRange({ kind: 'absolute', from: 10, to: 20 })).toEqual({ from: 10, to: 20 });
    const r = resolveRange({ kind: 'relative', key: '1h' });
    expect(r.to - r.from).toBe(3600);
  });
});

describe('chart colors', () => {
  it('adds alpha to hex and rgb colors', () => {
    expect(withAlpha('#ff0000', 0.5)).toBe('rgba(255, 0, 0, 0.5)');
    expect(withAlpha('#0f0', 1)).toBe('rgba(0, 255, 0, 1)');
    expect(withAlpha('rgb(1, 2, 3)', 0.2)).toBe('rgba(1, 2, 3, 0.2)');
  });
});
