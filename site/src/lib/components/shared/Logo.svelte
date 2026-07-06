<script lang="ts">
  import logoSvg from '$lib/assets/logo.svg';
  import fullLogoSvg from '$lib/assets/full-logo.svg';
  import fullLogoLightSvg from '$lib/assets/full-logo-light.svg';
  import { resolvedTheme } from '$lib/stores/theme';
  import { appInfo } from '$lib/stores/auth';

  let {
    height = '7rem',
    iconOnly = false
  }: {
    height?: string;
    iconOnly?: boolean;
  } = $props();

  let logoSrc = $derived(
    iconOnly
      ? ($appInfo?.branding?.logo_url || logoSvg)
      : $resolvedTheme === 'dark'
        ? ($appInfo?.branding?.logo_light_url || $appInfo?.branding?.logo_url || fullLogoLightSvg)
        : ($appInfo?.branding?.logo_url || fullLogoSvg)
  );
</script>

<!-- Logo -->
<img
  src={logoSrc}
  alt={$appInfo?.branding?.app_name || "Flowctl"}
  style="height: {height}; width: auto; image-rendering: -webkit-optimize-contrast; image-rendering: crisp-edges;"
/>
