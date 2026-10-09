//go:build windows

package fsx_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/mrtheroi/memry-cli/internal/fsx"
)

// dacl reads the DACL of path: whether it is protected from inheritance
// and the SIDs of its ACEs.
func dacl(t *testing.T, path string) (protected bool, sids []*windows.SID) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			t.Fatal(err)
		}
		sids = append(sids, (*windows.SID)(unsafe.Pointer(&ace.SidStart)))
	}
	return control&windows.SE_DACL_PROTECTED != 0, sids
}

func currentUser(t *testing.T) *windows.SID {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return user.User.Sid
}

func assertOwnerOnly(t *testing.T, path string) {
	t.Helper()
	protected, sids := dacl(t, path)
	if !protected {
		t.Error("the DACL inherits from the directory, want it protected")
	}
	if len(sids) != 1 || !sids[0].Equals(currentUser(t)) {
		t.Errorf("DACL SIDs = %v, want only the current user %v", sids, currentUser(t))
	}
}

// S1.5.c: the real DACL is protected and has one ACE, for the user.
func TestWriteAtomicLeavesAProtectedOwnerOnlyDACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := fsx.WriteAtomic(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	assertOwnerOnly(t, path)
}

// S1.5.c: a widened ACL is tightened by the next write.
func TestWriteAtomicTightensAWidenedDACLOnTheNextWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	widened, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(everyone),
		},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, widened, nil); err != nil {
		t.Fatal(err)
	}

	if err := fsx.WriteAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	assertOwnerOnly(t, path)
}

func TestRetryableIsTheErrnosOfAFileInUse(t *testing.T) {
	for _, errno := range []syscall.Errno{windows.ERROR_SHARING_VIOLATION, windows.ERROR_ACCESS_DENIED, windows.ERROR_LOCK_VIOLATION} {
		if !fsx.Retryable(&os.LinkError{Op: "rename", Err: errno}) {
			t.Errorf("errno %d is not retryable, want it to be", errno)
		}
	}
	if fsx.Retryable(&os.LinkError{Op: "rename", Err: windows.ERROR_PATH_NOT_FOUND}) {
		t.Error("ERROR_PATH_NOT_FOUND is retryable, want it not to be")
	}
}

// A real locked file: the target is held open without FILE_SHARE_DELETE
// for ~50 ms, less than the retries wait in total.
func TestWriteAtomicRetriesWhileAnotherProgramHoldsTheFileOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	held, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(50 * time.Millisecond)
		_ = windows.CloseHandle(held)
	}()

	err = fsx.WriteAtomic(path, []byte("new"), 0o600)
	<-released

	if err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("contents = %q, want new", got)
	}
}
