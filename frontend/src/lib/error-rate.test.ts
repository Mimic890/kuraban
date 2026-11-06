import { describe, expect, it } from 'vitest';
import { errorRateTone } from './error-rate';

const tone = (requests: number, errors: number, serverErrors: number) => errorRateTone({ requests, errors, serverErrors }, 5);

describe('errorRateTone', () => {
  it('is neutral without traffic and green without errors', () => {
    expect(tone(0, 0, 0)).toBe('neutral');
    expect(tone(1000, 0, 0)).toBe('ok');
  });

  it('turns red only on 5xx above the threshold', () => {
    expect(tone(1000, 50, 50)).toBe('crit');
    expect(tone(1000, 900, 0)).toBe('warn');
  });

  it('stays green for a few 4xx and yellow for any 5xx', () => {
    expect(tone(1000, 49, 0)).toBe('ok');
    expect(tone(1000, 1, 1)).toBe('warn');
  });

  it('never turns red at low traffic', () => {
    expect(tone(1, 1, 1)).toBe('warn');
    expect(tone(19, 19, 19)).toBe('warn');
    expect(tone(20, 1, 1)).toBe('crit');
    expect(tone(3, 3, 0)).toBe('ok');
  });
});
