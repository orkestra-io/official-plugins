package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	shared "github.com/orkestra-io/orkestra-shared"
)

// CmdExecutor implémente l'interface NodeExecutor pour l'exécution de commandes.
type CmdExecutor struct{}

// GetCapabilities annonce les types de nœuds que ce plugin fournit.
func (c *CmdExecutor) GetCapabilities() ([]string, error) {
	return []string{"cmd/run"}, nil
}

// Execute est appelée par le moteur Orkestra pour exécuter le nœud.
func (c *CmdExecutor) Execute(node shared.Node, ctx shared.ExecutionContext) (interface{}, error) {
	withMap := node.With

	switch node.Uses {
	case "cmd/run":
		return executeCmdRun(withMap)
	default:
		return nil, fmt.Errorf("type de noeud inconnu dans le plugin cmd: '%s'", node.Uses)
	}
}

// main est le point d'entrée du plugin.
func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"executor": &shared.NodeExecutorPlugin{Impl: &CmdExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

// executeCmdRun contient la logique métier pour exécuter une commande.
func executeCmdRun(with map[string]interface{}) (interface{}, error) {
	command, ok := with["run"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("le paramètre 'run' est requis et doit être une chaîne de caractères")
	}

	shell, _ := with["shell"].(string)
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell"
		} else {
			shell = "sh"
		}
	}

	cwd, _ := with["cwd"].(string)

	log.Printf("LOG | Executing in shell '%s': %s", shell, command)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.Dir = cwd

	// Hérite de l'environnement du moteur par défaut
	cmd.Env = os.Environ()

	// Ajoute les variables d'environnement spécifiées par l'utilisateur
	if env, ok := with["env"].(map[string]interface{}); ok {
		for key, val := range env {
			// Ajoute ou remplace les variables existantes
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", key, val))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		} else {
			return nil, fmt.Errorf("failed to run command: %w. Stderr: %s", err, stderr.String())
		}
	}

	result := map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	}

	if exitCode != 0 {
		return result, fmt.Errorf("command failed with exit code %d", exitCode)
	}

	return result, nil
}
