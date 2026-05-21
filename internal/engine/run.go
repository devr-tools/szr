package engine

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"
)

type runOptions struct {
	command            []string
	teeOnFailure       bool
	teeDir             string
	captureStdout      bool
	captureStderr      bool
	stdoutPreviewBytes int
	stderrPreviewBytes int
	reduceStdoutLive   bool
	reduceStderrLive   bool
	reduceStdoutLater  bool
	reduceStderrLater  bool
	reducer            StreamReducer
}

type runResult struct {
	stdout      string
	stderr      string
	stdoutBytes int
	stderrBytes int
	rawTokens   int
	exitCode    int
	teePath     string
}

type outputCollector struct {
	builder   strings.Builder
	bytes     int
	capture   bool
	limit     int
	truncated bool
	tokens    tokenCounter
}

func (c *outputCollector) Consume(chunk []byte) {
	c.bytes += len(chunk)
	c.tokens.Consume(chunk)
	if c.capture {
		_, _ = c.builder.Write(chunk)
		return
	}
	if c.limit <= 0 || c.truncated {
		return
	}
	remaining := c.limit - c.builder.Len()
	if remaining <= 0 {
		c.truncated = true
		return
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
		c.truncated = true
	}
	_, _ = c.builder.Write(chunk)
}

func (c *outputCollector) String() string {
	if !c.capture && c.limit <= 0 {
		return ""
	}
	return c.builder.String()
}

func (c *outputCollector) TokenCount() int {
	return c.tokens.Estimate()
}

type synchronizedReducer struct {
	mu    sync.Mutex
	inner StreamReducer
}

func newSynchronizedReducer(reducer StreamReducer) *synchronizedReducer {
	if reducer == nil {
		return nil
	}
	return &synchronizedReducer{inner: reducer}
}

func (r *synchronizedReducer) ConsumeStdout(chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inner.ConsumeStdout(chunk)
}

func (r *synchronizedReducer) ConsumeStderr(chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inner.ConsumeStderr(chunk)
}

func (r *synchronizedReducer) Result() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.Result()
}

func (r *synchronizedReducer) BytesParsed() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.BytesParsed()
}

func (r *synchronizedReducer) FallbackUsed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.FallbackUsed()
}

func runCommand(ctx context.Context, args []string, cwd string, options runOptions) (runResult, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return runResult{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return runResult{}, err
	}

	tee, teeErr := newTeeCapture(options.teeDir, options.command, options.teeOnFailure)
	if teeErr != nil {
		tee = nil
	}
	reducer := newSynchronizedReducer(options.reducer)

	if err := cmd.Start(); err != nil {
		if tee != nil {
			tee.Discard()
		}
		return runResult{}, err
	}

	var stdout outputCollector
	stdout.capture = options.captureStdout
	stdout.limit = options.stdoutPreviewBytes
	var stderr outputCollector
	stderr.capture = options.captureStderr
	stderr.limit = options.stderrPreviewBytes

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- copyStream(stdoutPipe, &stdout, tee, reducer, options.reduceStdoutLive, true)
	}()
	go func() {
		defer wg.Done()
		errCh <- copyStream(stderrPipe, &stderr, tee, reducer, options.reduceStderrLive, false)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	close(errCh)

	streamErr := firstStreamError(errCh)

	if reducer != nil {
		if options.reduceStdoutLater {
			reducer.ConsumeStdout([]byte(stdout.String()))
		}
		if options.reduceStderrLater {
			reducer.ConsumeStderr([]byte(stderr.String()))
		}
	}

	exitCode := 1
	if waitErr == nil {
		exitCode = 0
	} else if exitErr, ok := waitErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	teePath := ""
	if tee != nil {
		path, finalizeErr := tee.Finalize(exitCode)
		if finalizeErr == nil {
			teePath = path
		}
	}

	result := runResult{
		stdout:      stdout.String(),
		stderr:      stderr.String(),
		stdoutBytes: stdout.bytes,
		stderrBytes: stderr.bytes,
		rawTokens:   stdout.TokenCount() + stderr.TokenCount(),
		exitCode:    exitCode,
		teePath:     teePath,
	}

	if waitErr == nil {
		if streamErr != nil {
			return result, streamErr
		}
		return result, nil
	}
	if _, ok := waitErr.(*exec.ExitError); ok {
		if streamErr != nil {
			return result, streamErr
		}
		return result, nil
	}
	return result, waitErr
}

func copyStream(
	reader io.Reader,
	collector *outputCollector,
	tee *teeCapture,
	reducer *synchronizedReducer,
	reduceLive bool,
	isStdout bool,
) error {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			collector.Consume(chunk)
			if tee != nil {
				tee.Write(chunk)
			}
			if reducer != nil && reduceLive {
				if isStdout {
					reducer.ConsumeStdout(chunk)
				} else {
					reducer.ConsumeStderr(chunk)
				}
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return err
	}
}

type tokenCounter struct {
	pending []byte
	runes   int
}

func (c *tokenCounter) Consume(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	if len(c.pending) > 0 {
		combined := append(append([]byte{}, c.pending...), chunk...)
		c.pending = c.pending[:0]
		c.consumeComplete(combined)
		return
	}
	c.consumeComplete(chunk)
}

func (c *tokenCounter) consumeComplete(chunk []byte) {
	for len(chunk) > 0 {
		if utf8.FullRune(chunk) {
			_, size := utf8.DecodeRune(chunk)
			c.runes++
			chunk = chunk[size:]
			continue
		}
		c.pending = append(c.pending[:0], chunk...)
		return
	}
}

func (c *tokenCounter) Estimate() int {
	runes := c.runes
	if len(c.pending) > 0 {
		runes += utf8.RuneCount(c.pending)
	}
	if runes == 0 {
		return 0
	}
	if runes < 4 {
		return 1
	}
	return (runes + 3) / 4
}

func firstStreamError(errCh <-chan error) error {
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
