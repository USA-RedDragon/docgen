package cli

import (
	"strings"
	"testing"
)

const rustLinksPage = `<!-- docgen:errors crate=err-store type=error::Error -->
<!-- /docgen:errors -->

<!-- docgen:errors crate=core-a type=errors::Error -->
<!-- /docgen:errors -->

<!-- docgen:doc crate=err-store -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=err-store name=store::Store -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=err-store name=store::Store::CAPACITY -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=err-store name=store::Store::open -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=err-store name=store::Store::first -->
<!-- /docgen:doc -->

<!-- docgen:doc crate=core-a -->
<!-- /docgen:doc -->
`

func TestRustLinks(t *testing.T) {
	t.Parallel()
	needCargo(t)
	root := rustRepo(t, map[string]string{"docs/links.md": rustLinksPage})
	if rc, log := docgen(t, root, "update"); rc != 0 {
		t.Fatalf("update: rc %d, log:\n%s", rc, log)
	}
	got := page(t, root, "links.md")
	const base = "https://docs.rs/err-store/latest/err_store/"
	wantIn(t, got,
		"| Error | Returned by | Message | Source | Meaning |\n|---|---|---|---|---|\n",
		"| [`Error::NotFound`]("+base+"error/enum.Error.html#variant.NotFound) | `store::Store::get`, `store::Store::remove`, `store::count` | `key not found: {0}` |  | The key is not in the store. |\n",
		"| [`Error::Locked`]("+base+"error/enum.Error.html#variant.Locked) | `store::Store::open`, `store::Store::remove`, `store::count` | `store is locked` |  | Another process holds the lock of the store. |\n",
		"| [`Error::Corrupt`]("+base+"error/enum.Error.html#variant.Corrupt) | `error::Error::from_code`, `repair` | `corrupt data` |  | The data on disk is corrupt. |\n",
		"| [`Error::Full`]("+base+"error/enum.Error.html#variant.Full) |  | `store is full` |  | The store is full. |\n",
		"Open a [`Store`]("+base+"store/struct.Store.html) and read a key with [`Store::get`]("+base+"store/struct.Store.html#method.get).",
		"[`Error`]("+base+"error/enum.Error.html), such as [`Error::NotFound`]("+base+"error/enum.Error.html#variant.NotFound).",
		"[`crate::MAX_KEYS`]("+base+"constant.MAX_KEYS.html) for the size.",
		"name=store::Store::CAPACITY -->\nThe most values that a store holds, as [`MAX_KEYS`]("+base+"constant.MAX_KEYS.html) says.\n",
		"Fails with [`Error::Locked`]("+base+"error/enum.Error.html#variant.Locked) when",
		"It does not return\n[`Error::Locked`]("+base+"error/enum.Error.html#variant.Locked), and the section below names only `Self::NotFound`,\nwhich is not a variant of [`Error`]("+base+"error/enum.Error.html) for a [`Store`]("+base+"store/struct.Store.html).",
		"| `Error::Read` |  | `read {path}: {source}` |",
		"The app runs at most `limits::MAX_JOBS` jobs at once",
	)
	if strings.Contains(got, "docs.rs/core-a") {
		t.Errorf("a crate with publish = false has docs.rs links:\n%s", got)
	}
	if rc, log := docgen(t, root, "check"); rc != 0 {
		t.Fatalf("check: rc %d, log:\n%s", rc, log)
	}
}
