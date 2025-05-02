package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// MCPProcess manages a local MCP process using stdio for communication
type MCPProcess struct {
	cfg       config.StdioConfig
	backendID string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	mutex     sync.Mutex
	logger    *slog.Logger
	started   bool
}

// NewMCPProcess creates a new MCP process manager
func NewMCPProcess(backendID string, cfg config.StdioConfig, logger *slog.Logger) *MCPProcess {
	if logger == nil {
		logger = slog.Default()
	}

	return &MCPProcess{
		cfg:       cfg,
		backendID: backendID,
		logger:    logger.With("backend", backendID, "transport", "stdio"),
		started:   false,
	}
}

// Start starts the MCP process
func (p *MCPProcess) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.started {
		return nil
	}

	// Create command with arguments
	p.cmd = exec.Command(p.cfg.Command, p.cfg.Args...)

	// Set working directory if specified
	if p.cfg.WorkingDir != "" {
		p.cmd.Dir = p.cfg.WorkingDir
	}

	// Set environment variables
	env := os.Environ()
	for key, value := range p.cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	p.cmd.Env = env

	// Get stdin and stdout pipes
	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Redirect stderr to log
	stderrPipe, err := p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the process
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}
	p.started = true

	// Read stderr in a goroutine and log it
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			p.logger.Debug("MCP stderr", "message", scanner.Text())
		}
	}()

	p.logger.Info("Started MCP process",
		"command", p.cfg.Command,
		"args", p.cfg.Args,
		"pid", p.cmd.Process.Pid)

	return nil
}

// Stop stops the MCP process
func (p *MCPProcess) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.started || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Close stdin to signal the process to exit gracefully
	if p.stdin != nil {
		p.stdin.Close()
	}

	// Wait for the process to exit with a timeout
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		p.started = false
		if err != nil {
			p.logger.Error("MCP process exited with error", "error", err)
			return fmt.Errorf("process exited with error: %w", err)
		}
		p.logger.Info("Stopped MCP process gracefully")
		return nil
	case <-time.After(5 * time.Second):
		// If the process doesn't exit after timeout, kill it
		p.logger.Warn("Killing MCP process after timeout")
		if err := p.cmd.Process.Kill(); err != nil {
			p.logger.Error("Failed to kill MCP process", "error", err)
			return fmt.Errorf("failed to kill process: %w", err)
		}
		p.started = false
		return nil
	}
}

// Request sends a request to the MCP process and returns the response
func (p *MCPProcess) Request(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.started {
		return nil, fmt.Errorf("MCP process not started")
	}

	// Add request ID and timestamp
	input["request_id"] = fmt.Sprintf("req-%d", time.Now().UnixNano())
	input["timestamp"] = time.Now().Format(time.RFC3339)

	// Convert input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Write input to stdin
	_, err = p.stdin.Write(append(inputJSON, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Create a context with timeout if specified
	var cancel context.CancelFunc
	if p.cfg.StdioTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, p.cfg.StdioTimeout)
		defer cancel()
	}

	// Read response from stdout with context timeout
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(p.stdout)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			errorCh <- fmt.Errorf("failed to read from stdout: %w", err)
			return
		}
		responseCh <- line
	}()

	// Wait for either response or context timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errorCh:
		return nil, err
	case line := <-responseCh:
		// Parse the response
		var response map[string]interface{}
		if err := json.Unmarshal(line, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return response, nil
	}
}
