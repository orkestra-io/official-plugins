package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/hashicorp/go-plugin"
	shared "github.com/orkestra-io/orkestra-shared"
	"golang.org/x/crypto/ssh"
)

type SSHExecutor struct{}

func (e *SSHExecutor) GetCapabilities() ([]string, error) {
	return []string{"ssh/run"}, nil
}

func (e *SSHExecutor) Execute(node shared.Node, ctx shared.ExecutionContext) (interface{}, error) {
	switch node.Uses {
	case "ssh/run":
		// On passe le contexte pour accéder aux secrets
		return executeSSHRun(node.With, &ctx)
	default:
		return nil, fmt.Errorf("unknown node type for ssh plugin: %s", node.Uses)
	}
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"executor": &shared.NodeExecutorPlugin{Impl: &SSHExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

func executeSSHRun(with map[string]interface{}, _ *shared.ExecutionContext) (interface{}, error) {
	host, ok := with["host"].(string)
	if !ok || host == "" {
		return nil, fmt.Errorf("le paramètre 'host' est requis")
	}

	command, ok := with["run"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("le paramètre 'run' est requis")
	}

	key, ok := with["key"].(string)
	if !ok || key == "" {
		return nil, fmt.Errorf("le paramètre 'key' est requis (ex: '{{ secrets.MY_SSH_KEY }}')")
	}

	userAndHost := strings.Split(host, "@")
	if len(userAndHost) != 2 {
		return nil, fmt.Errorf("le format de 'host' doit être 'user@hostname'")
	}
	user, hostname := userAndHost[0], userAndHost[1]

	port, _ := with["port"].(string)
	if port == "" {
		port = "22"
	}

	signer, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ATTENTION: Pas pour la production !
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(hostname, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	log.Printf("LOG | Executing SSH command on %s: %s", host, command)
	err = session.Run(command)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("failed to run ssh command: %w", err)
		}
	}

	result := map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	}

	if exitCode != 0 {
		return result, fmt.Errorf("ssh command failed with exit code %d", exitCode)
	}

	return result, nil
}
