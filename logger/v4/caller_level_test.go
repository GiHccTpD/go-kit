package v4

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestErrorOnlyCallerRespectsLevelAndDisableCaller(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		for _, disable := range []bool{false, true} {
			t.Run(format, func(t *testing.T) {
				file := filepath.Join(t.TempDir(), "caller.log")
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				relative, err := filepath.Rel(cwd, file)
				if err != nil {
					t.Fatal(err)
				}
				logger := NewLogger(&Options{
					Level: "debug", Format: format, OutputPaths: []string{relative},
					DisableCaller: disable, DisableStacktrace: true, ErrorOnlyCaller: true,
				})
				logger.C(nil).Debugw("debug entry")
				logger.C(nil).Infow("info entry")
				logger.C(nil).Warnw("warn entry")
				logger.C(nil).Errorw("error entry")
				logger.Sync()
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
				if len(lines) != 4 {
					t.Fatalf("got %d entries, want 4", len(lines))
				}
				for i, line := range lines {
					if hasCaller := bytes.Contains(line, []byte("caller_level_test.go:")); hasCaller != (i == 3 && !disable) {
						t.Fatalf("unexpected caller: %s", line)
					}
				}
			})
		}
	}
}
