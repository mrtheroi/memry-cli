package home

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestDir_WindowsLocalAppDataFallback(t *testing.T) {
	got, err := Dir("windows", env(map[string]string{"LOCALAPPDATA": `C:\U\a\AppData\Local`}))
	if err != nil || got != `C:\U\a` {
		t.Fatalf("Dir = %q, %v; want C:\\U\\a", got, err)
	}
}

func TestDir_LocalAppDataTrailingSeparatorIgnored(t *testing.T) {
	got, err := Dir("windows", env(map[string]string{"LOCALAPPDATA": `C:\U\a\AppData\Local\`}))
	if err != nil || got != `C:\U\a` {
		t.Fatalf("Dir = %q, %v; want C:\\U\\a", got, err)
	}
}

func TestDir_LocalAppDataWithoutTwoParentsIsNoHome(t *testing.T) {
	for _, local := range []string{"x", `\Local`, `C:\Local`, `\\\`} {
		got, err := Dir("windows", env(map[string]string{"LOCALAPPDATA": local}))
		if !errors.Is(err, ErrNoHome) || got != "" {
			t.Errorf("Dir(LOCALAPPDATA=%q) = %q, %v; want ErrNoHome", local, got, err)
		}
	}
}

func TestDir_WindowsUserprofileFirst(t *testing.T) {
	got, err := Dir("windows", env(map[string]string{
		"USERPROFILE": `C:\Users\ana`, "HOME": `D:\h`, "LOCALAPPDATA": `C:\U\a\AppData\Local`,
	}))
	if err != nil || got != `C:\Users\ana` {
		t.Fatalf("Dir = %q, %v; want C:\\Users\\ana", got, err)
	}
}

func TestDir_WindowsHomeFallback(t *testing.T) {
	got, err := Dir("windows", env(map[string]string{"HOME": `D:\h`, "LOCALAPPDATA": `C:\U\a\AppData\Local`}))
	if err != nil || got != `D:\h` {
		t.Fatalf("Dir = %q, %v; want D:\\h", got, err)
	}
}

func TestDir_UnixHomeFirst(t *testing.T) {
	got, err := Dir("linux", env(map[string]string{"HOME": "/home/ana", "USERPROFILE": `C:\Users\ana`}))
	if err != nil || got != "/home/ana" {
		t.Fatalf("Dir = %q, %v; want /home/ana", got, err)
	}
}

func TestDir_NothingSetReturnsErrNoHome(t *testing.T) {
	for _, goos := range []string{"", "linux", "windows"} {
		got, err := Dir(goos, env(nil))
		if !errors.Is(err, ErrNoHome) || got != "" {
			t.Errorf("Dir(%q) = %q, %v; want ErrNoHome", goos, got, err)
		}
	}
}

func TestJoin_PreservesTrailingSlashBase(t *testing.T) {
	got := Join("/x/", ".config", "memry", "config.json")
	if got != "/x//.config/memry/config.json" {
		t.Fatalf("Join = %q", got)
	}
}

func TestJoin_JoinsElementsUnderBase(t *testing.T) {
	got := Join("/home/ana", ".codeium", "windsurf", "mcp_config.json")
	want := "/home/ana" + string(os.PathSeparator) + filepath.Join(".codeium", "windsurf", "mcp_config.json")
	if got != want {
		t.Fatalf("Join = %q; want %q", got, want)
	}
}
