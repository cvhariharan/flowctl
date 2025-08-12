import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const load: PageLoad = async ({ params, parent }) => {
  const { user, namespaceId } = await parent();

  // Check permissions
  try {
    const permissions = await permissionChecker(user!, 'flow', namespaceId, ['view']);
    if (!permissions.canRead) {
      error(403, {
        message: 'You do not have permission to view flows in this namespace',
        code: 'INSUFFICIENT_PERMISSIONS'
      });
    }
  } catch (err) {
    if (err && typeof err === 'object' && 'status' in err) {
      throw err; // Re-throw SvelteKit errors
    }
    error(500, {
      message: 'Failed to check permissions',
      code: 'PERMISSION_CHECK_FAILED'
    });
  }

  try {
    const [flowInputs, flowMeta] = await Promise.all([
      apiClient.flows.getInputs(params.namespace, params.flowId),
      apiClient.flows.getMeta(params.namespace, params.flowId)
    ]);
    
    return {
      flowInputs: flowInputs.inputs,
      flowMeta,
    };
  } catch (err) {
    error(500, 'Failed to load flow data');
  }
};