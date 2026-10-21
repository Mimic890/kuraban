import { describe, expect, it } from 'vitest';
import { computeMenuPosition } from './select';

describe('computeMenuPosition', () => {
  it('opens downward and fills the space left below the trigger', () => {
    const pos = computeMenuPosition({ top: 200, bottom: 240, left: 130, width: 520 }, 900);
    expect(pos).toMatchObject({ top: 244, left: 130, width: 520, maxHeight: 652 });
    expect(pos).not.toHaveProperty('bottom');
  });

  it('flips upward when there is not enough room below', () => {
    const pos = computeMenuPosition({ top: 500, bottom: 540, left: 0, width: 200 }, 600);
    expect(pos).toMatchObject({ bottom: 104, maxHeight: 492 });
    expect(pos).not.toHaveProperty('top');
  });

  it('keeps a scrollable minimum when the viewport is cramped', () => {
    expect(computeMenuPosition({ top: 10, bottom: 50, left: 0, width: 200 }, 100).maxHeight).toBe(120);
  });
});
