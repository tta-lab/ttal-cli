package taskwarrior

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func AddTask(description string, modifiers ...string) (string, error) {
	args := make([]string, 0, 2+len(modifiers))
	args = append(args, "add", description)
	args = append(args, modifiers...)

	out, err := runTaskWithVerbose("new-uuid", args...)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	uuid, err := parseCreatedUUID(out)
	if err != nil {
		return "", err
	}
	return uuid, nil
}

func AnnotateTask(uuid, text string) error {
	_, err := runTask(uuid, "annotate", text)
	if err != nil {
		return fmt.Errorf("failed to annotate task %s: %w", uuid, err)
	}
	return nil
}

func StartTask(uuid string) error {
	_, err := runTask(uuid, "start")
	if err != nil {
		return fmt.Errorf("failed to start task %s: %w", uuid, err)
	}
	return nil
}

func MarkDone(uuid string) error {
	_, err := runTask(uuid, "done")
	if err != nil {
		return fmt.Errorf("failed to mark task %s as done: %w", uuid, err)
	}
	return nil
}

func MarkDeleted(uuid string) error {
	_, err := runTaskWithInput("yes\n", uuid, "delete")
	if err != nil {
		return fmt.Errorf("failed to delete task %s: %w", uuid, err)
	}
	return nil
}

func MarkWaiting(uuid string) error {
	_, err := runTask(uuid, "modify", "status:waiting")
	if err != nil {
		return fmt.Errorf("failed to mark task %s as waiting: %w", uuid, err)
	}
	return nil
}

func parseCreatedUUID(output string) (string, error) {
	if m := uuidFindPattern.FindString(output); m != "" {
		return m, nil
	}
	return "", fmt.Errorf("could not find UUID in task output: %q", output)
}

func runTaskWithVerbose(verbose string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContextWithVerbose(ctx, verbose, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("taskwarrior timeout after %s", cmdTimeout)
	}
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%w: %s", err, errMsg)
	}
	return stdout.String(), nil
}

func runTask(args ...string) (string, error) {
	return runTaskWithInput("", args...)
}

func runTaskWithInput(input string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := CommandContext(ctx, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("taskwarrior timeout after %s", cmdTimeout)
	}
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%w: %s", err, errMsg)
	}
	return stdout.String(), nil
}

func parseFirstTask(output string) (*Task, error) {
	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return nil, fmt.Errorf("no task found")
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task found")
	}
	return &tasks[0], nil
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func capitalizeWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
