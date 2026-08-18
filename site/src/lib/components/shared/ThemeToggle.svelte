<script lang="ts">
    import { resolvedTheme, theme, type Theme } from '$lib/stores/theme';
    import IconSun from '@tabler/icons-svelte/icons/sun';
    import IconMoon from '@tabler/icons-svelte/icons/moon';
    import IconDeviceDesktop from '@tabler/icons-svelte/icons/device-desktop';

    let { collapsed = false }: { collapsed?: boolean } = $props();

    const icons = { light: IconSun, dark: IconMoon, system: IconDeviceDesktop };
    const nextLabels: Record<Theme, string> = { system: 'Switch to system theme', light: 'Switch to light mode', dark: 'Switch to dark mode' };

    let nextTheme = $derived.by<Theme>(() => {
        if ($theme === 'system') return $resolvedTheme === 'dark' ? 'light' : 'dark';
        return $theme === 'dark' ? 'system' : 'dark';
    });

    function cycle() {
        theme.set(nextTheme);
    }
</script>

<button
    type="button"
    onclick={cycle}
    class={collapsed ? "ghost icon small" : "ghost hstack gap-3 w-100"}
    aria-label={nextLabels[nextTheme]}
    title={nextLabels[nextTheme]}
>
    <span class="theme-icon" aria-hidden="true">
        <svelte:component this={icons[$theme]} size={collapsed ? 18 : 20} />
    </span>
    {#if !collapsed}
        <span class="theme-label">{$theme}</span>
    {/if}
</button>

<style>
    .theme-label {
        text-transform: capitalize;
    }

    .theme-icon {
        display: inline-flex;
        transition: transform 0.25s ease;
    }

    button:active .theme-icon {
        transform: rotate(60deg);
    }
</style>
