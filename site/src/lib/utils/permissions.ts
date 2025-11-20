import { Authorizer } from 'casbin.js';
import { handleInlineError } from './errorHandling';

export interface ResourcePermissions {
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  canRead: boolean;
}

export interface User {
  id: string;
  groups?: string[];
}

export type PermissionAction = 'create' | 'view' | 'update' | 'delete';

/**
 * Checks permissions for a resource type in a given namespace
 * Checks both user permissions and group permissions
 */
export async function permissionChecker(user: User, resourceType: string, namespaceId: string, actions: PermissionAction[] = ['create', 'view', 'update', 'delete']): Promise<ResourcePermissions> {
  const permissions: ResourcePermissions = {
    canCreate: false,
    canRead: false,
    canUpdate: false,
    canDelete: false
  };

  try {
    const authorizer = new Authorizer('auto', {
      endpoint: '/api/v1/permissions'
    });

    // Check user permissions
    await authorizer.setUser(`user:${user.id}`);
    const userResults = await Promise.all(
      actions.map(action => authorizer.can(action, resourceType, namespaceId))
    );

    // Map user results to permissions
    actions.forEach((action, index) => {
      if (userResults[index]) {
        switch (action) {
          case 'create':
            permissions.canCreate = true;
            break;
          case 'view':
            permissions.canRead = true;
            break;
          case 'update':
            permissions.canUpdate = true;
            break;
          case 'delete':
            permissions.canDelete = true;
            break;
        }
      }
    });

    // Check group permissions if user has groups
    if (user.groups && user.groups.length > 0) {
      for (const groupId of user.groups) {
        await authorizer.setUser(`group:${groupId}`);
        const groupResults = await Promise.all(
          actions.map(action => authorizer.can(action, resourceType, namespaceId))
        );

        // Map group results to permissions (OR logic - if any group grants permission, allow it)
        actions.forEach((action, index) => {
          if (groupResults[index]) {
            switch (action) {
              case 'create':
                permissions.canCreate = true;
                break;
              case 'view':
                permissions.canRead = true;
                break;
              case 'update':
                permissions.canUpdate = true;
                break;
              case 'delete':
                permissions.canDelete = true;
                break;
            }
          }
        });
      }
    }
  } catch (err) {
    handleInlineError(err, 'Unable to Check Permissions');
    // permissions remain false on error
  }

  return permissions;
}