package api

import (
	"golang.org/x/sys/windows"

	"testing"
)

func TestSetEntriesInAcl(t *testing.T) {
	var (
		entries = []ExplicitAccess{
			{
				AccessPermissions: windows.GENERIC_READ,
				AccessMode:        GRANT_ACCESS,
				Inheritance:       NO_INHERITANCE,
				Trustee: Trustee{
					TrusteeForm: TRUSTEE_IS_NAME,
					Name:        windows.StringToUTF16Ptr("CURRENT_USER"),
				},
			},
		}
		acl windows.Handle
	)
	if err := SetEntriesInAcl(
		entries,
		0,
		&acl,
	); err != nil {
		t.Fatal(err)
	}
	defer windows.LocalFree(acl)
}
