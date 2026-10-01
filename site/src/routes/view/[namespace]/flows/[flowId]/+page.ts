import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient, ApiError } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const load: PageLoad = async ({ params, parent, url }) => {
  const { user, namespaceId } = await parent();

  try {
    const permissions = await permissionChecker(user!, 'flow', namespaceId, ['view'], '_');
    if (!permissions.canRead) {
      error(403, {
        message: 'You do not have permission to view flows in this namespace',
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

  const rerunFromExecId = url.searchParams.get('rerun_from');

  try {
    const [flowInputs, flowMeta, executionData, schedules, allSchedules] = await Promise.all([
      apiClient.flows.getInputs(params.namespace, params.flowId),
      apiClient.flows.getMeta(params.namespace, params.flowId),
      rerunFromExecId
        ? apiClient.executions.getById(params.namespace, rerunFromExecId).catch(() => null)
        : Promise.resolve(null),
      apiClient.flows.schedules.list(params.namespace, params.flowId),
      apiClient.flows.schedules.list(params.namespace, params.flowId, { count_per_page: 100 }),
    ]);

    return {
      flowInputs: flowInputs.inputs,
      optionsRequestId: flowInputs.options_request_id,
      flowMeta,
      namespaceId,
      rerunFromExecId,
      executionInput: executionData?.input || null,
      userSchedules: schedules.schedules || [],
      userSchedulesPageCount: schedules.page_count || 1,
      userSchedulesTotalCount: schedules.total_count || 0,
      allUserSchedules: allSchedules.schedules || [],
    };
  } catch (err) {
    if (err instanceof ApiError) {
      error(err.status, {
        message: err.data?.error || 'An error occurred',
        code: err.data?.code
      });
    }
    error(500, { message: 'Failed to load flow data', code: 'INTERNAL_ERROR' });
  }
};