<script lang="ts">
  import logoSvg from '$lib/assets/logo.svg';
  import fullLogoSvg from '$lib/assets/full-logo.svg';

  let {
    height = 'h-28',
    iconOnly = false
  }: {
    height?: string;
    iconOnly?: boolean;
  } = $props();
</script>

<!-- Logo -->
<img
  src={iconOnly ? logoSvg : fullLogoSvg}
  alt="Flowctl"
  class="{height} w-auto"
  style="image-rendering: -webkit-optimize-contrast; image-rendering: crisp-edges;"
/>
