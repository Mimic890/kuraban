// Last known bucket and key lists, shown at once while fresh ones load. Nothing
// here is secret: the list endpoints never return secret keys.
// Bump the version when the shape of a cached list changes.
const PREFIX = 'kuraban-list-cache:v1:';

export function readListCache<T>(key: string): T[] | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(PREFIX + key) ?? 'null');
    return Array.isArray(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

export function writeListCache(key: string, list: unknown[]) {
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify(list));
  } catch {
    // Storage is full or blocked: the page still works, it just loads uncached.
  }
}

export function clearListCache() {
  try {
    Object.keys(localStorage)
      .filter((key) => key.startsWith(PREFIX))
      .forEach((key) => localStorage.removeItem(key));
  } catch {
    // Storage is blocked, so there is nothing to clear.
  }
}
