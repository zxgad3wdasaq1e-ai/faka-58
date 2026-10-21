type StorageAccess = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export const readRenamedStorageValue = (storage: StorageAccess, key: string, legacyKey: string): string | null => {
    const current = storage.getItem(key)
    const legacy = storage.getItem(legacyKey)
    const value = current ?? legacy
    if (value !== null && legacy !== null) {
        try {
            if (current === null) storage.setItem(key, value)
            storage.removeItem(legacyKey)
        } catch {
            // 迁移失败时继续使用旧值，只有新键写入成功后才移除旧键。
        }
    }
    return value
}
