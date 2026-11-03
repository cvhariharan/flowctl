import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient, ApiError } from '$lib/apiClient';
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
    const executionSummary = await apiClient.executions.getById(namespace, logId);
    const flowMeta = await apiClient.flows.getMeta(namespace, flowId, executionSummary.flow_version);

    return {
      namespace,
      flowId,
      logId,
      flowMeta,
      executionSummary
    };
  } catch (err) {
    if (err instanceof ApiError) {
      error(err.status, {
        message: err.data?.error || 'An error occurred',
        code: err.data?.code
      });
    }
    error(500, { message: 'Failed to load flow status data', code: 'INTERNAL_ERROR' });
  }
};