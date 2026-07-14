import { apiClient } from '$lib/apiClient.js';
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

    // Resolve $defs/$ref in each messenger schema
    const messengerConfigs: Record<string, any> = {};
    for (const [name, schema] of Object.entries(messengerSchemas)) {
      if (schema.$defs && schema.$ref) {
        const refPath = schema.$ref.replace('#/$defs/', '');
        messengerConfigs[name] = schema.$defs[refPath] || schema;
      } else {
        messengerConfigs[name] = schema;
      }
    }

    let prefillFlow = null;
    if (duplicateConfig) {
      prefillFlow = {
        metadata: {
          id: '',
          name: duplicateConfig.metadata.name ? duplicateConfig.metadata.name + ' copy' : '',
          description: duplicateConfig.metadata.description || '',
          prefix: duplicateConfig.metadata.prefix || '',
          schedules: duplicateConfig.metadata.schedules || [],
          namespace,
          allow_overlap: duplicateConfig.metadata.allow_overlap || false,
          user_schedulable: duplicateConfig.metadata.user_schedulable || false,
        },
        inputs: (duplicateConfig.inputs || []).map((input: any) => ({
          ...input,
          optionsText: input.options ? input.options.join('\n') : '',
          maxFileSizeMB: input.max_file_size ? input.max_file_size / 1024 / 1024 : undefined,
          multiple: input.multiple ?? false,
          required: input.required ?? false,
        })),
        actions: (duplicateConfig.actions || []).map((action: any, index: number) => ({
          tempId: Date.now() + index,
          ...action,
          variables: action.variables
            ? action.variables.map((varObj: any) => {
                const [key, value] = Object.entries(varObj)[0];
                return { name: key, value };
              })
            : [],
          artifacts: action.artifacts || [],
          selectedNodes: action.on || [],
          approval: action.approval ?? false,
          allow_node_override: action.allow_node_override ?? false,
          collapsed: false,
        })),
        notifications: (duplicateConfig.notify || []).map((n: any) => ({
          channel: n.channel || '',
          events: n.events || [],
          config: n.config || {},
        })),
      };
    }

    return {
      availableExecutors,
      availableMessengers: Object.keys(messengerSchemas),
      messengerConfigs,
      prefillFlow,
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