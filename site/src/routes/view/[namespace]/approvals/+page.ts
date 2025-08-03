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
		const status = url.searchParams.get('status') || '';

		// Fetch approvals data
		const approvalsResponse = await apiClient.approvals.list(namespace, {
			page,
			count_per_page: DEFAULT_PAGE_SIZE,
			filter: search || undefined,
			status: status as any || undefined
		});

		return {
			approvals: approvalsResponse.approvals || [],
			totalCount: approvalsResponse.total_count || 0,
			pageCount: approvalsResponse.page_count || 1,
			currentPage: page,
			searchQuery: search,
			statusFilter: status,
			namespace
		};
	} catch (err) {
		console.error('Failed to load approvals data:', err);
		throw error(500, 'Failed to load approvals data');
	}
};