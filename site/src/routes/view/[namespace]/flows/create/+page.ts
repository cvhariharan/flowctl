import { apiClient } from '$lib/apiClient.js';
import { resolveSchema } from '$lib/utils/flowBuilder';
import { permissionChecker } from '$lib/utils/permissions';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent, url }) => {
  const { user, namespaceId, namespace } = await parent();
  
  // Check create permissions
  try {
    const permissions = await permissionChecker(user!, 'flow', namespaceId, ['create'], '_');
    if (!permissions.canCreate) {
      error(403, {
        message: 'You do not have permission to create flows in this namespace',
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
  
  const duplicateFrom = url.searchParams.get('duplicate_from');

  try {
    const [executorData, messengerSchemas, duplicateConfig] = await Promise.all([
      apiClient.executors.list(),
      apiClient.messengers.list(),
      duplicateFrom ? apiClient.flows.getConfig(namespace, duplicateFrom).catch(() => null) : Promise.resolve(null),
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
      prefillFlow: duplicateConfig,
    };
  } catch (loadError) {
    console.error('Error loading executors/messengers:', loadError);
    return {
      availableExecutors: [],
      availableMessengers: [],
      messengerConfigs: {},
    };
  }
};