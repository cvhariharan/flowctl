import type { ExecutionMode } from '$lib/types';
import { actionStages, withImplicitNeeds } from './dag';

export type Geometry = {
  nodeWidth: number;
  nodeHeight: number;
  gapX: number;
  gapY: number;
  stageHeader: number;
};

export const DEFAULT_GEOMETRY: Geometry = {
  nodeWidth: 190,
  nodeHeight: 54,
  gapX: 68,
  gapY: 14,
  stageHeader: 30
};

/** Editor nodes carry ports and hover tools, so they need more room than the read-only graph. */
export const EDITOR_GEOMETRY: Geometry = {
  nodeWidth: 220,
  nodeHeight: 78,
  gapX: 88,
  gapY: 22,
  stageHeader: 36
};

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
  /** Midpoint of the curve, where an insert-between control can sit. */
  midX: number;
  midY: number;
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
  executionMode: ExecutionMode = 'dag',
  geometry: Geometry = DEFAULT_GEOMETRY
): PipelineLayout<T> {
  if (actions.length === 0) {
    return { stages: [], edges: [], width: 0, height: 0 };
  }

  const { nodeWidth, nodeHeight, gapX, gapY, stageHeader } = geometry;
  const levels = orderStages(actionStages(actions, executionMode));
  const tallest = Math.max(...levels.map((l) => l.length));
  const columnHeight = (rows: number) => rows * nodeHeight + (rows - 1) * gapY;
  const fullHeight = columnHeight(tallest);

  const position = new Map<string, PlacedNode<T>>();
  const stages: Stage<T>[] = levels.map((level, index) => {
    const x = index * (nodeWidth + gapX);
    // Centre shorter columns against the tallest one so connectors stay roughly horizontal.
    const top = stageHeader + (fullHeight - columnHeight(level.length)) / 2;

    const nodes = level.map((action, row) => {
      const placed = { action, x, y: top + row * (nodeHeight + gapY) };
      position.set(action.id, placed);
      return placed;
    });

    return { index, x, nodes };
  });

  const edges: Edge[] = [];
  for (const action of withImplicitNeeds(actions, executionMode)) {
    const to = position.get(action.id);
    if (!to) continue;

    for (const dep of action.needs ?? []) {
      const from = position.get(dep);
      if (!from) continue;

      const x1 = from.x + nodeWidth;
      const y1 = from.y + nodeHeight / 2;
      const x2 = to.x;
      const y2 = to.y + nodeHeight / 2;
      const bend = Math.max((x2 - x1) * 0.5, gapX * 0.5);

      edges.push({
        from: dep,
        to: action.id,
        path: `M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`,
        // The curve is symmetric, so its midpoint is the midpoint of the endpoints.
        midX: (x1 + x2) / 2,
        midY: (y1 + y2) / 2
      });
    }
  }

  return {
    stages,
    edges,
    width: levels.length * nodeWidth + (levels.length - 1) * gapX,
    height: stageHeader + fullHeight
  };
}
