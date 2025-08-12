import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const load: PageLoad = async ({ params, parent }) => {
  const { user, namespaceId } = await parent();
  const { namespace, flowId, logId } = params;

  // Check permissions
  try {
    const permissions = await permissionChecker(user!, 'execution', namespaceId, ['view']);
    if (!permissions.canRead) {
      error(403, {
        message: 'You do not have permission to view execution results in this namespace',
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
    // Load flow metadata and execution summary in parallel
    const [flowMeta, executionSummary] = await Promise.all([
      apiClient.flows.getMeta(namespace, flowId),
      apiClient.executions.getById(namespace, logId)
    ]);

    return {
      namespace,
      flowId,
      logId,
      flowMeta,
      executionSummary
    };
  } catch (err) {
    error(500, 'Failed to load flow status data');
  }
};