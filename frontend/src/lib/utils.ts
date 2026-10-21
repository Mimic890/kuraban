import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useSettingsStore } from '@/store/settings-store';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date;
  const { timezone: timeZone, language } = useSettingsStore.getState();
  return new Intl.DateTimeFormat(language, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone,
    timeZoneName: 'short',
  }).format(d);
}

export function dateTimeInputValue(date: Date, timeZone = useSettingsStore.getState().timezone): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  }).formatToParts(date).reduce<Record<string, string>>((out, part) => {
    out[part.type] = part.value;
    return out;
  }, {});
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

export function dateTimeInputToIso(value: string, timeZone = useSettingsStore.getState().timezone): string {
  const wallClock = new Date(`${value}:00Z`);
  const shown = new Date(new Intl.DateTimeFormat('en-US', {
    timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  }).format(wallClock));
  return new Date(wallClock.getTime() + (wallClock.getTime() - shown.getTime())).toISOString();
}
