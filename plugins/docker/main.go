package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	shared "github.com/orkestra-io/orkestra-shared"

	"github.com/hashicorp/go-plugin"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type DockerExecutor struct{}

func (e *DockerExecutor) GetCapabilities() ([]string, error) {
	return []string{"docker/run"}, nil
}

func (e *DockerExecutor) Execute(node shared.Node, ctx shared.ExecutionContext) (interface{}, error) {
	switch node.Uses {
	case "docker/run":
		return executeDockerRun(node.With)
	default:
		return nil, fmt.Errorf("unknown node type for docker plugin: %s", node.Uses)
	}
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"executor": &shared.NodeExecutorPlugin{Impl: &DockerExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

func executeDockerRun(with map[string]interface{}) (interface{}, error) {
	image, ok := with["image"].(string)
	if !ok || image == "" {
		return nil, fmt.Errorf("le paramètre 'image' est requis")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx := context.Background()
	reader, err := cli.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull docker image '%s': %w", image, err)
	}
	io.Copy(io.Discard, reader) // Attend la fin du téléchargement

	containerConfig := &container.Config{
		Image: image,
	}

	if cmd, ok := with["command"].(string); ok {
		containerConfig.Cmd = []string{cmd}
	}
	if args, ok := with["args"].([]interface{}); ok {
		for _, arg := range args {
			containerConfig.Cmd = append(containerConfig.Cmd, fmt.Sprintf("%v", arg))
		}
	}
	if env, ok := with["env"].(map[string]interface{}); ok {
		for key, val := range env {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%v", key, val))
		}
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	defer cli.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})

	if err := cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		out, err := cli.ContainerLogs(ctx, resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return nil, fmt.Errorf("failed to get container logs: %w", err)
		}

		buf := new(strings.Builder)

		_, err = io.Copy(buf, out)
		if err != nil {
			return nil, fmt.Errorf("failed to copy container logs: %w", err)
		}

		result := map[string]interface{}{
			"logs":         buf.String(),
			"exit_code":    status.StatusCode,
			"container_id": resp.ID,
		}

		if status.StatusCode != 0 {
			return result, fmt.Errorf("container exited with code %d", status.StatusCode)
		}
		return result, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("container run timed out")
	}

	return nil, fmt.Errorf("unknown error")
}
