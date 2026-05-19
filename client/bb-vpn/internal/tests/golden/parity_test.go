// Package golden owns the parity-harness corpus for the
// bash↔Go render comparison.
//
// PR B (this PR): just the corpus + structural assertions. The actual
// pkg/render call + byte-equal assertion lands in PR C, when pkg/render
// exists. Keeping the corpus + a structural test in their own commit
// lets PR C focus purely on the Go renderer.
//
// Corpus shape (under expected/):
//
//	expected/<cell>/sing-box.json     present for every cell
//	expected/<cell>/xray.json         present iff proto != tcp-vision
//
// Cells:
//
//	proto-{all,tcp-vision,xhttp}_ts-{on,off}_corp-{on,off}_n{1,2,3}
//
// 3*2*2*3 = 36 cells. tcp-vision drops xray (12 cells xray-less),
// so 36 sing-box files + 24 xray files = 60 total.
//
// Regenerate (after an intentional render-config change):
//
//	bash internal/tests/golden/scripts/generate.sh
//
// The goldens encode bash render-config's current behavior including
// any latent bugs. Divergence is a parity break, NOT a bug fix.
package golden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	expectedDir = "expected"
	// Matrix is proto{3} * ts{2} * corp{2} * n{3} = 36 cells; each has
	// sing-box.json, and only non-tcp-vision (24 cells) also has
	// xray.json. So total .json files = 36 + 24 = 60.
	totalCells       = 36
	totalGoldenFiles = 60
	tcpVisionPrefix  = "proto-tcp-vision"
)

// allCellNames returns the names every cell directory must have.
func allCellNames() []string {
	var names []string
	for _, proto := range []string{"all", "tcp-vision", "xhttp"} {
		for _, ts := range []string{"off", "on"} {
			for _, corp := range []string{"off", "on"} {
				for _, n := range []int{1, 2, 3} {
					names = append(names, formatCell(proto, ts, corp, n))
				}
			}
		}
	}
	return names
}

func formatCell(proto, ts, corp string, n int) string {
	return "proto-" + proto + "_ts-" + ts + "_corp-" + corp + "_n" + strconv.Itoa(n)
}

func TestGolden_CellDirectoriesExist(t *testing.T) {
	want := allCellNames()
	for _, name := range want {
		dir := filepath.Join(expectedDir, name)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("cell %q: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("cell %q: not a directory", name)
		}
	}

	// No surprise cells either — guards against generator drift that
	// silently adds new combinations. Filter for directories so stray
	// files (.DS_Store, editor swap files) don't inflate the count;
	// name any offenders to make drift diagnosable.
	entries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Fatalf("read %s: %v", expectedDir, err)
	}
	var dirCount int
	var nonDirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirCount++
		} else {
			nonDirs = append(nonDirs, e.Name())
		}
	}
	if dirCount != totalCells {
		t.Errorf("cell count = %d, want %d (non-dir entries: %v)", dirCount, totalCells, nonDirs)
	}
}

func TestGolden_PerCellFilesPresentAndValid(t *testing.T) {
	for _, name := range allCellNames() {
		t.Run(name, func(t *testing.T) {
			singBoxPath := filepath.Join(expectedDir, name, "sing-box.json")
			mustBeValidJSON(t, singBoxPath)

			xrayPath := filepath.Join(expectedDir, name, "xray.json")
			isTCPVision := strings.HasPrefix(name, tcpVisionPrefix)
			_, err := os.Stat(xrayPath)
			switch {
			case isTCPVision && err == nil:
				t.Errorf("%s: xray.json must NOT exist for tcp-vision cells", xrayPath)
			case !isTCPVision && os.IsNotExist(err):
				t.Errorf("%s: xray.json required for non-tcp-vision cells", xrayPath)
			case !isTCPVision && err == nil:
				mustBeValidJSON(t, xrayPath)
			case err != nil && !os.IsNotExist(err):
				t.Errorf("%s: stat: %v", xrayPath, err)
			}
		})
	}
}

func TestGolden_TotalFileCount(t *testing.T) {
	var count int
	err := filepath.Walk(expectedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if count != totalGoldenFiles {
		t.Errorf("total .json files = %d, want %d", count, totalGoldenFiles)
	}
}

func mustBeValidJSON(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Errorf("%s: invalid JSON: %v", path, err)
		return
	}
	// A valid render must produce a JSON object — never null, array,
	// scalar. The bash script's `render_singbox`/`render_xray` emit
	// `{...}` trees. Catches accidental empty/truncated goldens.
	if _, ok := v.(map[string]any); !ok {
		t.Errorf("%s: top-level must be a JSON object, got %T", path, v)
	}
}
