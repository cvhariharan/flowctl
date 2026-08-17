import {
  IconLetterCase,
  IconNumbers,
  IconToggleLeft,
  IconLock,
  IconFile,
  IconCalendar,
  IconList,
  IconServer
} from '@tabler/icons-svelte';
import type { BuilderAction, BuilderInput, FlowInputReq } from '$lib/types';

/** The input types the API accepts, in the order the picker shows them. */
export const INPUT_TYPES: Array<{ value: FlowInputReq['type']; label: string; icon: any }> = [
  { value: 'string', label: 'String', icon: IconLetterCase },
  { value: 'number', label: 'Number', icon: IconNumbers },
  { value: 'checkbox', label: 'Checkbox', icon: IconToggleLeft },
  { value: 'password', label: 'Password', icon: IconLock },
  { value: 'file', label: 'File', icon: IconFile },
  { value: 'datetime', label: 'Datetime', icon: IconCalendar },
  { value: 'select', label: 'Select', icon: IconList },
  { value: 'node', label: 'Node', icon: IconServer }
];

export const inputIcon = (type: string) =>
  INPUT_TYPES.find((t) => t.value === type)?.icon ?? IconLetterCase;

export interface InputReferences {
  byName: Map<string, string[]>;
  nodeOverride: string[];
}

/**
 * Actions that read each input, in one pass over the flow: scanning per input card instead
 * re-serialises every action's payload once per card.
 */
export function inputReferences(actions: BuilderAction[]): InputReferences {
  const byName = new Map<string, string[]>();
  const nodeOverride: string[] = [];

  for (const action of actions) {
    const label = action.name || action.id;
    if (action.allow_node_override) nodeOverride.push(label);

    const haystack =
      JSON.stringify(action.with ?? {}) + action.variables.map((v) => v.value ?? '').join('\n');

    for (const [, name] of haystack.matchAll(/\{\{\s*inputs\.(\w+)\s*\}\}/g)) {
      const seen = byName.get(name);
      if (!seen) byName.set(name, [label]);
      else if (!seen.includes(label)) seen.push(label);
    }
  }

  return { byName, nodeOverride };
}

export function referencesFor(input: BuilderInput, references: InputReferences): string[] {
  if (!input.name) return [];

  const used = references.byName.get(input.name) ?? [];
  if (input.type !== 'node') return used;

  return [...new Set([...used, ...references.nodeOverride])];
}

export function applyInputType(input: BuilderInput, type: FlowInputReq['type']) {
  input.type = type;

  if (type === 'file') {
    input.default = '';
  }
  if (type === 'node' && input.multiple === undefined) {
    input.multiple = false;
  }
}
