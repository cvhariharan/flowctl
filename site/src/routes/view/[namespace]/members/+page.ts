import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const ssr = false;

export const load: PageLoad = async ({ params, parent }) => {
	const { user, namespaceId } = await parent();

	// Check permissions
	try {
		const permissions = await permissionChecker(user!, 'member', namespaceId, ['view']);
		if (!permissions.canRead) {
			error(403, {
				message: 'You do not have permission to view members in this namespace',
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

		// Fetch namespace members
		const membersResponse = await apiClient.namespaces.members.list(namespace);

		return {
			members: membersResponse.members || [],
			namespace
		};
	} catch (err) {
		error(500, 'Failed to load members data');
	}
};