// Package appserver contains the small JSONL client GARY needs to host a
// durable Luna Yardmaster conversation.
package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"
)

// ToolHandler is called for every dynamic tool invocation sent by app-server.
// The returned text is given back to Luna as the tool result.
type ToolHandler func(name string, arguments json.RawMessage) (string, error)

// Client is a single app-server stdio connection. A connection is deliberately
// used for one turn at a time so request/notification ordering is inspectable.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	seq     uint64
	log     io.Writer
}

type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// Start launches the locally installed Codex App Server over stdio.
func Start(ctx context.Context, codexBinary string, log io.Writer) (*Client, error) {
	if codexBinary == "" {
		codexBinary = "codex"
	}
	cmd := exec.CommandContext(ctx, codexBinary, "app-server", "--listen", "stdio://")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("app-server stdout: %w", err)
	}
	if log == nil {
		log = io.Discard
	}
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	return &Client{cmd: cmd, stdin: stdin, scanner: scanner, log: log}, nil
}

// Close closes the client-side pipe and waits for the child process when it
// has already exited. Killing the parent leaves Codex's persisted thread
// untouched; a later invocation can resume it by id.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	_ = c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) send(method string, params any) (json.RawMessage, error) {
	id := atomic.AddUint64(&c.seq, 1)
	request := map[string]any{"method": method, "id": id, "params": params}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", method, err)
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}
	wantID, _ := json.Marshal(id)
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		var msg message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("decode app-server response: %w", err)
		}
		if string(msg.ID) != string(wantID) {
			c.writeLog(line)
			continue
		}
		if len(msg.Error) > 0 && string(msg.Error) != "null" {
			return nil, fmt.Errorf("app-server %s error: %s", method, msg.Error)
		}
		return msg.Result, nil
	}
	if err := c.scanner.Err(); err != nil {
		return nil, fmt.Errorf("read app-server response: %w", err)
	}
	return nil, fmt.Errorf("app-server closed while waiting for %s", method)
}

func (c *Client) notify(method string, params any) error {
	data, err := json.Marshal(map[string]any{"method": method, "params": params})
	if err != nil {
		return fmt.Errorf("encode notification %s: %w", method, err)
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return fmt.Errorf("send notification %s: %w", method, err)
	}
	return nil
}

func (c *Client) response(id json.RawMessage, result any) error {
	data, err := json.Marshal(map[string]any{"id": json.RawMessage(id), "result": result})
	if err != nil {
		return fmt.Errorf("encode server response: %w", err)
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return fmt.Errorf("send server response: %w", err)
	}
	return nil
}

func (c *Client) writeLog(line []byte) {
	_, _ = fmt.Fprintf(c.log, "%s\n", line)
}

// Initialize performs the required app-server handshake and opts into the
// experimental dynamic-tools surface used by the local yard boundary.
func (c *Client) Initialize() error {
	params := map[string]any{
		"clientInfo": map[string]any{
			"name":    "gary_luna_yardmaster",
			"title":   "GARY Luna Yardmaster",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{"experimentalApi": true},
	}
	if _, err := c.send("initialize", params); err != nil {
		return err
	}
	return c.notify("initialized", map[string]any{})
}

// StartThread creates or resumes a durable thread. For a resumed thread the
// dynamic tool declarations are supplied again, making restarts independent
// of in-memory client state while preserving the stored conversation.
func (c *Client) StartThread(threadID, model, cwd string, tools []map[string]any) (string, error) {
	var method string
	params := map[string]any{
		"model":          model,
		"cwd":            cwd,
		"approvalPolicy": "never",
		"sandbox":        "read-only",
		"serviceName":    "gary_luna_yardmaster",
		"dynamicTools":   tools,
	}
	if threadID == "" {
		method = "thread/start"
	} else {
		method = "thread/resume"
		params = map[string]any{
			"threadId":       threadID,
			"model":          model,
			"cwd":            cwd,
			"approvalPolicy": "never",
			"sandbox":        "read-only",
			"dynamicTools":   tools,
		}
	}
	result, err := c.send(method, params)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil || envelope.Thread.ID == "" {
		return "", fmt.Errorf("app-server %s returned no thread id: %s", method, result)
	}
	return envelope.Thread.ID, nil
}

// Turn starts one turn and consumes its streamed events until turn/completed.
// Dynamic tools and server requests are handled on the same ordered stream.
func (c *Client) Turn(ctx context.Context, threadID, input string, handler ToolHandler, progress io.Writer) (string, error) {
	params := map[string]any{
		"threadId": threadID,
		"input":    []map[string]any{{"type": "text", "text": input}},
	}
	data, err := json.Marshal(map[string]any{"method": "turn/start", "id": atomic.AddUint64(&c.seq, 1), "params": params})
	if err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return "", fmt.Errorf("send turn/start: %w", err)
	}
	var finalStatus string
	for {
		select {
		case <-ctx.Done():
			return finalStatus, ctx.Err()
		default:
		}
		if !c.scanner.Scan() {
			if err := c.scanner.Err(); err != nil {
				return finalStatus, fmt.Errorf("read turn events: %w", err)
			}
			return finalStatus, fmt.Errorf("app-server closed during turn")
		}
		line := append([]byte(nil), c.scanner.Bytes()...)
		var msg message
		if err := json.Unmarshal(line, &msg); err != nil {
			return finalStatus, fmt.Errorf("decode turn event: %w", err)
		}
		c.writeLog(line)
		if progress != nil && msg.Method == "item/agentMessage/delta" {
			var p struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(msg.Params, &p) == nil {
				_, _ = io.WriteString(progress, p.Delta)
			}
		}
		switch msg.Method {
		case "item/tool/call":
			var call struct {
				Namespace string          `json:"namespace"`
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
				CallID    string          `json:"callId"`
			}
			if err := json.Unmarshal(msg.Params, &call); err != nil {
				return finalStatus, fmt.Errorf("decode dynamic tool call: %w", err)
			}
			if handler == nil {
				return finalStatus, fmt.Errorf("dynamic tool %q requested without handler", call.Tool)
			}
			result, callErr := handler(call.Tool, call.Arguments)
			if callErr != nil {
				result = "tool error: " + callErr.Error()
			}
			if err := c.response(msg.ID, map[string]any{
				"success": true,
				"contentItems": []map[string]string{{
					"type": "inputText",
					"text": result,
				}},
			}); err != nil {
				return finalStatus, err
			}
		case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
			// The Yardmaster is deliberately read-only and has all local yard
			// operations exposed as dynamic tools. Decline any attempt to edit
			// through the built-in environment.
			if err := c.response(msg.ID, map[string]string{"decision": "decline"}); err != nil {
				return finalStatus, err
			}
		case "tool/requestUserInput":
			// Persisted human boundaries are represented by the yard tool. If
			// the model uses Codex's own input tool, answer empty and let the
			// resulting turn evidence explain the interruption.
			if err := c.response(msg.ID, map[string]any{"answers": map[string]any{}}); err != nil {
				return finalStatus, err
			}
		case "turn/completed":
			var p struct {
				Turn struct {
					Status string `json:"status"`
				} `json:"turn"`
			}
			if json.Unmarshal(msg.Params, &p) == nil {
				finalStatus = p.Turn.Status
			}
			return finalStatus, nil
		case "error":
			return finalStatus, fmt.Errorf("app-server turn error: %s", msg.Params)
		}
	}
}

// DynamicTools returns the deliberately small GARY boundary advertised to
// Luna. JSON Schema is inline so the prototype has no MCP/config dependency.
func DynamicTools() []map[string]any {
	return []map[string]any{
		functionTool("gary_spawn_worker", "Start the single bounded Codex worker in a fresh local git worktree. Never call this more than once for a job.", map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"task":    map[string]any{"type": "string"},
				"profile": map[string]any{"type": "string", "description": "A supported Codex model id; use gpt-5.6-luna for the prototype."},
			},
			"required": []string{"task", "profile"},
		}),
		functionTool("gary_worker_status", "Reconcile the worker process and real git worktree state after any interruption.", objectSchema("worker_id")),
		functionTool("gary_run_acceptance", "Run the job's declared deterministic acceptance commands in the worker worktree.", objectSchema("worker_id")),
		functionTool("gary_inspect_diff", "Inspect the actual worker diff and commit state; do not rely on worker prose.", objectSchema("worker_id")),
		functionTool("gary_request_matthew", "Persist a precise human-only blocker and stop the logical job until Matthew supplies the requested action.", map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"summary":        map[string]any{"type": "string"},
				"needed":         map[string]any{"type": "string"},
				"evidence":       map[string]any{"type": "string"},
				"unblock_action": map[string]any{"type": "string"},
			},
			"required": []string{"summary", "needed", "evidence", "unblock_action"},
		}),
	}
}

func functionTool(name, description string, inputSchema map[string]any) map[string]any {
	return map[string]any{"type": "function", "name": name, "description": description, "inputSchema": inputSchema}
}

func objectSchema(required string) map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"worker_id": map[string]any{"type": "string"}},
		"required":   []string{required},
	}
}
