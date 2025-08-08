import { apiClient } from '$lib/apiClient';
import { FLOWS_PER_PAGE } from '$lib/constants';

export const load = async ({ params, url, parent }: { params: any; url: any; parent: any }) => {
  const { namespaceId } = await parent();
  const page = Number(url.searchParams.get('page')) || 1;
  const filter = url.searchParams.get('filter') || '';
  
  try {
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
  } catch (error) {
    console.error('Failed to load flows:', error);
    return {
      flows: [],
      pageCount: 0,
      totalCount: 0,
      currentPage: 1,
      filter,
      error: 'Failed to load flows',
      namespaceId
    };
  }
};