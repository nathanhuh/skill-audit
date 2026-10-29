# Windows and Linux adaptation

The rules behind this checklist are in [ADR-0009](docs/adr/0009-portability-boundaries.md).

Both are intended future targets. Keep the core portable; isolate OS-specific behavior at filesystem discovery/persistence and child-process boundaries. Filesystem discovery includes drive enumeration, path handling and link/reparse-point detection. No new platform framework is needed. A native runtime environment can be supplied later; none is needed for the current design review.

## Current evidence

| Target | Evidence | Remaining validation |
| --- | --- | --- |
| macOS arm64 | Tests, vet, build and fixture demo passed | Real Skill X-Ray integration |
| Linux amd64 | Offline cross-build passed on macOS with Go 1.23.5, CGO disabled | Native tests, persistence, permissions, cancellation and demo |
| Windows amd64 | Offline `.exe` cross-build passed on macOS with Go 1.23.5, CGO disabled | Native test harness and runtime adaptations below |

Build success proves compilation only. Native Windows is not yet a supported runtime claim. A Linux VM/container can validate Linux; WSL results do not validate native Windows filesystem or process behavior. ARM64 variants have not been cross-built or run.

## Decisions that already help

- `os.UserHomeDir`, `filepath` and explicit root flags avoid hard-coded host paths in the CLI. Relative manifest paths use `/`; absolute local identities remain host-specific.
- Parser dependencies are vendored and test catalogs are pinned locally; cross-builds need no scanner installation, CGO or network. Runtime audit defaults to the embedded pinned catalog; an explicitly selected external catalog is optional.
- Shell files are parsed in Go, not executed. SA001 does not need a local Bash executable, even on Windows. PowerShell and batch analysis would be separate rules with separate coverage claims.
- Raw-byte SHA-256 deliberately distinguishes LF from CRLF. `.gitattributes` disables text conversion for the pinned catalog so Windows Git settings cannot silently invalidate its hashes. Do not normalize scanned bytes to make an exact match pass. Similarity may still recognize line-ending edits.
- Process setup already has a small OS-specific boundary in `process_unix.go` and `process_other.go`.

## Work before claiming native Windows support

1. **Test helpers and demo:** the rewrite now uses a copied Go test executable for fake scanner argument, staging and cancellation checks. Native Windows execution of that helper remains pending. The demonstration is still a POSIX shell script; provide a PowerShell or Go fixture demo before claiming native Windows support. Keep symlink tests explicit about required privileges; an unsupported test setup must not look like verified link handling.
2. **State persistence:** state and report/catalog directory durability is isolated in `fs_unix.go` and `fs_windows.go`. Windows skips the unsupported read-only-directory `Sync`, while file `Sync` still precedes `Rename`; Unix errors after replacement carry a committed-write outcome: the CLI reports recorded but durability unconfirmed (exit 2), rather than failed recording (exit 3). Validate replacement on NTFS with `TestNativeWindowsStateReplacement` (baseline/append/readback). Directory crash durability is not promised on Windows. Go does not promise atomic rename on non-Unix systems. Define old-or-new reader behavior, handle files opened by other processes, and test interrupted writes and locks before making a durability claim. Mode `0600` is not a Windows ACL guarantee. [Go os documentation](https://pkg.go.dev/os#Rename)
3. **Child-process cleanup:** Windows currently cancels only the direct scanner process. Evaluate a Windows Job Object implementation so cancellation also accounts for scanner children such as OpenGrep, and test that no descendant survives. Do not claim Unix process groups cover Windows. [Microsoft Job Objects documentation](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
4. **Paths and link boundaries:** test drive letters, drive-relative input, case variants, spaces/non-ASCII names, long paths, UNC shares, directory symlinks and junction/reparse points. Check containment and skipped-link reporting before claiming the no-follow boundary works on Windows. Do not assume case folding or Unicode normalization is identical across filesystems. Local scope/path identity does not support moving state between OSes.
5. **Environment and executables:** verify `.exe` invocation, argument quoting, temporary directories and cleanup, and necessary home/cache/temp variables under the adapter's environment allowlist. Keep credential removal and LLM-disabled behavior. Run real Skill X-Ray only with an explicitly supplied trusted build; report absent OpenGrep coverage honestly.

## Later runtime test plan

Use disposable fixtures and a non-administrator/non-root account. Record OS, architecture, Go version and filesystem. Start with one Linux amd64 environment and one native Windows amd64 environment; existing macOS validation remains the reference.

- Build, vet and run the complete offline suite; list any skips and unsupported cases explicitly. The Go helper removes the shell dependency from scanner tests, but the full suite still needs native Windows validation.
- Run the copy → baseline → bundled edit → audit → snapshot demo. Confirm exact initial bytes, possible modified correlation, added/removed/modified files, and comparable `new` finding labels.
- Test unreadable files and state directories, missing roots, duplicate names, symlink/junction loops, lock contention, corrupt state, and replacement while another process reads state.
- Copy identical fixture bytes and relative paths between hosts and compare file/directory hashes, not entire reports containing host-specific absolute paths. Single-host checks do not establish cross-host agreement; ADR-0009 P1 remains pending until another target platform is exercised. Separately convert LF to CRLF and expect exact hashes to change.
- Exercise scanner paths containing spaces, bounded output, cancellation and descendant cleanup. Run the optional real scanner only if safely available, otherwise keep it unverified.

Do not add CI, install virtual machines, or download platform scanner binaries merely to satisfy this plan. Those can follow when a test environment is available.

The directory-sync decision was checked against the installed Go 1.23.5 Windows implementation (`os/file_windows.go`, `internal/poll/fd_fsync_windows.go`, `syscall/syscall_windows.go`) and [Microsoft FlushFileBuffers documentation](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers), which requires a handle with write access. This source review is not a native runtime test.

[ADR-0004](docs/adr/0004-observations-and-baselines.md)/[0005](docs/adr/0005-outcomes-and-exit-codes.md) alignment gives security findings exit priority over coverage and save warnings. Report/v2 independently records persistence status; state/v3 derives complete-package anchors from the first observation despite unrelated gaps. Stored observations omit the final recording outcome, which is only known after the write; CLI reports carry it. Published state/v1 remains comparison-readable and append-read-only; unpublished state/v2 is rejected. Case-alias overlap is now checked against filesystem identity and exercised on the macOS fixture volume. The rewrite exercises case aliases on this volume; distinct case-sensitive directories still need another native filesystem; Linux/case-sensitive-volume and native Windows runtime validation remain pending. Root identity checks do not promise Unicode normalization or arbitrary mount deduplication.

## Isolated rewrite evidence (2026-10-29)

The rewrite remains separate from canonical main; see docs/rewrite-comparison.md and VERIFICATION.md. Filesystem policy lives in fs_policy.go, Unix/Windows link and persistence behavior in fs_unix.go/fs_windows.go, and child-process behavior in process_unix.go/process_windows.go. Windows checks reparse attributes as well as ModeSymlink, but D7/P4 still require native junction evidence. P3 remains pending: Windows cancellation only kills the direct child, without Job Objects. The new Go fake scanner is compile-checked for Windows; this is not a native execution result. P1 remains a cross-host check, not established by same-host fixture hashes.
