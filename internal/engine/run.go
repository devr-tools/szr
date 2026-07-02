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
	stopStdoutEarly    bool
	stopStderrEarly    bool
	stdoutBypassBytes  int
	stdoutBypassTokens int
	stderrBypassBytes  int
	stderrBypassTokens int
	stdoutPreviewBytes int
	stderrPreviewBytes int
	reduceStdoutLive   bool
	reduceStderrLive   bool
	reduceStdoutLater  bool
	reduceStderrLater  bool
	reducer            StreamReducer
	onPreview          func(text string, bytesParsed int, done bool)
}

type runResult struct {
	stdout           string
	stderr           string
	stdoutBytes      int
	stderrBytes      int
	rawTokens        int
	exitCode         int
	teePath          string
	captureTruncated bool
}

type outputCollector struct {
	builder        strings.Builder
	bytes          int
	capture        bool
	limit          int
	truncated      bool
	accountTokens  bool
	tokensDisabled bool
	tokens         tokenCounter
}

func (c *outputCollector) Consume(chunk []byte) {
	c.bytes += len(chunk)
	if c.accountTokens {
		c.tokens.Consume(chunk)
	}
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
	if !c.capture && c.limit <= 0 && c.builder.Len() == 0 {
		return ""
	}
	return c.builder.String()
}

func (c *outputCollector) DisableTokenAccounting() {
	c.accountTokens = false
	c.tokensDisabled = true
}

func (c *outputCollector) DisableBuffering() {
	c.capture = false
	c.limit = 0
	c.truncated = true
}

func (c *outputCollector) TokenCount() int {
	return c.tokens.Estimate()
}

type synchronizedReducer struct {
	mu            sync.Mutex
	inner         StreamReducer
	done          bool
	lastPublished string
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
	if r.done {
		return
	}
	r.inner.ConsumeStdout(chunk)
	r.updateDoneLocked()
}

func (r *synchronizedReducer) ConsumeStderr(chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done {
		return
	}
	r.inner.ConsumeStderr(chunk)
	r.updateDoneLocked()
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

func (r *synchronizedReducer) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

func (r *synchronizedReducer) SafeToStopBuffering() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done {
		return false
	}
	if r.inner.FallbackUsed() {
		return false
	}
	return strings.TrimSpace(r.inner.Result()) != ""
}

func (r *synchronizedReducer) publishPreview(cb func(text string, bytesParsed int, done bool)) {
	if cb == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previewer, ok := r.inner.(StreamReducerPreview)
	if !ok {
		return
	}
	preview := previewer.Preview()
	if preview == "" || preview == r.lastPublished {
		return
	}
	r.lastPublished = preview
	cb(preview, r.inner.BytesParsed(), r.done)
}

func (r *synchronizedReducer) updateDoneLocked() {
	done, ok := r.inner.(StreamReducerDone)
	if ok && done.Done() {
		r.done = true
	}
}

func runCommand(ctx context.Context, args []string, cwd string, options runOptions) (runResult, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd

	stdoutPipe, stderrPipe, err := commandPipes(cmd)
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
	stdout.accountTokens = true
	var stderr outputCollector
	stderr.capture = options.captureStderr
	stderr.limit = options.stderrPreviewBytes
	stderr.accountTokens = true

	streamErr := collectCommandStreams(stdoutPipe, stderrPipe, &stdout, &stderr, tee, reducer, options)
	waitErr := cmd.Wait()

	if reducer != nil {
		if options.reduceStdoutLater {
			reducer.ConsumeStdout([]byte(stdout.String()))
			reducer.publishPreview(options.onPreview)
		}
		if options.reduceStderrLater {
			reducer.ConsumeStderr([]byte(stderr.String()))
			reducer.publishPreview(options.onPreview)
		}
	}

	exitCode := exitCodeForWaitError(waitErr)
	teePath := finalizeTeeCapture(tee, exitCode)

	result := runResult{
		stdout:           stdout.String(),
		stderr:           stderr.String(),
		stdoutBytes:      stdout.bytes,
		stderrBytes:      stderr.bytes,
		rawTokens:        stdout.TokenCount() + stderr.TokenCount(),
		exitCode:         exitCode,
		teePath:          teePath,
		captureTruncated: stdout.truncated || stderr.truncated,
	}
	return finalizeRunCommandResult(result, streamErr, waitErr)
}

func commandPipes(cmd *exec.Cmd) (io.ReadCloser, io.ReadCloser, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	return stdoutPipe, stderrPipe, nil
}

func collectCommandStreams(
	stdoutPipe io.Reader,
	stderrPipe io.Reader,
	stdout *outputCollector,
	stderr *outputCollector,
	tee *teeCapture,
	reducer *synchronizedReducer,
	options runOptions,
) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- copyStream(stdoutPipe, stdout, tee, reducer, streamCopyOptions{
			reduceLive:      options.reduceStdoutLive,
			onPreview:       options.onPreview,
			isStdout:        true,
			stopBufferEarly: options.stopStdoutEarly,
			bypassBytes:     options.stdoutBypassBytes,
			bypassTokens:    options.stdoutBypassTokens,
		})
	}()
	go func() {
		defer wg.Done()
		errCh <- copyStream(stderrPipe, stderr, tee, reducer, streamCopyOptions{
			reduceLive:      options.reduceStderrLive,
			onPreview:       options.onPreview,
			isStdout:        false,
			stopBufferEarly: options.stopStderrEarly,
			bypassBytes:     options.stderrBypassBytes,
			bypassTokens:    options.stderrBypassTokens,
		})
	}()
	wg.Wait()
	close(errCh)
	return firstStreamError(errCh)
}

func exitCodeForWaitError(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

func finalizeTeeCapture(tee *teeCapture, exitCode int) string {
	if tee == nil {
		return ""
	}
	path, finalizeErr := tee.Finalize(exitCode)
	if finalizeErr != nil {
		return ""
	}
	return path
}

func finalizeRunCommandResult(result runResult, streamErr error, waitErr error) (runResult, error) {
	if waitErr == nil {
		return result, streamErr
	}
	if _, ok := waitErr.(*exec.ExitError); ok {
		return result, streamErr
	}
	return result, waitErr
}

func copyStream(
	reader io.Reader,
	collector *outputCollector,
	tee *teeCapture,
	reducer *synchronizedReducer,
	options streamCopyOptions,
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
			if reducer != nil && options.reduceLive && !reducer.Done() {
				if options.isStdout {
					reducer.ConsumeStdout(chunk)
				} else {
					reducer.ConsumeStderr(chunk)
				}
				reducer.publishPreview(options.onPreview)
			}
			if shouldDisableCollectorTokens(collector, reducer, options) {
				collector.DisableTokenAccounting()
			}
			if shouldStopCollectorAfterChunk(collector, reducer, options) {
				collector.DisableTokenAccounting()
				collector.DisableBuffering()
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

type streamCopyOptions struct {
	reduceLive      bool
	onPreview       func(text string, bytesParsed int, done bool)
	isStdout        bool
	stopBufferEarly bool
	bypassBytes     int
	bypassTokens    int
}

func shouldStopCollectorAfterChunk(collector *outputCollector, reducer *synchronizedReducer, options streamCopyOptions) bool {
	if reducer == nil || !options.reduceLive || !options.stopBufferEarly {
		return false
	}
	if !reducer.SafeToStopBuffering() {
		return false
	}
	if options.bypassBytes > 0 && collector.bytes > options.bypassBytes {
		return true
	}
	if options.bypassTokens > 0 && collector.TokenCount() > options.bypassTokens {
		return true
	}
	return false
}

func shouldDisableCollectorTokens(collector *outputCollector, reducer *synchronizedReducer, options streamCopyOptions) bool {
	if reducer == nil || !options.reduceLive || !collector.accountTokens || !reducer.Done() {
		return false
	}
	if !options.stopBufferEarly {
		return true
	}
	return shouldStopCollectorAfterChunk(collector, reducer, options)
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
