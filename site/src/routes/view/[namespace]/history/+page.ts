import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { DEFAULT_PAGE_SIZE } from '$lib/constants';

export const ssr = false;

export const load: PageLoad = async ({ params, url }) => {
	try {
		const { namespace } = params;
		const page = Number(url.searchParams.get('page') || '1');
		const search = url.searchParams.get('search') || '';

		// Fetch execution history data
		const executionsResponse = await apiClient.executions.list(namespace, {
			page,
			count_per_page: DEFAULT_PAGE_SIZE,
			filter: search || undefined
		});

		return {
			executions: executionsResponse.executions || [],
			totalCount: executionsResponse.total_count || 0,
			pageCount: executionsResponse.page_count || 1,
			currentPage: page,
			searchQuery: search,
			namespace
		};
	} catch (err) {
		console.error('Failed to load execution history:', err);
		throw error(500, 'Failed to load execution history');
	}
};