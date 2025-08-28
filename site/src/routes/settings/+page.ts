import type { PageLoad } from './$types';
import { apiClient } from '$lib/apiClient';
import { DEFAULT_PAGE_SIZE } from '$lib/constants';
import type { GroupsPaginateResponse, UsersPaginateResponse, NamespacesPaginateResponse } from '$lib/types';

export const ssr = false;

export const load: PageLoad = async () => {
	let usersResponse: UsersPaginateResponse | null = null;
	let groupsResponse: GroupsPaginateResponse | null = null;
	let namespacesResponse: NamespacesPaginateResponse | null = null;
	
	try {
		usersResponse = await apiClient.users.list({
			page: 1,
			count_per_page: DEFAULT_PAGE_SIZE
		});
	
		groupsResponse = await apiClient.groups.list({
			page: 1,
			count_per_page: DEFAULT_PAGE_SIZE
		});

		namespacesResponse = await apiClient.namespaces.list({
			page: 1,
			count_per_page: DEFAULT_PAGE_SIZE
		});
	} catch(error) {
		console.error("failed to fetch users, groups, and namespaces: ", error);
	}

	return {
		users: usersResponse?.users || [],
		usersTotalCount: usersResponse?.total_count || 0,
		usersPageCount: usersResponse?.page_count || 1,
		groups: groupsResponse?.groups || [],
		groupsTotalCount: groupsResponse?.total_count || 0,
		groupsPageCount: groupsResponse?.page_count || 1,
		namespaces: namespacesResponse?.namespaces || [],
		namespacesTotalCount: namespacesResponse?.total_count || 0,
		namespacesPageCount: namespacesResponse?.page_count || 1
	};
};