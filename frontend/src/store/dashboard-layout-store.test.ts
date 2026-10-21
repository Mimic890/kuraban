import { describe, expect, it } from 'vitest';
import { mergeOrder, resolveFree } from './dashboard-layout-store';

describe('mergeOrder', () => {
  it('keeps defaults when nothing is saved', () => {
    expect(mergeOrder(undefined, ['a', 'b', 'c'])).toEqual(['a', 'b', 'c']);
  });
  it('applies the saved order and drops unknown ids', () => {
    expect(mergeOrder(['c', 'x', 'a', 'b'], ['a', 'b', 'c'])).toEqual(['c', 'a', 'b']);
  });
  it('slots new panels after their default predecessor', () => {
    expect(mergeOrder(['c', 'a'], ['a', 'b', 'c', 'd'])).toEqual(['c', 'd', 'a', 'b']);
    expect(mergeOrder(['b'], ['a', 'b'])).toEqual(['a', 'b']);
  });
});

describe('resolveFree', () => {
  const groups = [{ id: 'g1', title: 'G' }];
  it('starts as the grouped order in the top section', () => {
    expect(resolveFree(null, groups, ['a', 'b', 'c'])).toEqual({ _: ['a', 'b', 'c'], g1: [] });
  });
  it('keeps saved sections, drops unknown and duplicate ids', () => {
    expect(resolveFree({ _: ['c', 'x'], g1: ['b', 'c'] }, groups, ['a', 'b', 'c'])).toEqual({ _: ['a', 'c'], g1: ['b'] });
  });
  it('ignores sections of removed groups', () => {
    expect(resolveFree({ _: ['b'], gone: ['a'] }, [], ['a', 'b'])).toEqual({ _: ['a', 'b'] });
  });
});
