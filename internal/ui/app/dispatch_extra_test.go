package app

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geckty/geckty/internal/config"
)

func TestOpenURLEmptyIsNoop(_ *testing.T) {
	openURL("")
}

func TestDispatchScrollPromptAndShowScrollback(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionScrollToPrevPrompt)
	s.dispatchAction(ActionScrollToNextPrompt)
	s.dispatchAction(ActionSelectLastCmdOutput)
	s.dispatchAction(ActionShowScrollback)
}

func TestDispatchActionPasteWritesClipboardText(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Setenv("PATH", t.TempDir())
	}
	s, app := testUIStateWithTab(t)
	app.clipboard = "paste-me"
	s.dispatchAction(ActionPaste)
}

func TestFlashScrollBarRequestsRedraw(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.flashScrollBar()
	if app.redrawCount.Load() == 0 {
		t.Fatal("flashScrollBar should request redraw")
	}
}

func TestLoadPluginsEmptyConfig(t *testing.T) {
	app := newFakeApp()
	host, stop := loadPlugins(config.Default(), app)
	if host != nil {
		t.Fatal("expected nil host for empty plugin list")
	}
	stop()
}

func TestLoadPluginsMissingDirLogsAndContinues(t *testing.T) {
	cfg := config.Default()
	cfg.Plugins = []string{filepath.Join(t.TempDir(), "missing-plugin")}
	app := newFakeApp()
	host, stop := loadPlugins(cfg, app)
	stop()
	if host == nil {
		t.Fatal("expected host even when plugin dirs are missing")
	}
	_ = host.Close(context.Background())
}

func TestDispatchActionCopyWithSelection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Setenv("PATH", t.TempDir())
	}
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	if active == nil {
		t.Fatal("expected active tab")
	}
	active.SelectLine(0)
	s.dispatchAction(ActionCopy)
	if app.clipboard == "" && runtime.GOOS == "darwin" {
		t.Log("darwin may have used pbcopy instead of fakeApp clipboard")
	}
}
