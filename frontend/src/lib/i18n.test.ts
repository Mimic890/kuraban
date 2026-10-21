import { describe, expect, it } from 'vitest';
import en from './locales/en.json';
import ru from './locales/ru.json';

const placeholders = (value: string) => [...value.matchAll(/\{\{(\w+)\}\}/g)].map((match) => match[1]).sort();
const dynamicKeys = [
  ...['healthy', 'degraded', 'unhealthy', 'unknown'].map((status) => `dashboard.health_${status}`),
  'dashboard.storage_across_one_bucket',
  'dashboard.storage_across_buckets',
  'dashboard.one_object_count',
  'dashboard.objects_count',
  ...['active', 'inactive'].map((status) => `access_control.status_${status}`),
  'access_control.one_bucket_count',
  'access_control.buckets_count',
];

describe('locales', () => {
  it('have matching, non-empty keys and placeholders', () => {
    expect(Object.keys(ru).sort()).toEqual(Object.keys(en).sort());
    for (const key of Object.keys(en)) {
      expect(en[key as keyof typeof en].trim(), `${key} is blank in English`).not.toBe('');
      expect(ru[key as keyof typeof ru].trim(), `${key} is blank in Russian`).not.toBe('');
      expect(placeholders(ru[key as keyof typeof ru]), `${key} has different placeholders`).toEqual(placeholders(en[key as keyof typeof en]));
    }
  });

  it.each([['en', en], ['ru', ru]] as const)('contains dynamic Dashboard and Access Control keys in %s', (_, locale) => {
    for (const key of dynamicKeys) expect(locale).toHaveProperty(key);
  });
});
