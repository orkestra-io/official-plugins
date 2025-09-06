package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/go-plugin"
	shared "github.com/orkestra-io/orkestra-shared"
)

type FSExecutor struct{}

func (c *FSExecutor) GetCapabilities() ([]string, error) {
	return []string{"fs/read"}, nil
}

func (c *FSExecutor) Execute(node shared.Node, ctx shared.ExecutionContext) (interface{}, error) {
	withMap := node.With

	switch node.Uses {
	case "fs/read":
		return executeFSRead(withMap)
	default:
		return nil, fmt.Errorf("type de noeud inconnu dans le plugin fs: '%s'", node.Uses)
	}
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"executor": &shared.NodeExecutorPlugin{Impl: &FSExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

func executeFSRead(with map[string]interface{}) (interface{}, error) {
	path, ok := with["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("le paramètre 'path' est requis et doit être une chaîne de caractères pour fs/read")
	}

	log.Printf("LOG | Reading file from path: %s", path)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file '%s': %w", path, err)
	}

	return map[string]interface{}{
		"content": string(content),
		"size":    len(content),
	}, nil
}
