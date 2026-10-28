# Windows and Linux adaptation

Both are intended future targets. Keep the core portable; add OS-specific behavior only at filesystem persistence and child-process boundaries. No new platform framework is needed. A native runtime environment can be supplied later; none is needed for the current design review.

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

1. **Test helpers and demo:** replace `/bin/sh` fake scanners with a small compiled Go helper executable; preserve all argument, timeout, malformed-output and partial-coverage assertions. The current helper also uses `sleep`, `cat` and `dd`. Provide a PowerShell or Go fixture demo. Keep symlink tests explicit about required privileges; an unsupported test setup must not look like verified link handling.
2. **State persistence:** state and report/catalog directory durability is isolated in `atomic_sync_unix.go` and `atomic_sync_windows.go`. Windows skips the unsupported read-only-directory `Sync`, while file `Sync` still precedes `Rename`; Unix errors after replacement explicitly report that state was replaced. Validate replacement on NTFS with the new native-Windows-only baseline/append/readback regression. Directory crash durability is not promised on Windows. Go does not promise atomic rename on non-Unix systems. Define old-or-new reader behavior, handle files opened by other processes, and test interrupted writes and locks before making a durability claim. Mode `0600` is not a Windows ACL guarantee. [Go os documentation](https://pkg.go.dev/os#Rename)
3. **Child-process cleanup:** Windows currently cancels only the direct scanner process. Evaluate a Windows Job Object implementation so cancellation also accounts for scanner children such as OpenGrep, and test that no descendant survives. Do not claim Unix process groups cover Windows. [Microsoft Job Objects documentation](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
4. **Paths and link boundaries:** test drive letters, drive-relative input, case variants, spaces/non-ASCII names, long paths, UNC shares, directory symlinks and junction/reparse points. Check containment and skipped-link reporting before claiming the no-follow boundary works on Windows. Do not assume case folding or Unicode normalization is identical across filesystems. Local scope/path identity does not support moving state between OSes.
5. **Environment and executables:** verify `.exe` invocation, argument quoting, temporary directories and cleanup, and necessary home/cache/temp variables under the adapter's environment allowlist. Keep credential removal and LLM-disabled behavior. Run real Skill X-Ray only with an explicitly supplied trusted build; report absent OpenGrep coverage honestly.

## Later runtime test plan

Use disposable fixtures and a non-administrator/non-root account. Record OS, architecture, Go version and filesystem. Start with one Linux amd64 environment and one native Windows amd64 environment; existing macOS validation remains the reference.

- Build, vet and run the complete offline suite; list any skips and unsupported cases explicitly. Windows still needs the fake-scanner helper work above before expecting the full suite to pass.
- Run the copy → baseline → bundled edit → audit → snapshot demo. Confirm exact initial bytes, possible modified correlation, added/removed/modified files, and comparable `new` finding labels.
- Test unreadable files and state directories, missing roots, duplicate names, symlink/junction loops, lock contention, corrupt state, and replacement while another process reads state.
- Copy identical fixture bytes between hosts and compare file/directory hashes, not entire reports containing host-specific absolute paths. Separately convert LF to CRLF and expect exact hashes to change.
- Exercise scanner paths containing spaces, bounded output, cancellation and descendant cleanup. Run the optional real scanner only if safely available, otherwise keep it unverified.

Do not add CI, install virtual machines, or download platform scanner binaries merely to satisfy this plan. Those can follow when a test environment is available.

The directory-sync decision was checked against the installed Go 1.23.5 Windows implementation (`os/file_windows.go`, `internal/poll/fd_fsync_windows.go`, `syscall/syscall_windows.go`) and [Microsoft FlushFileBuffers documentation](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers), which requires a handle with write access. This source review is not a native runtime test.
