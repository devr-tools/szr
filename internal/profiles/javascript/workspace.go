package javascript

import "strings"

func isJSWorkspaceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn":
		return !isPackageManagerTest(args)
	case "biome", "bun", "esbuild", "eslint", "next", "node", "nuxt", "nx", "rollup", "swc", "swc-cli", "ts-node", "tsc", "tsx", "tsup", "turbo", "vite", "webpack":
		if args[0] != "node" {
			return true
		}
		return len(args) >= 2 && (strings.HasSuffix(args[1], ".mjs") || strings.HasSuffix(args[1], ".cjs"))
	default:
		return false
	}
}
