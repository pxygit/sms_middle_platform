const STORAGE_METADATA_KEY = '__smsStorageUpdatedAt';
export const LOCAL_STORAGE_TTL_MS = 7 * 24 * 60 * 60 * 1000;

type StorageMetadata = Record<string, number>;

function readMetadata(): StorageMetadata {
  try {
    const raw = localStorage.getItem(STORAGE_METADATA_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === 'object' ? parsed as StorageMetadata : {};
  } catch {
    return {};
  }
}

function writeMetadata(metadata: StorageMetadata) {
  try {
    if (Object.keys(metadata).length === 0) {
      localStorage.removeItem(STORAGE_METADATA_KEY);
      return;
    }
    localStorage.setItem(STORAGE_METADATA_KEY, JSON.stringify(metadata));
  } catch {
    // Storage can be unavailable in private or restricted browser contexts.
  }
}

export function setLocalStorageItem(key: string, value: string) {
  const now = Date.now();
  try {
    const metadata = readMetadata();
    const current = localStorage.getItem(key);
    const updatedAt = metadata[key];
    if (current === value && typeof updatedAt === 'number' && now - updatedAt < LOCAL_STORAGE_TTL_MS) {
      return;
    }

    localStorage.setItem(key, value);
    metadata[key] = now;
    writeMetadata(metadata);
  } catch {
    // Keep writes best-effort when browser storage is unavailable.
  }
}

export function getLocalStorageItem(key: string): string | null {
  try {
    const value = localStorage.getItem(key);
    const metadata = readMetadata();
    if (value === null) {
      if (key in metadata) {
        delete metadata[key];
        writeMetadata(metadata);
      }
      return null;
    }

    const updatedAt = metadata[key];
    if (typeof updatedAt !== 'number') {
      metadata[key] = Date.now();
      writeMetadata(metadata);
      return value;
    }
    if (Date.now() - updatedAt >= LOCAL_STORAGE_TTL_MS) {
      localStorage.removeItem(key);
      delete metadata[key];
      writeMetadata(metadata);
      return null;
    }
    return value;
  } catch {
    return null;
  }
}

export function removeLocalStorageItem(key: string) {
  try {
    localStorage.removeItem(key);
    const metadata = readMetadata();
    if (key in metadata) {
      delete metadata[key];
      writeMetadata(metadata);
    }
  } catch {
    // Keep cleanup best-effort when browser storage is unavailable.
  }
}

export function purgeExpiredLocalStorageItems() {
  const now = Date.now();
  try {
    const metadata = readMetadata();
    let changed = false;
    Object.entries(metadata).forEach(([key, updatedAt]) => {
      if (localStorage.getItem(key) === null || typeof updatedAt !== 'number' || now - updatedAt >= LOCAL_STORAGE_TTL_MS) {
        localStorage.removeItem(key);
        delete metadata[key];
        changed = true;
      }
    });
    if (changed) writeMetadata(metadata);
  } catch {
    // Cleanup is retried on the next application load or item read.
  }
}
