// Package prompts holds user-facing text for MCP prompts (ported from mbrt/kustomize-mcp).
package prompts

// Usage describes available tools and common workflows (reference server USAGE string).
const Usage = `The following tools are available to you for rendering Kubernetes
manifests from Kustomize configurations, comparing outputs, and troubleshooting
resource provenance: create_checkpoint, render, diff_checkpoints, diff_paths,
dependencies, inventory, trace.

To compare changes in a Kustomize configuration over time, create a checkpoint,
render the configuration into it, make your changes, render again into a new
checkpoint, and then diff the two checkpoints. This will show you the effects of
your changes, and is especially useful to verify that refactorings do not change
the resulting configuration.

To compare Kustomize configurations in two separate directories, create a
checkpoint, render both directories into it, and then diff the two rendered
paths.

To inspect what resources a Kustomize directory produces, create a checkpoint,
render the directory, and use the inventory tool to list all resources with their
origin and transformer metadata. Use the filter parameter to narrow results by
kind, api_version, namespace, or name.

To trace the provenance of a specific resource, render the directory and use the
trace tool with the resource's kind and name. It returns the source file that
introduced the resource and an ordered list of transformers that modified it.`

// ExplainBody returns the single user message body for the explain prompt.
func ExplainBody(query string) string {
	return Usage + "\n\nExplain the Kustomize configuration in the current project " +
		"by answering the question: " + query
}

// RefactorBody returns the single user message body for the refactor prompt.
func RefactorBody(query string) string {
	return Usage + "\n\nRefactor the Kustomize configuration in the current project " +
		"by fulfilling the request: " + query
}

// TroubleshootBody returns the user message body for the troubleshoot prompt.
func TroubleshootBody(path, kind, name string) string {
	return Usage + "\n\nTroubleshoot the Kustomize resource " + kind + "/" + name +
		" in the directory " + path + ". Create a checkpoint, render the directory, " +
		"then use the trace tool to show where the resource originated and what " +
		"transformers modified it. Use the inventory tool if you need to find the " +
		"exact resource name or kind first."
}

// DiffDirsMessages returns the two user message bodies for the diff_dirs prompt.
func DiffDirsMessages(path1, path2 string) [2]string {
	return [2]string{
		"Explain the differences between the Kustomize directories at " + path1 + " and " + path2 + ".",
		Usage + " To compare the two Kustomize directories, create a checkpoint, " +
			"render both directories into it, and then diff the rendered outputs, " +
			"by using the create_checkpoint and diff_paths tools.",
	}
}
