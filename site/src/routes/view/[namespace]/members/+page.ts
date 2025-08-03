import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';

export const ssr = false;

export const load: PageLoad = async ({ params }) => {
	try {
		const { namespace } = params;

		// Fetch namespace members
		const membersResponse = await apiClient.namespaces.members.list(namespace);

		return {
			members: membersResponse.members || [],
			namespace
		};
	} catch (err) {
		console.error('Failed to load members data:', err);
		throw error(500, 'Failed to load members data');
	}
};