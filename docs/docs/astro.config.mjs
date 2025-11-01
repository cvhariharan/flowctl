// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import rehypeMermaid from "rehype-mermaid";

// https://astro.build/config
export default defineConfig({
  markdown: {
    syntaxHighlight: {
      type: "shiki",
      excludeLangs: ["mermaid", "math"],
    },
    rehypePlugins: [rehypeMermaid],
  },
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
            { label: "Flows", slug: "general/flows" },
            { label: "Nodes", slug: "general/nodes-and-executors" },
            { label: "Access Control", slug: "general/access-control" },
          ],
        },
        {
          label: "Development",
          autogenerate: {
            directory: "development",
          },
        },
      ],
    }),
  ],
});
