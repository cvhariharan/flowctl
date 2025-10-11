<script lang="ts">
  import logoSvg from '$lib/assets/logo.svg';
  import flowctlSvg from '$lib/assets/flowctl.svg';

  let {
    logoHeight = 'h-28',
    typographyHeight = 'h-24',
    gap = 'gap-4',
    showSubtext = false,
    subtext = '',
    align = 'center'
  }: {
    logoHeight?: string;
    typographyHeight?: string;
    gap?: string;
    showSubtext?: boolean;
    subtext?: string;
    align?: 'left' | 'center' | 'right';
  } = $props();

  const alignClass = {
    left: 'justify-start',
    center: 'justify-center',
    right: 'justify-end'
  }[align];
</script>

<!-- Logo -->
<div class="text-{align}">
  <div class="flex items-center {alignClass}">
    <img src={logoSvg} alt="Flowctl logo" class="{logoHeight} w-auto" />
    <img src={flowctlSvg} alt="Flowctl" class="{typographyHeight} w-auto" />
  </div>
  {#if subtext.length > 0}
    <p class="mt-2 text-sm text-slate-600">{subtext}</p>
  {/if}
</div>