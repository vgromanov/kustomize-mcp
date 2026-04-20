package render

import (
	"strings"
	"testing"

	"github.com/vgromanov/kustomize-mcp/internal/flux"
)

func TestInjectFluxFields_mergePatchesAndReplaceNamespace(t *testing.T) {
	base := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: dev
resources:
- a.yaml
patches:
- patch: |-
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: x
  target:
    kind: ConfigMap
    name: x
`
	spec := flux.FluxKustomizationSpec{
		TargetNamespace: "prod",
		NamePrefix:      "p-",
		NameSuffix:      "-s",
		Patches: []map[string]any{{
			"patch":  "apiVersion: v1\nkind: Service\nmetadata:\n  name: y\n",
			"target": map[string]any{"kind": "Service", "name": "y"},
		}},
		Images: []map[string]any{
			{"name": "nginx", "newName": "reg/nginx", "newTag": "1.0"},
		},
		Components: []string{"comp1"},
	}
	out1, err := injectFluxFields([]byte(base), &spec)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := injectFluxFields(out1, &spec)
	if err != nil {
		t.Fatal(err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("expected idempotent inject on patches+metadata, got different lengths %d vs %d", len(out1), len(out2))
	}
}

func TestInjectFluxFields_imageReplaceByName(t *testing.T) {
	base := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
images:
- name: nginx
  newTag: "1.25"
`
	spec := flux.FluxKustomizationSpec{
		Images: []map[string]any{
			{"name": "nginx", "newName": "my/reg", "newTag": "1.27"},
		},
	}
	out, err := injectFluxFields([]byte(base), &spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "my/reg") || !strings.Contains(string(out), "1.27") {
		t.Fatalf("%s", string(out))
	}
}
