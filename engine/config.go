/* MAP
 * §1 package & imports       L10-L24
 * §2 constants & ToolPaths   L26-L65
 * §3 DetectTools             L67-L98
 * §4 Path & conflict helpers L100-L185
 */

// §1 package & imports
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// §2 constants & ToolPaths
const (
	WriteBufSize       = 8 * 1024 * 1024
	StreamCopyBufSize  = 1 * 1024 * 1024
	GzipBlockSize      = 2 * 1024 * 1024
	PrefetchMaxFile    = 4 * 1024 * 1024
	ZipParallelMaxFile = 16 * 1024 * 1024
	ExtractSmallMax    = 2 * 1024 * 1024
	ProgressIntervalMs = 120
	LongPathThreshold  = 240
)

type ToolPaths struct {
	Tar      string `json:"tar"`
	Zstd     string `json:"zstd"`
	Rar      string `json:"rar"`
	SevenZip string `json:"sevenZip"`
}

type FormatInfo struct {
	ID            string `json:"id"`
	Ext           string `json:"ext"`
	Label         string `json:"label"`
	CanCompress   bool   `json:"canCompress"`
	CanExtract    bool   `json:"canExtract"`
	SupportsLevel bool   `json:"supportsLevel"`
	Multithreaded bool   `json:"multithreaded"`
	Engine        string `json:"engine"`
}

func firstExisting(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// §3 DetectTools
func DetectTools() ToolPaths {
	tarPath, _ := exec.LookPath("tar.exe")
	if tarPath == "" {
		tarPath = firstExisting(`C:\Windows\System32\tar.exe`)
	}
	zstdPath, _ := exec.LookPath("zstd.exe")
	if zstdPath == "" {
		var exeDir string
		if exe, err := os.Executable(); err == nil {
			exeDir = filepath.Dir(exe)
		}
		var localZstd string
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			localZstd = filepath.Join(localApp, "Programs", "zstd", "zstd.exe")
		}
		zstdPath = firstExisting(
			filepath.Join(exeDir, "zstd.exe"),
			`C:\Program Files\zstd\zstd.exe`,
			localZstd,
		)
	}
	rarPath := firstExisting(
		`C:\Program Files\WinRAR\Rar.exe`,
		`C:\Program Files (x86)\WinRAR\Rar.exe`,
	)
	if rarPath == "" {
		rarPath, _ = exec.LookPath("Rar.exe")
	}
	szPath := firstExisting(
		`C:\Program Files\7-Zip\7z.exe`,
		`C:\Program Files (x86)\7-Zip\7z.exe`,
	)
	if szPath == "" {
		szPath, _ = exec.LookPath("7z.exe")
	}
	return ToolPaths{
		Tar:      tarPath,
		Zstd:     zstdPath,
		Rar:      rarPath,
		SevenZip: szPath,
	}
}

// §4 Path & conflict helpers
func WinLongPath(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if strings.HasPrefix(abs, `\\?\`) {
		return abs
	}
	if len(abs) < LongPathThreshold {
		return abs
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(abs, `\\`)
	}
	return `\\?\` + abs
}

func CleanExtractPath(baseDir, entryName string) (string, error) {
	norm := strings.ReplaceAll(entryName, `\`, "/")
	norm = strings.TrimLeft(norm, "/")
	if norm == "" {
		return "", errors.New("empty archive entry name")
	}
	if len(norm) >= 2 && norm[1] == ':' {
		return "", errors.New("drive-prefixed path rejected: " + entryName)
	}
	cleanRel := filepath.Clean(filepath.FromSlash(norm))
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal rejected: " + entryName)
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(baseAbs, cleanRel)
	rel, err := filepath.Rel(baseAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("escape outside destination rejected: " + entryName)
	}
	return target, nil
}

func ResolveOutputPath(baseName, outDir, ext, conflict string) (finalPath string, skipped bool, err error) {
	if err := os.MkdirAll(WinLongPath(outDir), 0755); err != nil {
		return "", false, err
	}
	candidate := filepath.Join(outDir, baseName+ext)
	_, statErr := os.Stat(WinLongPath(candidate))
	if os.IsNotExist(statErr) {
		return candidate, false, nil
	}
	switch conflict {
	case "overwrite":
		return candidate, false, nil
	case "skip":
		return candidate, true, nil
	default: // "rename"
		for i := 1; i < 100000; i++ {
			next := filepath.Join(outDir, baseName+"-"+itoa(i)+ext)
			if _, e := os.Stat(WinLongPath(next)); os.IsNotExist(e) {
				return next, false, nil
			}
		}
		return "", false, errors.New("could not find free output filename")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
