import type { BuilderFlow, FlowProblem } from '$lib/types';
import { cyclicActions } from './dag';

const label = (name: string) => name?.trim() || 'Untitled';

function requiredKeys(schema: any): Array<{ key: string; title: string }> {
  return (schema?.required ?? []).map((key: string) => ({
    key,
    title: schema.properties?.[key]?.title || key
  }));
}

/**
 * Everything that would make a save fail, checked continuously instead of on submit. Covers the
 * client-side rules and the dependency rules the server enforces in scheduler.BuildGraph.
 */
export function validateFlow(
  flow: BuilderFlow,
  executorConfigs: Record<string, any> = {}
): FlowProblem[] {
  const problems: FlowProblem[] = [];
  const isDAG = flow.metadata.execution_mode === 'dag';

  if (!flow.metadata.name?.trim()) {
    problems.push({ severity: 'error', message: 'The flow needs a name.', section: 'general' });
  }

  if (flow.actions.length === 0) {
    problems.push({ severity: 'error', message: 'A flow needs at least one action.', section: 'actions' });
  }

  for (const [i, action] of flow.actions.entries()) {
    if (!action.name?.trim()) {
      problems.push({
        severity: 'error',
        message: `Action ${i + 1} has no name.`,
        section: 'actions',
        actionId: action.id
      });
    }
    if (!action.executor) {
      problems.push({
        severity: 'error',
        message: `“${label(action.name)}” has no executor.`,
        section: 'actions',
        actionId: action.id
      });
      continue;
    }
    for (const field of requiredKeys(executorConfigs[action.executor])) {
      if (String(action.with?.[field.key] ?? '').trim()) continue;
      problems.push({
        severity: 'error',
        message: `“${label(action.name)}” is missing ${field.title}.`,
        section: 'actions',
        actionId: action.id
      });
    }
  }

  const seenIds = new Set<string>();
  for (const action of flow.actions) {
    if (!action.id) continue;
    if (seenIds.has(action.id)) {
      problems.push({
        severity: 'error',
        message: `More than one action is called “${label(action.name)}”.`,
        section: 'actions',
        actionId: action.id
      });
    }
    seenIds.add(action.id);
  }

  for (const [i, input] of flow.inputs.entries()) {
    if (!input.name?.trim()) {
      problems.push({ severity: 'error', message: `Input ${i + 1} has no name.`, section: 'inputs' });
    }
  }
  if (flow.inputs.filter((input) => input.type === 'node').length > 1) {
    problems.push({
      severity: 'error',
      message: 'Only one input of type node is allowed per flow.',
      section: 'inputs'
    });
  }

  if (isDAG) {
    const known = new Set(flow.actions.map((a) => a.id));
    for (const action of flow.actions) {
      for (const dep of action.needs ?? []) {
        if (dep === action.id) {
          problems.push({
            severity: 'error',
            message: `“${label(action.name)}” depends on itself.`,
            section: 'actions',
            actionId: action.id
          });
        } else if (!known.has(dep)) {
          problems.push({
            severity: 'error',
            message: `“${label(action.name)}” depends on ${dep}, which no longer exists.`,
            section: 'actions',
            actionId: action.id
          });
        }
      }
    }
    const names = new Map(flow.actions.map((a) => [a.id, a.name]));
    for (const id of cyclicActions(flow.actions)) {
      problems.push({
        severity: 'error',
        message: `“${label(names.get(id) ?? id)}” is part of a dependency cycle.`,
        section: 'actions',
        actionId: id
      });
    }
  } else {
    const carried = flow.actions.filter((a) => (a.needs ?? []).length > 0);
    if (carried.length > 0) {
      problems.push({
        severity: 'warning',
        message: `${carried.length} action(s) still carry dependencies. They are dropped when you save.`,
        section: 'actions',
        actionId: carried[0].id
      });
    }
  }

  return problems;
}
