import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const ssr = false;

export const load: PageLoad = async ({ params, parent }) => {
  const { user, namespaceId } = await parent();

  let permissions;
  try {
    permissions = await permissionChecker(user!, 'webhook', namespaceId, ['view', 'create', 'update', 'delete']);
    if (!permissions.canRead) {
      error(403, {
        message: 'You do not have permission to view webhooks in this namespace',
        code: 'INSUFFICIENT_PERMISSIONS'
      });
    }
  } catch (err) {
    if (err && typeof err === 'object' && 'status' in err) {
      throw err;
    }
    error(500, {
      message: 'Failed to check permissions',
      code: 'PERMISSION_CHECK_FAILED'
    });
  }

  const { namespace } = params;
  const webhooksPromise = apiClient.namespaceWebhooks.list(namespace);

  return {
    webhooksPromise,
    namespace,
    permissions,
    user,
    namespaceId
  };
};
