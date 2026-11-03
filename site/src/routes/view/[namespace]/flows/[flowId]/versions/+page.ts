import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { apiClient, ApiError } from '$lib/apiClient';
import { permissionChecker } from '$lib/utils/permissions';

export const load: PageLoad = async ({ params, parent, url }) => {
  const { user, namespaceId } = await parent();
  const permissions = await permissionChecker(user!, 'flow', namespaceId, ['view_config', 'update'], '_');
  if (!permissions.canViewConfig) error(403, { message: 'You do not have permission to view flow versions' });

  try {
    const history = await apiClient.flows.versions.list(params.namespace, params.flowId);
    const from = Number(url.searchParams.get('from') || history.versions[0]?.version);
    const to = url.searchParams.has('to') ? Number(url.searchParams.get('to')) : undefined;
    const diff = await apiClient.flows.versions.diff(params.namespace, params.flowId, from, to);
    // Omitting the target resolves the active version, including after a switch back.
    const activeVersion = to === undefined
      ? diff.to
      : (await apiClient.flows.versions.diff(params.namespace, params.flowId, from)).to;
    return { history, diff, activeVersion, canUpdate: permissions.canUpdate };
  } catch (err) {
    if (err instanceof ApiError) error(err.status, { message: err.data?.error || 'Failed to load flow versions', code: err.data?.code });
    error(500, { message: 'Failed to load flow versions' });
  }
};
