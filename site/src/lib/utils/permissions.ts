import { handleInlineError } from './errorHandling';
import { apiClient } from '$lib/apiClient';

export interface ResourcePermissions {
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  canRead: boolean;
  canViewConfig: boolean;
  canExport: boolean;
}

export interface User {
  id: string;
  groups?: string[];
}

export type PermissionAction = 'create' | 'view' | 'update' | 'delete' | 'view_config' | 'export';

const CACHE_PREFIX = 'permissions:';
const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

interface CacheEntry {
  permissions: ResourcePermissions;
  expiry: number;
}

function buildCacheKey(resourceType: string, domain: string, actions: PermissionAction[]): string {
  return `${CACHE_PREFIX}${resourceType}:${domain}:${[...actions].sort().join(',')}`;
}

function getCached(key: string): ResourcePermissions | null {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    const entry: CacheEntry = JSON.parse(raw);
    if (Date.now() > entry.expiry) {
      localStorage.removeItem(key);
      return null;
    }
    return entry.permissions;
  } catch {
    return null;
  }
}

function setCache(key: string, permissions: ResourcePermissions): void {
  try {
    const entry: CacheEntry = { permissions, expiry: Date.now() + CACHE_TTL_MS };
    localStorage.setItem(key, JSON.stringify(entry));
  } catch {
    // localStorage full or unavailable — silently ignore
  }
}

/** Clears all cached permission entries from localStorage. */
export function clearPermissionCache(): void {
  try {
    const keysToRemove: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith(CACHE_PREFIX)) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach((key) => localStorage.removeItem(key));
  } catch {
    // localStorage unavailable — silently ignore
  }
}

/**
 * Builds a Casbin domain string for permission checks.
 * Namespace-level: /<namespaceId>/*
 * Prefix-level:    /<namespaceId>/<prefix>
 */
function buildDomain(namespaceId: string, prefix?: string): string {
  if (prefix) {
    return `/${namespaceId}/${prefix}`;
  }
  return `/${namespaceId}/*`;
}

/**
 * Checks permissions for a resource type in a given namespace.
 * Calls the server-side permission check API which uses the properly
 * configured Casbin enforcer with domain matching.
 * Results are cached in localStorage for 5 minutes.
 */
export async function permissionChecker(
  _user: User,
  resourceType: string,
  namespaceId: string,
  actions: PermissionAction[] = ['create', 'view', 'update', 'delete'],
  prefix?: string
): Promise<ResourcePermissions> {
  const domain = buildDomain(namespaceId, prefix);
  const cacheKey = buildCacheKey(resourceType, domain, actions);

  const cached = getCached(cacheKey);
  if (cached) return cached;

  const permissions: ResourcePermissions = {
    canCreate: false,
    canRead: false,
    canUpdate: false,
    canDelete: false,
    canViewConfig: false,
    canExport: false,
  };

  try {
    const data: { results: Record<string, boolean> } = await apiClient.permissions.check(
      actions.map(action => ({ resource: resourceType, action, domain }))
    );

    for (const action of actions) {
      const key = `${domain}:${resourceType}:${action}`;
      const allowed = data.results[key] || false;
      switch (action) {
        case 'create':
          permissions.canCreate = allowed;
          break;
        case 'view':
          permissions.canRead = allowed;
          break;
        case 'update':
          permissions.canUpdate = allowed;
          break;
        case 'delete':
          permissions.canDelete = allowed;
          break;
        case 'view_config':
          permissions.canViewConfig = allowed;
          break;
        case 'export':
          permissions.canExport = allowed;
          break;
      }
    }

    setCache(cacheKey, permissions);
  } catch (err) {
    handleInlineError(err, 'Unable to Check Permissions');
  }

  return permissions;
}
