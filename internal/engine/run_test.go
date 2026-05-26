package engine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCopyStreamDisablesTokenAccountingAfterReducerDone(t *testing.T) {
	reducer := newSynchronizedReducer(&stubDoneReducer{doneAfterStdoutChunks: 1})
	collector := &outputCollector{
		capture:       true,
		accountTokens: true,
	}

	err := copyStream(&chunkedReader{chunks: [][]byte{[]byte("alpha"), []byte("beta")}}, collector, nil, reducer, true, nil, true)
	if err != nil {
		t.Fatalf("copyStream error: %v", err)
	}

	if got := collector.String(); got != "alphabeta" {
		t.Fatalf("captured output = %q, want %q", got, "alphabeta")
	}
	if got, want := collector.bytes, len("alphabeta"); got != want {
		t.Fatalf("collector bytes = %d, want %d", got, want)
	}
	if got := collector.TokenCount(); got != estimateTokensForTest("alpha") {
		t.Fatalf("token count = %d, want %d", got, estimateTokensForTest("alpha"))
	}
	if !reducer.Done() {
		t.Fatal("expected reducer to report done")
	}
	if !collector.tokensDisabled {
		t.Fatal("expected token accounting to be disabled once reducer is done")
	}
}

func TestCollectCommandStreamsContinuesDrainingAfterReducerDone(t *testing.T) {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	defer stdoutReader.Close()
	defer stderrReader.Close()

	reducer := newSynchronizedReducer(&stubDoneReducer{doneAfterStdoutChunks: 1})
	stdoutCollector := &outputCollector{
		capture:       true,
		accountTokens: true,
	}
	stderrCollector := &outputCollector{
		capture:       true,
		accountTokens: true,
	}

	writeDone := make(chan error, 1)
	go func() {
		if _, err := stdoutWriter.Write([]byte("alpha")); err != nil {
			writeDone <- err
			return
		}
		if _, err := stdoutWriter.Write([]byte("beta")); err != nil {
			writeDone <- err
			return
		}
		writeDone <- stdoutWriter.Close()
	}()
	go func() {
		_ = stderrWriter.Close()
	}()

	err := collectCommandStreams(stdoutReader, stderrReader, stdoutCollector, stderrCollector, nil, reducer, runOptions{
		reduceStdoutLive: true,
	})
	if err != nil {
		t.Fatalf("collectCommandStreams error: %v", err)
	}

	select {
	case writeErr := <-writeDone:
		if writeErr != nil {
			t.Fatalf("stdout writer error: %v", writeErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stdout writer blocked after reducer completion")
	}

	if got := stdoutCollector.String(); got != "alphabeta" {
		t.Fatalf("captured stdout = %q, want %q", got, "alphabeta")
	}
	if got := stdoutCollector.TokenCount(); got != estimateTokensForTest("alpha") {
		t.Fatalf("token count = %d, want %d", got, estimateTokensForTest("alpha"))
	}
}

func TestCopyStreamPreservesTeeAfterReducerDone(t *testing.T) {
	dir := t.TempDir()
	tee, err := newTeeCapture(dir, []string{"test"}, true)
	if err != nil {
		t.Fatalf("newTeeCapture error: %v", err)
	}

	reducer := newSynchronizedReducer(&stubDoneReducer{doneAfterStdoutChunks: 1})
	collector := &outputCollector{
		capture:       true,
		accountTokens: true,
	}

	err = copyStream(strings.NewReader("alphabeta"), collector, tee, reducer, true, nil, true)
	if err != nil {
		t.Fatalf("copyStream error: %v", err)
	}

	teePath, err := tee.Finalize(1)
	if err != nil {
		t.Fatalf("tee finalize error: %v", err)
	}
	if teePath == "" {
		t.Fatal("expected tee file path for failing command")
	}

	data, err := os.ReadFile(filepath.Clean(teePath))
	if err != nil {
		t.Fatalf("read tee file: %v", err)
	}
	if got := string(data); got != "alphabeta" {
		t.Fatalf("tee content = %q, want %q", got, "alphabeta")
	}
}

type stubDoneReducer struct {
	stdoutChunks          int
	doneAfterStdoutChunks int
	stdout                strings.Builder
}

func (r *stubDoneReducer) ConsumeStdout(chunk []byte) {
	r.stdoutChunks++
	_, _ = r.stdout.Write(chunk)
}

func (r *stubDoneReducer) ConsumeStderr([]byte) {}

func (r *stubDoneReducer) Result() string {
	return r.stdout.String()
}

func (r *stubDoneReducer) BytesParsed() int {
	return len(r.stdout.String())
}

func (r *stubDoneReducer) FallbackUsed() bool {
	return false
}

func (r *stubDoneReducer) Done() bool {
	return r.doneAfterStdoutChunks > 0 && r.stdoutChunks >= r.doneAfterStdoutChunks
}

func estimateTokensForTest(text string) int {
	var counter tokenCounter
	counter.Consume([]byte(text))
	return counter.Estimate()
}

type chunkedReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	copy(p, chunk)
	return len(chunk), nil
}
