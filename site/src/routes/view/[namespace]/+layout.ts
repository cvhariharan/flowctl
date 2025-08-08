import { error } from '@sveltejs/kit';
import { apiClient } from '$lib/apiClient.js';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ params, parent }) => {
  const { user } = await parent();
  const namespace = params.namespace;
  
  // Check if user has access to this namespace and get the namespace object
  try {
    const namespacesResponse = await apiClient.namespaces.list();
    const namespaceObject = namespacesResponse.namespaces.find(ns => ns.name === namespace);
    
    if (!namespaceObject) {
      error(403, 'Access denied. You do not have permission to access this namespace.');
    }

    return {
      namespace: namespaceObject.name,
      namespaceId: namespaceObject.id
    };
  } catch (err) {
    error(500, 'Could not retrieve the namespace');
  }
};