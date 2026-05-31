package tools

import "testing"

func TestRegistryReturnsToolsInNameOrder(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&recordingTool{name: "zeta"})
	registry.Register(&recordingTool{name: "alpha"})

	all := registry.All()
	if len(all) != 2 || all[0].Name() != "alpha" || all[1].Name() != "zeta" {
		t.Fatalf("tool order = %v, want [alpha zeta]", toolNames(all))
	}

	definitions := registry.ToFunctionDefinitions()
	first := definitions[0]["function"].(map[string]any)["name"]
	second := definitions[1]["function"].(map[string]any)["name"]
	if first != "alpha" || second != "zeta" {
		t.Fatalf("definition order = [%v %v], want [alpha zeta]", first, second)
	}
}

func toolNames(tools []Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	return names
}
