package main

import (
	"bytes"
	"os/exec"
)

func executeShellCommand(command string) (stdout string, stderr string) {
	cmd := exec.Command("sh", "-c", command)

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()
	stdout = stdoutBuffer.String()
	stderr = stderrBuffer.String()

	if err != nil && stderr == "" {
		stderr = err.Error()
	}

	return stdout, stderr
}
