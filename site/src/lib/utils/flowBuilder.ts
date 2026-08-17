import { apiClient } from '$lib/apiClient';
import type {
  BuilderAction,
  BuilderFlow,
  BuilderInput,
  FlowActionReq,
  FlowCreateReq,
  FlowInputReq
} from '$lib/types';
import { createSlug } from '$lib/utils';
import { needsForMode } from './dag';
import { handleInlineError } from './errorHandling';

let sequence = 0;
const nextTempId = () => Date.now() + sequence++;

export function resolveSchema(config: any): any {
  if (!config?.$defs || !config?.$ref) return config;
  return config.$defs[config.$ref.replace('#/$defs/', '')] ?? config;
}

export async function loadExecutorConfigs(
  actions: BuilderAction[]
): Promise<Record<string, any>> {
  const executors = [...new Set(actions.map((a) => a.executor).filter(Boolean))];
  const configs: Record<string, any> = {};

  await Promise.all(
    executors.map(async (executor) => {
      try {
        configs[executor] = resolveSchema(await apiClient.executors.getConfig(executor));
      } catch (error) {
        handleInlineError(error, `Error loading config for executor ${executor}`);
      }
    })
  );

  return configs;
}

/**
 * A new action gets a provisional id so it can be linked before it is named. Naming it replaces
 * the id and rewrites whatever pointed at the provisional one; an unnamed action fails validation
 * and never reaches the API.
 */
export function newAction(executor = '', needs: string[] = []): BuilderAction {
  return {
    tempId: nextTempId(),
    id: `action_${++sequence}`,
    name: '',
    executor,
    with: {},
    withByExecutor: {},
    selectedNodes: [],
    variables: [],
    approval: false,
    allow_node_override: false,
    needs: [...needs]
  };
}

export function newInput(type: FlowInputReq['type'] = 'string'): BuilderInput {
  return {
    name: '',
    type,
    label: '',
    description: '',
    required: false,
    default: '',
    validation: '',
    options: [],
    multiple: false
  };
}

/**
 * Dependencies reference action ids, so a rename has to be carried into them or they would point
 * at an action that no longer exists.
 */
export function renameAction(actions: BuilderAction[], action: BuilderAction, name: string) {
  const previousId = action.id;
  action.name = name;
  // An empty name keeps the old id: ids are the graph's keys and two blank ones would collide.
  action.id = createSlug(name) || previousId;

  if (!previousId || previousId === action.id) return;
  for (const other of actions) {
    if (!other.needs.includes(previousId)) continue;
    other.needs = other.needs.map((dep) => (dep === previousId ? action.id : dep));
  }
}

/** Removes an action and reconnects what waited on it to what it waited for. */
export function removeAction(actions: BuilderAction[], id: string) {
  const index = actions.findIndex((a) => a.id === id);
  if (index < 0) return;

  const [removed] = actions.splice(index, 1);
  for (const other of actions) {
    if (!other.needs.includes(id)) continue;
    other.needs = [...new Set([...other.needs.filter((dep) => dep !== id), ...removed.needs])].filter(
      (dep) => dep !== other.id
    );
  }
}

export function duplicateAction(actions: BuilderAction[], id: string): string {
  const index = actions.findIndex((a) => a.id === id);
  if (index < 0) return '';

  const original = actions[index];
  const taken = new Set(actions.map((a) => a.id));
  let copyId = `${original.id}_copy`;
  while (taken.has(copyId)) copyId = `${copyId}_copy`;

  const copy: BuilderAction = {
    ...JSON.parse(JSON.stringify(original)),
    tempId: nextTempId(),
    id: copyId,
    name: original.name ? `${original.name} (Copy)` : ''
  };
  actions.splice(index + 1, 0, copy);
  return copy.id;
}

/** Adds an action that runs after `afterId`: a dependency in a graph, the next slot in a sequence. */
export function addAfter(actions: BuilderAction[], afterId: string, isDAG: boolean): BuilderAction {
  const action = newAction('', isDAG ? [afterId] : []);
  const index = actions.findIndex((a) => a.id === afterId);

  if (isDAG || index < 0) actions.push(action);
  else actions.splice(index + 1, 0, action);

  return action;
}

/** Adds an action between two, taking over the dependency edge or the list slot. */
export function insertBetween(
  actions: BuilderAction[],
  fromId: string,
  toId: string,
  isDAG: boolean
): BuilderAction {
  const action = newAction('', isDAG ? [fromId] : []);
  const target = actions.find((a) => a.id === toId);

  if (isDAG) {
    actions.push(action);
    if (target) target.needs = [...target.needs.filter((dep) => dep !== fromId), action.id];
  } else {
    const index = actions.findIndex((a) => a.id === toId);
    actions.splice(index < 0 ? actions.length : index, 0, action);
  }

  return action;
}

export function linkActions(actions: BuilderAction[], fromId: string, toId: string) {
  const target = actions.find((a) => a.id === toId);
  if (!target || target.needs.includes(fromId)) return;
  target.needs = [...target.needs, fromId];
}

export function unlinkActions(actions: BuilderAction[], fromId: string, toId: string) {
  const target = actions.find((a) => a.id === toId);
  if (!target) return;
  target.needs = target.needs.filter((dep) => dep !== fromId);
}

/** Resolves an action id to its name, for the places that only hold ids (needs, edges). */
export function nameLookup(actions: BuilderAction[]): (id: string) => string {
  const names = new Map(actions.map((a) => [a.id, a.name]));
  return (id: string) => names.get(id) || id;
}

export function emptyFlow(id = ''): BuilderFlow {
  return {
    metadata: {
      id,
      name: '',
      description: '',
      prefix: '',
      schedules: [],
      allow_overlap: false,
      user_schedulable: false,
      max_retries: 0,
      execution_mode: '',
      max_parallel: 0
    },
    inputs: [],
    actions: [],
    notifications: []
  };
}

/** API config → editor state. */
export function toBuilderFlow(config: any, id = ''): BuilderFlow {
  const metadata = config.metadata ?? {};
  return {
    metadata: {
      id,
      name: metadata.name ?? '',
      description: metadata.description || '',
      prefix: metadata.prefix || '',
      schedules: metadata.schedules || [],
      allow_overlap: metadata.allow_overlap || false,
      user_schedulable: metadata.user_schedulable || false,
      max_retries: metadata.max_retries || 0,
      execution_mode: metadata.execution_mode || '',
      max_parallel: metadata.max_parallel || 0
    },
    inputs: (config.inputs || []).map(toBuilderInput),
    actions: (config.actions || []).map(toBuilderAction),
    notifications: (config.notify || []).map((notification: any) => ({
      channel: notification.channel || '',
      events: notification.events || [],
      config: notification.config || {}
    }))
  };
}

function toBuilderAction(action: any, index: number): BuilderAction {
  return {
    tempId: nextTempId() + index,
    id: action.id || createSlug(action.name ?? ''),
    name: action.name ?? '',
    executor: action.executor ?? '',
    with: action.with ?? {},
    withByExecutor: {},
    selectedNodes: action.on ?? [],
    variables: (action.variables ?? []).map((entry: Record<string, string>) => {
      const [name, value] = Object.entries(entry)[0];
      return { name, value };
    }),
    approval: action.approval ?? false,
    allow_node_override: action.allow_node_override ?? false,
    needs: action.needs ?? []
  };
}

function toBuilderInput(input: any): BuilderInput {
  return {
    name: input.name ?? '',
    type: input.type ?? 'string',
    label: input.label ?? '',
    description: input.description ?? '',
    required: input.required ?? false,
    default: input.default ?? '',
    validation: input.validation ?? '',
    options: input.options ?? [],
    multiple: input.multiple ?? false,
    maxFileSizeMB: input.max_file_size ? input.max_file_size / 1024 / 1024 : undefined,
    useRemoteOptions: !!input.remote_options,
    remote_options: input.remote_options
  };
}

/** Editor state → API request. Create nests the metadata, update flattens it. */
export function toFlowRequest(flow: BuilderFlow): FlowCreateReq {
  const { metadata } = flow;
  return {
    metadata: {
      name: metadata.name,
      description: metadata.description || undefined,
      prefix: metadata.prefix || undefined,
      schedules: metadata.schedules.filter((s) => s.cron.trim()),
      allow_overlap: metadata.allow_overlap,
      user_schedulable: metadata.user_schedulable,
      max_retries: metadata.max_retries || 0,
      execution_mode: metadata.execution_mode || undefined,
      max_parallel: metadata.execution_mode === 'dag' ? metadata.max_parallel || 0 : undefined
    },
    inputs: flow.inputs.filter((i) => i.name).map(toInputRequest),
    actions: flow.actions
      .filter((a) => a.name)
      .map((action) => toActionRequest(action, metadata.execution_mode)),
    notify: flow.notifications.filter((n) => n.channel)
  };
}

function toActionRequest(
  action: BuilderAction,
  executionMode: string | undefined
): FlowActionReq {
  return {
    name: action.name,
    executor: action.executor as FlowActionReq['executor'],
    with: action.with ?? {},
    approval: action.approval,
    allow_node_override: action.allow_node_override,
    variables: action.variables
      .filter((v) => v.name.trim())
      .map((v) => ({ [v.name]: v.value })),
    on: action.selectedNodes.length > 0 ? action.selectedNodes : undefined,
    needs: needsForMode(action.needs, executionMode)
  };
}

function toInputRequest(input: BuilderInput): FlowInputReq {
  const isSelect = input.type === 'select';
  return {
    name: input.name,
    type: input.type,
    label: input.label || undefined,
    description: input.description || undefined,
    validation: input.validation || undefined,
    required: input.required,
    default: input.default || undefined,
    options: isSelect && !input.useRemoteOptions && input.options.length > 0
      ? input.options.filter((option) => option.trim())
      : undefined,
    remote_options: isSelect && input.useRemoteOptions && input.remote_options?.url
      ? {
          url: input.remote_options.url,
          method: input.remote_options.method || undefined,
          headers: Object.keys(input.remote_options.headers ?? {}).length > 0
            ? input.remote_options.headers
            : undefined
        }
      : undefined,
    max_file_size: input.type === 'file' && input.maxFileSizeMB
      ? input.maxFileSizeMB * 1024 * 1024
      : undefined,
    multiple: input.type === 'node' ? input.multiple : undefined
  };
}
