// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  integrations: [
    starlight({
      title: "flowctl",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/cvhariharan/flowctl",
        },
      ],
      sidebar: [
        {
          label: "General",
          items: [
            { label: "Getting Started", slug: "general/getting-started" },
            { label: "Flows", slug: "general/flows" },
            { label: "Nodes", slug: "general/nodes-and-executors" },
            { label: "Execution", slug: "general/execution" },
            { label: "Access Control", slug: "general/access-control" },
          ],
        },
        {
          label: "Advanced",
          autogenerate: {
            directory: "advanced"
          }
        }
      ],
    }),
  ],
});
