package render

import (
	"gopkg.in/yaml.v3"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
)

// applyCommonMetadataYAML merges Flux spec.commonMetadata into a rendered manifest document.
// Flux labels and annotations override existing keys on conflict.
func applyCommonMetadataYAML(doc []byte, cm *flux.CommonMetadata) ([]byte, error) {
	if cm == nil || (len(cm.Labels) == 0 && len(cm.Annotations) == 0) {
		return doc, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return doc, nil
	}
	meta, _ := root["metadata"].(map[string]any)
	if meta == nil {
		meta = make(map[string]any)
		root["metadata"] = meta
	}
	if len(cm.Labels) > 0 {
		labels, _ := meta["labels"].(map[string]any)
		if labels == nil {
			labels = make(map[string]any)
		}
		for k, v := range cm.Labels {
			labels[k] = v
		}
		meta["labels"] = labels
	}
	if len(cm.Annotations) > 0 {
		ann, _ := meta["annotations"].(map[string]any)
		if ann == nil {
			ann = make(map[string]any)
		}
		for k, v := range cm.Annotations {
			ann[k] = v
		}
		meta["annotations"] = ann
	}
	return yaml.Marshal(root)
}
