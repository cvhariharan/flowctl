import type { ActionStatus, ExecutionMode } from '$lib/types';

export type StepStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'failed'
  | 'awaiting_approval'
  | 'cancelled'
  | 'skipped';

const statusMap: Record<ActionStatus, StepStatus> = {
  pending: 'pending',
  running: 'running',
  completed: 'completed',
  failed: 'failed',
  skipped: 'skipped',
  blocked: 'awaiting_approval',
  cancelled: 'cancelled'
};

export function actionStatusToStepStatus(status: ActionStatus): StepStatus {
  return statusMap[status] ?? 'pending';
}

type HasNeeds = { id: string; needs?: string[] };

/**
 * The server rejects needs outside dag mode, so a flow switched back to sequential must not carry
 * the dependencies the editor still has in state.
 */
export function needsForMode(
  needs: string[] | undefined,
  mode: string | undefined
): string[] | undefined {
  if (mode !== 'dag' || !needs || needs.length === 0) return undefined;
  return needs;
}

/**
 * Groups actions into topological levels. Every action in a level can run at the same time.
 * Actions in a cycle or with unknown dependencies are placed at the end so the editor can still
 * render a flow the server would reject.
 */
export function actionLevels<T extends HasNeeds>(actions: T[]): T[][] {
  const known = new Set(actions.map((a) => a.id));
  const depth = new Map<string, number>();
  const remaining = new Map<string, string[]>();

  for (const action of actions) {
    remaining.set(
      action.id,
      (action.needs ?? []).filter((dep) => known.has(dep) && dep !== action.id)
    );
  }

  let progressed = true;
  while (progressed) {
    progressed = false;
    for (const action of actions) {
      if (depth.has(action.id)) continue;
      const deps = remaining.get(action.id) ?? [];
      if (deps.some((dep) => !depth.has(dep))) continue;
      depth.set(action.id, deps.reduce((max, dep) => Math.max(max, depth.get(dep)! + 1), 0));
      progressed = true;
    }
  }

  const unresolved = actions.filter((a) => !depth.has(a.id));
  const maxDepth = depth.size === 0 ? -1 : Math.max(...depth.values());

  const levels: T[][] = [];
  for (const action of actions) {
    const level = depth.get(action.id) ?? maxDepth + 1;
    while (levels.length <= level) levels.push([]);
    levels[level].push(action);
  }

  return unresolved.length === actions.length && actions.length > 0 ? [actions] : levels;
}

/**
 * Returns the stages shown by an execution pipeline. DAG actions are grouped
 * by dependency depth; sequential actions always occupy one stage each.
 */
export function actionStages<T extends HasNeeds>(
  actions: T[],
  executionMode: ExecutionMode = 'dag'
): T[][] {
  return executionMode === 'dag' ? actionLevels(actions) : actions.map((action) => [action]);
}

function walk<T extends HasNeeds>(
  actions: T[],
  id: string,
  edges: Map<string, string[]>
): Set<string> {
  const seen = new Set<string>();
  const queue = [...(edges.get(id) ?? [])];
  while (queue.length > 0) {
    const next = queue.shift()!;
    if (seen.has(next)) continue;
    seen.add(next);
    queue.push(...(edges.get(next) ?? []));
  }
  return seen;
}

function childEdges<T extends HasNeeds>(actions: T[]): Map<string, string[]> {
  const children = new Map<string, string[]>();
  for (const action of actions) {
    for (const dep of action.needs ?? []) {
      children.set(dep, [...(children.get(dep) ?? []), action.id]);
    }
  }
  return children;
}

function parentEdges<T extends HasNeeds>(actions: T[]): Map<string, string[]> {
  return new Map(actions.map((a) => [a.id, a.needs ?? []]));
}

/** Returns the ids of every action reachable from id, following needs edges forwards. */
export function descendants<T extends HasNeeds>(actions: T[], id: string): Set<string> {
  return walk(actions, id, childEdges(actions));
}

/** Returns the ids of every action id depends on, directly or transitively. */
export function ancestors<T extends HasNeeds>(actions: T[], id: string): Set<string> {
  return walk(actions, id, parentEdges(actions));
}

/** The full dependency path through an action: the action, everything it waits for, and everything waiting on it. */
export function dependencyPath<T extends HasNeeds>(actions: T[], id: string): Set<string> {
  const path = new Set<string>([id]);
  for (const other of ancestors(actions, id)) path.add(other);
  for (const other of descendants(actions, id)) path.add(other);
  return path;
}
