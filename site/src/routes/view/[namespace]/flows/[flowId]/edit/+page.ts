import { apiClient } from '$lib/apiClient.js';
import { resolveSchema } from '$lib/utils/flowBuilder';
import { permissionChecker } from '$lib/utils/permissions';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
  const { user, namespaceId } = await parent();

  // Check update and view_config permissions
  let readonly = false;
  try {
    const permissions = await permissionChecker(user!, 'flow', namespaceId, ['update', 'view_config'], '_');
    if (!permissions.canUpdate && !permissions.canViewConfig) {
      error(403, {
        message: 'You do not have permission to view the flow configuration in this namespace',
        code: 'INSUFFICIENT_PERMISSIONS'
      });
    }
    readonly = !permissions.canUpdate;
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
    const [executorData, messengerSchemas] = await Promise.all([
      apiClient.executors.list(),
      apiClient.messengers.list(),
    ]);

    const availableExecutors = executorData.executors.map(info => ({
      name: info.name,
      capabilities: info.capabilities,
    }));

    const messengerConfigs: Record<string, any> = {};
    for (const [name, schema] of Object.entries(messengerSchemas)) {
      messengerConfigs[name] = resolveSchema(schema);
    }

    return {
      availableExecutors,
      availableMessengers: Object.keys(messengerSchemas),
      messengerConfigs,
      readonly,
    };
  } catch (loadError) {
    console.error('Error loading executors/messengers:', loadError);
    return {
      availableExecutors: [],
      availableMessengers: [],
      messengerConfigs: {},
      readonly,
    };
  }
};