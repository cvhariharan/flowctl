import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { FLOWS_PER_PAGE } from '$lib/constants';
import { permissionChecker } from '$lib/utils/permissions';

export const load: PageLoad = async ({ params, url, parent }) => {
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
      throw err; // Re-throw SvelteKit errors (like the 403 above)
    }
    error(500, {
      message: 'Failed to check permissions',
      code: 'PERMISSION_CHECK_FAILED'
    });
  }

  try {
    const page = Number(url.searchParams.get('page')) || 1;
    const filter = url.searchParams.get('filter') || '';
    
    const data = await apiClient.flows.list(params.namespace, {
      page,
      count_per_page: FLOWS_PER_PAGE,
      filter
    });
    
    return {
      flows: data.flows,
      pageCount: data.page_count,
      totalCount: data.total_count,
      currentPage: page,
      filter,
      namespaceId
    };
  } catch (err) {
    console.log("failed to load flows: ", err)
    error(500, 'Failed to load flows data');
  }
};