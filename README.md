# Archivatore

## Enterprise edition

We build and deploy a paid Enterprise edition to order. Examples of what we can develop and extend for your organization are listed below.

- Centralized batch archiving policies, scheduled pipelines and automated cloud or NAS handoff across workstations and servers.
- Managed encryption (AES-256 / GPG), compliance checksum manifests and post-archive integrity verification.
- Silent MSI/GPO deployment, role-based presets and audit logs for IT teams.

For an Enterprise edition or custom development, contact us at [verobots.eu](https://verobots.eu/) or [olarian.com](https://olarian.com/).

Archivatore is a multi-threaded Windows desktop archiver and folder encapsulator built with a pure Go standard-library engine and an Electron UI (zero runtime npm dependencies). It turns folders with huge file trees into single files for fast cloud or Google Drive uploads, compresses and extracts across nine archive formats, and keeps a non-blocking parallel queue so you can keep browsing and queuing new batches while jobs run.

## Key capabilities

- **Fast encapsulation & 9 formats:** `.tar` capsule (0% store for maximum speed), `.tar.zst` / `.zst`, `.zip` (multi-core Store & Deflate), `.7z`, `.rar` (RAR5 via WinRAR `Rar.exe` when installed), `.tar.gz`, `.gz` (multi-member parallel RFC 1952), `.tar.xz`, and `.tar.bz2`, with adjustable compression levels (`0` Store to `9` Ultra).
- **Commander multi-select explorer:** browse drives and folders with checkboxes, *Check all*, *All folders*, *All archives*, *Compress All* (creates one archive per selected folder), and *Extract All* (extracts selected archives in place).
- **Non-blocking parallel queue:** switch between **Auto**, **Parallel** (up to 32 concurrent jobs, default 8), and **Sequential** (all configured CPU cores on one job, minimum 8 recommended) without blocking navigation.
- **Safety & reliability:** smart in-place extraction (single root directly beside the archive, multi-root into a subfolder), conflict policies (`rename`, `overwrite`, `skip`), atomic temp-file writes, Zip-Slip path traversal rejection, Windows long-path (`\\?\`) support, and Light/Dark/System themes.

## Install and quick start

1. Download `archivatore-install.exe` from [Releases](../../releases) and run it (the Windows installer is unsigned).
2. Browse to a folder containing subfolders or archives in the left commander pane.
3. Select multiple folders with checkboxes, pick the output format (`.tar` capsule for fastest cloud packaging, or compressed `.tar.zst`, `.zip`, `.7z`, `.rar`, `.tar.gz`, `.tar.xz`, `.tar.bz2`), compression level, execution mode, jobs, and CPU cores.
4. Click **Compress All** to create one archive per selected folder and keep browsing to queue more batches while jobs run.
5. Select archives and click **Extract All** to unpack in place using your chosen extraction mode and conflict policy.

[Full visual guide and video tutorial](https://olarian.com/free-projects/archivatore)

## Build and test locally

Use Go 1.22+ and Node.js 22+ on Windows.

```powershell
cd engine
go test -v ./...

cd ..\app
npm install
npm test
npm run dist
```

The installer is written to `archivatore-install.exe` at the repository root.

## License

Archivatore is distributed under the [MIT License](LICENSE). Electron and optional external CLI tools retain their own licenses.
