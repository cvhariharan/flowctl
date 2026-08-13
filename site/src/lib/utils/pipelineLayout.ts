import type { ExecutionMode } from '$lib/types';
import { actionStages } from './dag';

export const NODE_WIDTH = 190;
export const NODE_HEIGHT = 54;
export const GAP_X = 68;
export const GAP_Y = 14;
export const STAGE_HEADER = 30;

type HasNeeds = { id: string; needs?: string[] };

export type PlacedNode<T> = {
  action: T;
  x: number;
  y: number;
};

export type Stage<T> = {
  index: number;
  x: number;
  nodes: PlacedNode<T>[];
};

export type Edge = {
  from: string;
  to: string;
  path: string;
};

export type PipelineLayout<T> = {
  stages: Stage<T>[];
  edges: Edge[];
  width: number;
  height: number;
};

/**
 * Orders the actions inside each stage so an action sits near the actions it depends on. One
 * barycenter pass is enough to keep the common shapes (fan out, fan in, diamond) untangled.
 */
function orderStages<T extends HasNeeds>(levels: T[][]): T[][] {
  const rowOf = new Map<string, number>();
  levels[0]?.forEach((action, row) => rowOf.set(action.id, row));

  for (let i = 1; i < levels.length; i++) {
    const scored = levels[i].map((action, row) => {
      const parentRows = (action.needs ?? [])
        .map((dep) => rowOf.get(dep))
        .filter((r): r is number => r !== undefined);
      const score =
        parentRows.length > 0
          ? parentRows.reduce((sum, r) => sum + r, 0) / parentRows.length
          : row;
      return { action, score, row };
    });

    scored.sort((a, b) => a.score - b.score || a.row - b.row);
    levels[i] = scored.map((s) => s.action);
    levels[i].forEach((action, row) => rowOf.set(action.id, row));
  }

  return levels;
}

export function pipelineLayout<T extends HasNeeds>(
  actions: T[],
  executionMode: ExecutionMode = 'dag'
): PipelineLayout<T> {
  if (actions.length === 0) {
    return { stages: [], edges: [], width: 0, height: 0 };
  }

  const levels = orderStages(actionStages(actions, executionMode));
  const tallest = Math.max(...levels.map((l) => l.length));
  const columnHeight = (rows: number) => rows * NODE_HEIGHT + (rows - 1) * GAP_Y;
  const fullHeight = columnHeight(tallest);

  const position = new Map<string, PlacedNode<T>>();
  const stages: Stage<T>[] = levels.map((level, index) => {
    const x = index * (NODE_WIDTH + GAP_X);
    // Centre shorter columns against the tallest one so connectors stay roughly horizontal.
    const top = STAGE_HEADER + (fullHeight - columnHeight(level.length)) / 2;

    const nodes = level.map((action, row) => {
      const placed = { action, x, y: top + row * (NODE_HEIGHT + GAP_Y) };
      position.set(action.id, placed);
      return placed;
    });

    return { index, x, nodes };
  });

  const edges: Edge[] = [];
  for (const [actionIndex, action] of actions.entries()) {
    const to = position.get(action.id);
    if (!to) continue;

    const dependencies = executionMode === 'dag'
      ? (action.needs ?? [])
      : actionIndex > 0
        ? [actions[actionIndex - 1].id]
        : [];

    for (const dep of dependencies) {
      const from = position.get(dep);
      if (!from) continue;

      const x1 = from.x + NODE_WIDTH;
      const y1 = from.y + NODE_HEIGHT / 2;
      const x2 = to.x;
      const y2 = to.y + NODE_HEIGHT / 2;
      const bend = Math.max((x2 - x1) * 0.5, GAP_X * 0.5);

      edges.push({
        from: dep,
        to: action.id,
        path: `M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`
      });
    }
  }

  return {
    stages,
    edges,
    width: levels.length * NODE_WIDTH + (levels.length - 1) * GAP_X,
    height: STAGE_HEADER + fullHeight
  };
}
