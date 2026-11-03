<script lang="ts">
  import { page } from '$app/state';
  import { goto, invalidateAll } from '$app/navigation';
  import { apiClient, ApiError } from '$lib/apiClient';
  import Header from '$lib/components/shared/Header.svelte';
  import Table from '$lib/components/shared/Table.svelte';
  import OatSelect from '$lib/components/shared/OatSelect.svelte';
  import Pagination from '$lib/components/shared/Pagination.svelte';
  import CodeEditor from '$lib/components/shared/CodeEditor.svelte';
  import FlowDiff from '$lib/components/shared/FlowDiff.svelte';
  import { formatDateTime } from '$lib/utils';
  import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
  import { autofocus } from '$lib/utils/autofocus';
  import type { FlowVersionListItem, FlowVersionSource, FlowVersionDiff, TableColumn, TableAction } from '$lib/types';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  const namespace = $derived(page.params.namespace!);
  const flowId = $derived(page.params.flowId!);
  const baseUrl = $derived(`/view/${encodeURIComponent(namespace)}/flows/${flowId}`);
  let history = $derived(data.history);
  let currentPage = $state(1);
  let loading = $state(false);
  let source = $state<FlowVersionSource | null>(null);
  let from = $derived(String(data.diff.from));
  let to = $derived(String(data.diff.to));
  let target = $state<number | null>(null);
  let switchDiff = $state<FlowVersionDiff | null>(null);
  let switching = $state(false);
  let switchError = $state('');
  let dialogEl = $state<HTMLDialogElement>();
  const versions = $derived(Array.from({ length: data.history.versions[0]?.version || 0 }, (_, i) => i + 1).reverse());

  const versionOptions = $derived(versions.map(version => ({ value: String(version), label: String(version) })));

  $effect(() => {
    data.history;
    currentPage = 1;
  });
  $effect(() => { if (target !== null) dialogEl?.showModal(); });

  const columns: TableColumn<FlowVersionListItem>[] = [
    { key: 'version', header: 'Version' },
    { key: 'created_at', header: 'Created', render: (value) => formatDateTime(value) },
    { key: 'active', header: 'Status', render: (active) => active ? '<span class="badge">Active</span>' : '' },
  ];
  const actions: TableAction<FlowVersionListItem>[] = [
    { label: 'View', onClick: (row) => viewSource(row.version) },
    { label: 'Compare with active', href: (row) => `${baseUrl}/versions?from=${row.version}&to=${data.activeVersion}` },
    { label: 'Switch to this version', visible: (row) => data.canUpdate && !row.active, onClick: (row) => prepareSwitch(row.version) },
  ];

  async function loadPage(n: number) {
    loading = true;
    try {
      history = await apiClient.flows.versions.list(namespace, flowId, { page: n });
      currentPage = n;
    } catch (err) { handleInlineError(err, 'Failed to load versions'); }
    finally { loading = false; }
  }

  async function viewSource(version: number) {
    try { source = await apiClient.flows.versions.get(namespace, flowId, version); }
    catch (err) { handleInlineError(err, 'Failed to load source'); }
  }

  async function prepareSwitch(version: number) {
    try {
      switchDiff = await apiClient.flows.versions.diff(namespace, flowId, data.activeVersion, version);
      switchError = '';
      target = version;
    } catch (err) { handleInlineError(err, 'Failed to compare versions'); }
  }

  async function activate() {
    if (target === null) return;
    switching = true;
    try {
      const result = await apiClient.flows.versions.activate(namespace, flowId, target);
      dialogEl?.close();
      target = null;
      source = null;
      showSuccess('Flow version switched', `Switched to version ${result.version}. The flow file was updated.`);
      await invalidateAll();
    } catch (err) {
      switchError = err instanceof ApiError ? err.data?.error || err.message : 'Failed to switch version';
    } finally { switching = false; }
  }
</script>

<svelte:head><title>Flow versions - {flowId} | Flowctl</title></svelte:head>

<Header breadcrumbs={[
  { label: 'Flows', url: `/view/${encodeURIComponent(namespace)}/flows` },
  { label: flowId, url: baseUrl },
  { label: 'Versions' },
]} />

<div class="container mt-6 mb-6 min-w-0">
  <h1>Versions</h1>
  <Table {columns} data={history.versions} {actions} {loading} emptyMessage="No versions" />
  <Pagination {currentPage} totalPages={history.page_count} {loading} on:page-change={(event) => loadPage(event.detail.page)} />

  {#if source}
    <section class="card mt-6 min-w-0">
      <header class="hstack justify-between"><h2>Source: version {source.version}</h2><button class="outline small" onclick={() => source = null}>Close</button></header>
      <CodeEditor value={source.source} language={source.format} readonly height="400px" />
    </section>
  {/if}

  <section class="card comparison mt-6 min-w-0">
    <header><h2>Compare versions</h2></header>
    <form class="compare-controls hstack gap-4" onsubmit={(event) => { event.preventDefault(); goto(`${baseUrl}/versions?from=${from}&to=${to}`); }}>
      <div class="compare-field">
        <label for="version-from">From</label>
        <OatSelect id="version-from" bind:value={from} options={versionOptions} />
      </div>
      <div class="compare-field">
        <label for="version-to">To</label>
        <OatSelect id="version-to" bind:value={to} options={versionOptions} />
      </div>
      <button type="submit">Compare</button>
    </form>
    <p class="text-light text-sm mt-2">Showing {data.diff.from} → {data.diff.to}</p>
    <FlowDiff lines={data.diff.lines} />
  </section>
</div>

{#if target !== null}
  <dialog bind:this={dialogEl} onclose={() => { target = null; }} oncancel={(event) => { if (switching) event.preventDefault(); }}>
    <header><h3>Switch to version {target}</h3></header>
    <section>
      {#if switchDiff}<FlowDiff lines={switchDiff.lines} />{/if}
      {#if switchError}<div role="alert" data-variant="danger" class="mt-4">{switchError}</div>{/if}
    </section>
    <footer>
      <button data-variant="secondary" disabled={switching} onclick={() => dialogEl?.close()} use:autofocus>Cancel</button>
      <button disabled={switching} onclick={activate}>{switching ? 'Switching…' : `Switch to version ${target}`}</button>
    </footer>
  </dialog>
{/if}

<style>
  .compare-controls { align-items: flex-end; }
  .compare-field { min-width: 10rem; }
  dialog { width: min(64rem, calc(100vw - 2rem)); }
  dialog section { max-height: 65vh; }
</style>
