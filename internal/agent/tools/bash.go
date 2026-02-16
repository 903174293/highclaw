// Package tools implements agent tool handlers.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// BashInput represents the input to the bash tool.
type BashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // Timeout in seconds
}

// BashOutput represents the output from the bash tool.
type BashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// Bash executes a shell command and returns the output.
func Bash(ctx context.Context, inputJSON string) (string, error) {
	var input BashInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("invalid bash input: %w", err)
	}

	if input.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Default timeout: 30 seconds
	timeout := 30 * time.Second
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command via shell
	cmd := exec.CommandContext(execCtx, "sh", "-c", input.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	output := BashOutput{
		ExitCode: 0,
	}

	err := cmd.Run()
	output.Stdout = stdout.String()
	output.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output.ExitCode = exitErr.ExitCode()
		} else {
			output.Error = err.Error()
			output.ExitCode = -1
		}
	}

	// Format output as JSON
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshal output: %w", err)
	}

	return string(result), nil
}

// BashProcess manages a long-running bash process.
type BashProcess struct {
	ID      string
	Command string
	Cmd     *exec.Cmd
	Started time.Time
	Stdin   io.WriteCloser
}

var processes = make(map[string]*BashProcess)

// BashProcessInput represents input for bash_process tool.
type BashProcessInput struct {
	Action  string `json:"action"` // "start", "send", "kill"
	ID      string `json:"id,omitempty"`
	Command string `json:"command,omitempty"`
	Input   string `json:"input,omitempty"`
}

// BashProcessTool manages background bash processes.
func BashProcessTool(ctx context.Context, inputJSON string) (string, error) {
	var input BashProcessInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("invalid bash_process input: %w", err)
	}

	switch input.Action {
	case "start":
		return startProcess(input.Command)
	case "send":
		return sendToProcess(input.ID, input.Input)
	case "kill":
		return killProcess(input.ID)
	default:
		return "", fmt.Errorf("unknown action: %s", input.Action)
	}
}

func startProcess(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	id := fmt.Sprintf("proc-%d", time.Now().UnixNano())
	cmd := exec.Command("sh", "-c", command)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start process: %w", err)
	}

	processes[id] = &BashProcess{
		ID:      id,
		Command: command,
		Cmd:     cmd,
		Started: time.Now(),
		Stdin:   stdin,
	}

	return fmt.Sprintf(`{"id": "%s", "status": "started"}`, id), nil
}

func sendToProcess(id, input string) (string, error) {
	proc, ok := processes[id]
	if !ok {
		return "", fmt.Errorf("process not found: %s", id)
	}

	if proc.Stdin == nil {
		return "", fmt.Errorf("process stdin is not available: %s", id)
	}
	if _, err := io.WriteString(proc.Stdin, input); err != nil {
		return "", fmt.Errorf("write stdin: %w", err)
	}
	if input == "" || !strings.HasSuffix(input, "\n") {
		if _, err := io.WriteString(proc.Stdin, "\n"); err != nil {
			return "", fmt.Errorf("write newline: %w", err)
		}
	}

	return fmt.Sprintf(`{"id": "%s", "status": "sent"}`, id), nil
}

func killProcess(id string) (string, error) {
	proc, ok := processes[id]
	if !ok {
		return "", fmt.Errorf("process not found: %s", id)
	}

	if err := proc.Cmd.Process.Kill(); err != nil {
		return "", fmt.Errorf("kill process: %w", err)
	}
	if proc.Stdin != nil {
		_ = proc.Stdin.Close()
	}

	delete(processes, id)
	return fmt.Sprintf(`{"id": "%s", "status": "killed"}`, id), nil
}
