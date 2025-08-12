import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';
import { DEFAULT_PAGE_SIZE } from '$lib/constants';


export const load: PageLoad = async ({ params, url, parent }) => {
	const { user, namespaceId } = await parent();

	// Check permissions
	try {
		const permissions = await permissionChecker(user!, 'node', namespaceId, ['view']);
		if (!permissions.canRead) {
			error(403, {
				message: 'You do not have permission to view nodes in this namespace',
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
		const { namespace } = params;
		const page = Number(url.searchParams.get('page') || '1');
		const search = url.searchParams.get('search') || '';

		// Fetch nodes data
		const nodesResponse = await apiClient.nodes.list(namespace, {
			page,
			count_per_page: DEFAULT_PAGE_SIZE,
			filter: search || undefined
		});

		// Fetch credentials for the modal
		const credentialsResponse = await apiClient.credentials.list(namespace);

		return {
			nodes: nodesResponse.nodes || [],
			totalCount: nodesResponse.total_count || 0,
			pageCount: nodesResponse.page_count || 1,
			currentPage: page,
			searchQuery: search,
			credentials: credentialsResponse.credentials || [],
			namespace
		};
	} catch (err) {
		error(500, 'Failed to load nodes data');
	}
};