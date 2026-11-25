/* MAP
 * §1 package & imports        L8-L20
 * §2 Test fixture helpers     L22-L67
 * §3 Round-trip format tests  L69-L176
 * §4 Conflict, smart & safety L178-L250
 */

// §1 package & imports
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §2 Test fixture helpers
func createSampleTree(t *testing.T, root string, count int) string {
	t.Helper()
	folder := filepath.Join(root, "Date_România_ăâîșț")
	sub := filepath.Join(folder, "Subfolder_Școală")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("Archivatore fast multithreaded payload 0123456789 "), 64)
	for i := 0; i < count; i++ {
		dir := folder
		if i%2 == 1 {
			dir = sub
		}
		name := fmt.Sprintf("fișier_%03d_ăâîșț.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return folder
}

func verifySampleTree(t *testing.T, folder string, count int) {
	t.Helper()
	expected := bytes.Repeat([]byte("Archivatore fast multithreaded payload 0123456789 "), 64)
	for i := 0; i < count; i++ {
		dir := folder
		if i%2 == 1 {
			dir = filepath.Join(folder, "Subfolder_Școală")
		}
		name := fmt.Sprintf("fișier_%03d_ăâîșț.txt", i)
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("missing extracted file %s: %v", name, err)
		}
		if !bytes.Equal(got, expected) {
			t.Fatalf("content mismatch in %s", name)
		}
	}
}

// §3 Round-trip format tests
func TestAllFolderFormatsRoundTrip(t *testing.T) {
	tools := DetectTools()
	tmp := t.TempDir()
	src := createSampleTree(t, tmp, 320)
	_, files, totalBytes, err := ScanSource(src)
	if err != nil {
		t.Fatalf("ScanSource failed: %v", err)
	}
	if files != 320 || totalBytes == 0 {
		t.Fatalf("unexpected scan counts: files=%d bytes=%d", files, totalBytes)
	}

	formats := []struct {
		id    string
		ext   string
		level int
		need  bool
	}{
		{"capsule-tar", ".tar", 0, true},
		{"zip", ".zip", 0, true},
		{"zip", ".zip", 6, true},
		{"tar-gz", ".tar.gz", 5, true},
		{"capsule-zst", ".tar.zst", 3, tools.Zstd != "" || tools.Tar != ""},
		{"7z", ".7z", 5, tools.SevenZip != "" || tools.Tar != ""},
		{"rar", ".rar", 3, tools.Rar != ""},
		{"tar-xz", ".tar.xz", 3, tools.Tar != ""},
		{"tar-bz2", ".tar.bz2", 5, tools.Tar != ""},
	}

	for idx, tc := range formats {
		if !tc.need {
			t.Logf("skipping %s (tool not installed)", tc.id)
			continue
		}
		outDir := filepath.Join(tmp, fmt.Sprintf("out_%d_%s", idx, tc.id))
		_ = os.MkdirAll(outDir, 0755)
		code := runCompressCmd(tools, []string{
			"--source", src,
			"--output-dir", outDir,
			"--format", tc.id,
			"--level", fmt.Sprintf("%d", tc.level),
			"--threads", "8",
			"--conflict", "rename",
		})
		if code != 0 {
			t.Fatalf("compress %s (lvl %d) failed with exit %d", tc.id, tc.level, code)
		}
		archivePath := filepath.Join(outDir, filepath.Base(src)+tc.ext)
		if _, err := os.Stat(archivePath); err != nil {
			t.Fatalf("expected archive %s missing: %v", archivePath, err)
		}

		extDir := filepath.Join(tmp, fmt.Sprintf("ext_%d_%s", idx, tc.id))
		_ = os.MkdirAll(extDir, 0755)
		eCode := runExtractCmd(tools, []string{
			"--source", archivePath,
			"--output-dir", extDir,
			"--mode", "smart",
			"--threads", "8",
			"--conflict", "rename",
		})
		if eCode != 0 {
			t.Fatalf("extract %s failed with exit %d", tc.id, eCode)
		}
		verifySampleTree(t, filepath.Join(extDir, filepath.Base(src)), 320)
	}
}

func TestSingleFileGzAndZst(t *testing.T) {
	tools := DetectTools()
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "raport_mare.log")
	data := bytes.Repeat([]byte("line-entry-1234567890\n"), 20000)
	if err := os.WriteFile(srcFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	for _, fmtName := range []string{"gz", "zst"} {
		if fmtName == "zst" && tools.Zstd == "" {
			continue
		}
		outDir := filepath.Join(tmp, "out_"+fmtName)
		_ = os.MkdirAll(outDir, 0755)
		if code := runCompressCmd(tools, []string{
			"--source", srcFile,
			"--output-dir", outDir,
			"--format", fmtName,
			"--level", "5",
			"--threads", "8",
		}); code != 0 {
			t.Fatalf("compress single %s failed", fmtName)
		}
		arch := filepath.Join(outDir, "raport_mare.log."+fmtName)
		extDir := filepath.Join(tmp, "ext_"+fmtName)
		_ = os.MkdirAll(extDir, 0755)
		if code := runExtractCmd(tools, []string{
			"--source", arch,
			"--output-dir", extDir,
			"--threads", "8",
		}); code != 0 {
			t.Fatalf("extract single %s failed", fmtName)
		}
		got, err := os.ReadFile(filepath.Join(extDir, "raport_mare.log"))
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("single %s mismatch err=%v", fmtName, err)
		}
	}
}

// §4 Conflict, smart & safety
func TestConflictPoliciesAndSmartExtract(t *testing.T) {
	tools := DetectTools()
	tmp := t.TempDir()
	src := createSampleTree(t, tmp, 4)

	// 1) rename creates -1
	for i := 0; i < 2; i++ {
		if code := runCompressCmd(tools, []string{
			"--source", src, "--output-dir", tmp, "--format", "capsule-tar", "--conflict", "rename", "--threads", "8",
		}); code != 0 {
			t.Fatalf("compress rename #%d failed", i)
		}
	}
	firstArch := filepath.Join(tmp, filepath.Base(src)+".tar")
	secondArch := filepath.Join(tmp, filepath.Base(src)+"-1.tar")
	if _, err := os.Stat(secondArch); err != nil {
		t.Fatalf("expected -1 archive: %v", err)
	}

	// 2) skip keeps existing untouched
	if err := os.WriteFile(firstArch, []byte("sentinel"), 0644); err != nil {
		t.Fatal(err)
	}
	if code := runCompressCmd(tools, []string{
		"--source", src, "--output-dir", tmp, "--format", "capsule-tar", "--conflict", "skip", "--threads", "8",
	}); code != 0 {
		t.Fatalf("compress skip failed")
	}
	b, _ := os.ReadFile(firstArch)
	if string(b) != "sentinel" {
		t.Fatalf("skip modified existing archive")
	}

	// 3) smart extract on existing folder with rename creates Folder-1
	if code := runExtractCmd(tools, []string{
		"--source", secondArch, "--output-dir", tmp, "--mode", "smart", "--conflict", "rename", "--threads", "8",
	}); code != 0 {
		t.Fatalf("smart extract rename failed")
	}
	verifySampleTree(t, filepath.Join(tmp, filepath.Base(src)+"-1"), 4)
}

func TestZipSlipRejected(t *testing.T) {
	tmp := t.TempDir()
	badZip := filepath.Join(tmp, "evil.zip")
	f, err := os.Create(badZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("nope"))
	_ = zw.Close()
	_ = f.Close()

	dest := filepath.Join(tmp, "safe")
	_, _, exErr := ExtractZipMT(badZip, dest, 4, nil)
	if exErr == nil || !strings.Contains(exErr.Error(), "rejected") {
		t.Fatalf("expected zip-slip rejection, got: %v", exErr)
	}
}
