package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = stdout
	}()

	if err := printJSON(map[string]string{"foo": "bar"}); err != nil {
		t.Fatalf("print json: %v", err)
	}
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("\"foo\"")) {
		t.Fatalf("expected json output")
	}
}
