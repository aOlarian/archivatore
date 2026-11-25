/* MAP
 * §1 package & imports   L7-L23
 * §2 CLI entrypoints     L25-L120
 * §3 Compress dispatch   L122-L214
 */

// §1 package & imports
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// §2 CLI entrypoints
func buildFormatCatalog(tools ToolPaths) []FormatInfo {
	return []FormatInfo{
		{ID: "capsule-tar", Ext: ".tar", Label: "Capsule (.tar · instant)", CanCompress: true, CanExtract: true, SupportsLevel: false, Multithreaded: true, Engine: "go-native"},
		{ID: "capsule-zst", Ext: ".tar.zst", Label: "Capsule Fast (.tar.zst)", CanCompress: tools.Zstd != "" || tools.Tar != "", CanExtract: tools.Zstd != "" || tools.Tar != "", SupportsLevel: true, Multithreaded: true, Engine: "go-tar+zstd"},
		{ID: "zip", Ext: ".zip", Label: "ZIP (.zip)", CanCompress: true, CanExtract: true, SupportsLevel: true, Multithreaded: true, Engine: "go-native"},
		{ID: "7z", Ext: ".7z", Label: "7-Zip (.7z)", CanCompress: tools.SevenZip != "" || tools.Tar != "", CanExtract: tools.SevenZip != "" || tools.Tar != "", SupportsLevel: true, Multithreaded: tools.SevenZip != "", Engine: tools.SevenZip},
		{ID: "rar", Ext: ".rar", Label: "RAR5 (.rar)", CanCompress: tools.Rar != "", CanExtract: tools.Rar != "", SupportsLevel: true, Multithreaded: tools.Rar != "", Engine: tools.Rar},
		{ID: "tar-gz", Ext: ".tar.gz", Label: "TAR.GZ (.tar.gz)", CanCompress: true, CanExtract: true, SupportsLevel: true, Multithreaded: true, Engine: "go-pigz"},
		{ID: "gz", Ext: ".gz", Label: "GZIP (.gz · single/tar)", CanCompress: true, CanExtract: true, SupportsLevel: true, Multithreaded: true, Engine: "go-pigz"},
		{ID: "tar-xz", Ext: ".tar.xz", Label: "TAR.XZ (.tar.xz · max ratio)", CanCompress: tools.Tar != "", CanExtract: tools.Tar != "", SupportsLevel: true, Multithreaded: true, Engine: "bsdtar-xz"},
		{ID: "tar-bz2", Ext: ".tar.bz2", Label: "TAR.BZ2 (.tar.bz2)", CanCompress: tools.Tar != "", CanExtract: tools.Tar != "", SupportsLevel: true, Multithreaded: false, Engine: "bsdtar-bzip2"},
	}
}

func runProbeTools() int {
	tools := DetectTools()
	payload := struct {
		CPUCores int          `json:"cpuCores"`
		Tools    ToolPaths    `json:"tools"`
		Formats  []FormatInfo `json:"formats"`
	}{
		CPUCores: runtime.NumCPU(),
		Tools:    tools,
		Formats:  buildFormatCatalog(tools),
	}
	b, _ := json.Marshal(payload)
	fmt.Println(string(b))
	return 0
}

func formatExtension(format string, isDir bool) (string, error) {
	switch format {
	case "capsule-tar", "tar":
		return ".tar", nil
	case "capsule-zst", "tar-zst":
		if !isDir && format == "zst" {
			return ".zst", nil
		}
		return ".tar.zst", nil
	case "zst":
		if isDir {
			return ".tar.zst", nil
		}
		return ".zst", nil
	case "zip":
		return ".zip", nil
	case "7z":
		return ".7z", nil
	case "rar":
		return ".rar", nil
	case "tar-gz":
		return ".tar.gz", nil
	case "gz":
		if isDir {
			return ".tar.gz", nil
		}
		return ".gz", nil
	case "tar-xz":
		return ".tar.xz", nil
	case "tar-bz2":
		return ".tar.bz2", nil
	default:
		return "", errors.New("unsupported format: " + format)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: archivatore-engine <probe-tools|compress|extract> [flags]")
		os.Exit(2)
	}
	sub := os.Args[1]
	tools := DetectTools()

	switch sub {
	case "probe-tools":
		os.Exit(runProbeTools())
	case "compress":
		os.Exit(runCompressCmd(tools, os.Args[2:]))
	case "extract":
		os.Exit(runExtractCmd(tools, os.Args[2:]))
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", sub)
		os.Exit(2)
	}
}

// §3 Compress dispatch
func runCompressCmd(tools ToolPaths, args []string) int {
	fs := flag.NewFlagSet("compress", flag.ContinueOnError)
	src := fs.String("source", "", "source file or folder")
	outDir := fs.String("output-dir", "", "output directory")
	format := fs.String("format", "capsule-tar", "archive format")
	level := fs.Int("level", 5, "compression level 0..9")
	threads := fs.Int("threads", runtime.NumCPU(), "worker threads")
	conflict := fs.String("conflict", "rename", "rename|overwrite|skip")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	emit := NewEmitter()
	if *src == "" {
		emit.EmitError("missing --source")
		return 1
	}
	cleanSrc := filepath.Clean(*src)
	if *outDir == "" {
		*outDir = filepath.Dir(cleanSrc)
	}
	if *threads < 1 {
		*threads = 1
	}
	info, err := os.Stat(WinLongPath(cleanSrc))
	if err != nil {
		emit.EmitError(err.Error())
		return 1
	}
	isDir := info.IsDir()
	ext, err := formatExtension(strings.ToLower(*format), isDir)
	if err != nil {
		emit.EmitError(err.Error())
		return 1
	}
	baseName := filepath.Base(cleanSrc)
	if !isDir {
		// Keep full filename for single-file .gz/.zst, strip ext for container archives
		if ext != ".gz" && ext != ".zst" {
			baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
			if baseName == "" {
				baseName = filepath.Base(cleanSrc)
			}
		}
	}
	finalOut, skipped, err := ResolveOutputPath(baseName, *outDir, ext, *conflict)
	if err != nil {
		emit.EmitError(err.Error())
		return 1
	}
	if skipped {
		emit.EmitDone(finalOut, 0, 0, 0, true)
		return 0
	}

	entries, fileCount, totalBytes, err := ScanSource(cleanSrc)
	if err != nil {
		emit.EmitError(err.Error())
		return 1
	}
	emit.EmitScan(fileCount, totalBytes)

	// Temp file must keep the target extension (Rar.exe requires .rar suffix)
	stemOut := strings.TrimSuffix(finalOut, ext)
	tmpOut := stemOut + ".archivatore-tmp" + ext
	_ = os.Remove(WinLongPath(tmpOut))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = os.Remove(WinLongPath(tmpOut))
		os.Exit(130)
	}()

	var compErr error
	switch strings.ToLower(*format) {
	case "capsule-tar", "tar":
		compErr = CompressCapsuleTar(tmpOut, entries, *threads, fileCount, totalBytes, emit)
	case "capsule-zst", "tar-zst", "zst":
		if !isDir && ext == ".zst" {
			compErr = CompressSingleZst(tools, cleanSrc, tmpOut, *level, *threads, emit)
		} else {
			compErr = CompressTarZst(tools, cleanSrc, tmpOut, entries, *level, *threads, fileCount, totalBytes, emit)
		}
	case "zip":
		compErr = CompressZipMT(tmpOut, entries, *level, *threads, fileCount, totalBytes, emit)
	case "7z":
		compErr = CompressSevenZip(tools, cleanSrc, tmpOut, entries, *level, *threads, fileCount, totalBytes, emit)
	case "rar":
		compErr = CompressRar(tools, cleanSrc, tmpOut, *level, *threads, fileCount, totalBytes, emit)
	case "tar-gz", "gz":
		if !isDir && ext == ".gz" {
			_, compErr = CompressSingleGzipMT(cleanSrc, tmpOut, *level, *threads, emit)
		} else {
			compErr = CompressTarGzipMT(tmpOut, entries, *level, *threads, fileCount, totalBytes, emit)
		}
	case "tar-xz", "tar-bz2":
		compErr = CompressTarXzOrBz2(tools, strings.ToLower(*format), tmpOut, entries, *level, *threads, fileCount, totalBytes, emit)
	default:
		compErr = errors.New("unsupported format: " + *format)
	}

	if compErr != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		emit.EmitError(compErr.Error())
		return 1
	}
	if err := os.Rename(WinLongPath(tmpOut), WinLongPath(finalOut)); err != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		emit.EmitError(err.Error())
		return 1
	}
	outInfo, _ := os.Stat(WinLongPath(finalOut))
	var outBytes int64
	if outInfo != nil {
		outBytes = outInfo.Size()
	}
	emit.EmitDone(finalOut, totalBytes, outBytes, fileCount, false)
	return 0
}

func runExtractCmd(tools ToolPaths, args []string) int {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	src := fs.String("source", "", "archive path")
	outDir := fs.String("output-dir", "", "destination folder")
	mode := fs.String("mode", "smart", "smart|folder|direct")
	threads := fs.Int("threads", runtime.NumCPU(), "worker threads")
	conflict := fs.String("conflict", "rename", "rename|overwrite|skip")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	emit := NewEmitter()
	if *src == "" {
		emit.EmitError("missing --source")
		return 1
	}
	if *threads < 1 {
		*threads = 1
	}
	if err := ExecuteExtract(tools, filepath.Clean(*src), *outDir, *mode, *conflict, *threads, emit); err != nil {
		emit.EmitError(err.Error())
		return 1
	}
	return 0
}
