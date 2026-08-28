//go:build windows

package pty

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type conPTY struct {
	hpc        windows.Handle
	job        windows.Handle
	pipeIn     windows.Handle // write end: geckty -> child stdin
	pipeOut    windows.Handle // read end: child stdout -> geckty
	proc       windows.Handle
	pid        uint32
	closed     chan struct{}
	closedOnce func()
}

// conptyHandles tracks OS resources acquired during Open so error paths
// can release them without repeating CloseHandle sequences.
type conptyHandles struct {
	hpc                    windows.Handle
	ptyInWrite, ptyOutRead windows.Handle
}

func (h *conptyHandles) release() {
	if h.hpc != 0 {
		windows.ClosePseudoConsole(h.hpc)
		h.hpc = 0
	}
	if h.ptyInWrite != 0 {
		_ = windows.CloseHandle(h.ptyInWrite)
		h.ptyInWrite = 0
	}
	if h.ptyOutRead != 0 {
		_ = windows.CloseHandle(h.ptyOutRead)
		h.ptyOutRead = 0
	}
}

// Open spawns Config.Command (or the platform default shell) attached to a
// new Windows ConPTY (windows.CreatePseudoConsole via kernel32).
//
// Deliberately does NOT depend on a side-by-side OpenConsole.exe /
// conpty.dll bundle (what Windows Terminal ships). The public ConPTY API
// is the supported contract; optionally loading a newer host remains a
// future opt-in, not a hard dependency — see the project roadmap.
//
// This mirrors the sequence termizard's pty_windows.go uses, including two
// gotchas that are easy to get wrong:
//
//  1. The PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE handle must be passed BY VALUE
//     to ProcThreadAttributeListContainer.Update, not by pointer. Passing
//     &hpc (pointer-to-handle) causes CreateProcess to fail with
//     STATUS_DLL_INIT_FAILED (0xC0000142).
//  2. CREATE_NO_WINDOW, DETACHED_PROCESS, and CREATE_NEW_CONSOLE are all
//     forbidden in combination with ConPTY — they conflict with the
//     pseudoconsole's own console allocation.
//
// ConPTY performs VT translation internally, so unlike a classic Windows
// console host, no SetConsoleMode/ENABLE_VIRTUAL_TERMINAL_PROCESSING calls
// are needed or made here.
func Open(cfg Config) (PTY, error) {
	command := cfg.Command
	if len(command) == 0 {
		command = []string{resolveShell()}
	}
	command = ensureInteractive(command)

	cols, rows := cfg.Cols, cfg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	h, err := createConptyPipes(cols, rows)
	if err != nil {
		return nil, err
	}

	si, attrList, err := attachPseudoConsole(h.hpc)
	if err != nil {
		h.release()
		return nil, err
	}
	defer attrList.Delete()

	cmdLine, envBlock, dirPtr, err := prepareProcessParams(command, cfg)
	if err != nil {
		h.release()
		return nil, err
	}

	pi, err := spawnConptyProcess(cmdLine, envBlock, dirPtr, si)
	if err != nil {
		h.release()
		return nil, err
	}
	_ = windows.CloseHandle(pi.Thread)

	job := assignKillOnCloseJob(pi.Process)

	return &conPTY{
		hpc:     h.hpc,
		job:     job,
		pipeIn:  h.ptyInWrite,
		pipeOut: h.ptyOutRead,
		proc:    pi.Process,
		pid:     pi.ProcessId,
		closed:  make(chan struct{}),
	}, nil
}

func createConptyPipes(cols, rows uint16) (*conptyHandles, error) {
	var ptyIn, ptyInWrite, ptyOutRead, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &ptyInWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	if err := windows.CreatePipe(&ptyOutRead, &ptyOut, nil, 0); err != nil {
		_ = windows.CloseHandle(ptyIn)
		_ = windows.CloseHandle(ptyInWrite)
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	var hpc windows.Handle
	err := windows.CreatePseudoConsole(
		windows.Coord{X: int16(cols), Y: int16(rows)},
		ptyIn, ptyOut, 0, &hpc,
	)
	_ = windows.CloseHandle(ptyIn)
	_ = windows.CloseHandle(ptyOut)
	if err != nil {
		_ = windows.CloseHandle(ptyInWrite)
		_ = windows.CloseHandle(ptyOutRead)
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	return &conptyHandles{hpc: hpc, ptyInWrite: ptyInWrite, ptyOutRead: ptyOutRead}, nil
}

func attachPseudoConsole(hpc windows.Handle) (*windows.StartupInfoEx, *windows.ProcThreadAttributeListContainer, error) {
	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, nil, fmt.Errorf("NewProcThreadAttributeList: %w", err)
	}
	// `go vet` flags this uintptr->unsafe.Pointer conversion as a possible
	// misuse (its heuristic assumes uintptr values came from a Go pointer
	// that GC could move). hpc is an opaque OS handle, never a Go
	// pointer, so there is nothing to move — this is the documented
	// Win32 contract for PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE.
	if err := attrList.Update(
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		unsafe.Pointer(hpc), // by value — see Open doc comment
		unsafe.Sizeof(hpc),
	); err != nil {
		attrList.Delete()
		return nil, nil, fmt.Errorf("update attribute list: %w", err)
	}
	si := &windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
		},
		ProcThreadAttributeList: attrList.List(),
	}
	return si, attrList, nil
}

func prepareProcessParams(command []string, cfg Config) (cmdLine *uint16, envBlock *uint16, dirPtr *uint16, err error) {
	cmdLine, err = windows.UTF16PtrFromString(windows.ComposeCommandLine(command))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("compose command line: %w", err)
	}
	envBlock, err = makeEnvBlock(preparePlatformEnv(cfg.Env))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build environment block: %w", err)
	}
	dir := cfg.Dir
	if dir == "" {
		if home, herr := os.UserHomeDir(); herr == nil {
			dir = home
		}
	}
	if dir != "" {
		dirPtr, err = windows.UTF16PtrFromString(dir)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("convert dir: %w", err)
		}
	}
	return cmdLine, envBlock, dirPtr, nil
}

func spawnConptyProcess(cmdLine, envBlock, dirPtr *uint16, si *windows.StartupInfoEx) (*windows.ProcessInformation, error) {
	pi := new(windows.ProcessInformation)
	err := windows.CreateProcess(
		nil, cmdLine, nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT,
		envBlock, dirPtr, &si.StartupInfo, pi,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}
	return pi, nil
}

func assignKillOnCloseJob(proc windows.Handle) windows.Handle {
	job, jerr := windows.CreateJobObject(nil, nil)
	if jerr != nil {
		return 0
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, _ = windows.SetInformationJobObject(
		job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)),
	)
	_ = windows.AssignProcessToJobObject(job, proc)
	return job
}

func (p *conPTY) Write(b []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(p.pipeIn, b, &n, nil)
	return int(n), err
}

// Read polls the output pipe with PeekNamedPipe rather than issuing a
// blocking ReadFile. A blocking ReadFile would never return once the child
// exits and ConPTY closes its end mid-read, which would prevent Close from
// ever unblocking a caller stuck in Read. Polling lets Read notice p.closed
// and return promptly on shutdown.
func (p *conPTY) Read(b []byte) (int, error) {
	const pollInterval = 5 * time.Millisecond
	for {
		select {
		case <-p.closed:
			return 0, os.ErrClosed
		default:
		}

		avail, err := peekNamedPipe(p.pipeOut)
		if err != nil {
			return 0, err
		}
		if avail == 0 {
			time.Sleep(pollInterval)
			continue
		}

		var n uint32
		if err := windows.ReadFile(p.pipeOut, b, &n, nil); err != nil {
			return 0, err
		}
		return int(n), nil
	}
}

func (p *conPTY) Resize(cols, rows uint16) error {
	return windows.ResizePseudoConsole(p.hpc, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (p *conPTY) Pid() int { return int(p.pid) }

func (p *conPTY) Wait() error {
	_, err := windows.WaitForSingleObject(p.proc, windows.INFINITE)
	return err
}

func (p *conPTY) Close() error {
	select {
	case <-p.closed:
		return nil
	default:
		close(p.closed)
	}

	if p.job != 0 {
		// Closing the job handle kills the whole process tree
		// (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE).
		_ = windows.CloseHandle(p.job)
	} else {
		_ = windows.TerminateProcess(p.proc, 1)
	}

	windows.ClosePseudoConsole(p.hpc)
	_ = windows.CloseHandle(p.pipeIn)
	_ = windows.CloseHandle(p.pipeOut)
	_ = windows.CloseHandle(p.proc)
	return nil
}
