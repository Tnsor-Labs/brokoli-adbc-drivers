// Command loadcheck proves every driver in a catalog index can be installed
// and loaded the way a Brokoli worker installs and loads it.
//
// For each entry it uses Brokoli's own code to read the index (the same
// validation a server applies), download the archive, verify its digest,
// check that its manifest names that entry, and install it; then it loads
// the library through the ADBC driver manager, as the isolated worker does.
// A library that cannot be loaded -- a missing shared-library dependency, a
// missing entrypoint, a build for the wrong platform -- fails the check.
// Failing to reach a database afterwards is expected and ignored: the
// connection URI is a placeholder.
//
// Run it in an environment shaped like the native worker image, so a
// dependency the image does not have is caught here, not on a fleet.
//
//	loadcheck <index.json>
package main

import (
	"context"
	"debug/elf"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Tnsor-Labs/brokoli/pkg/drivers"
	"github.com/Tnsor-Labs/brokoli/pkg/netguard"
	"github.com/apache/arrow-adbc/go/adbc"
	"github.com/apache/arrow-adbc/go/adbc/drivermgr"
)

// loadFailures are the driver manager's words for a library that could not
// be loaded or initialised at all, as opposed to one that loaded and then
// refused the placeholder connection.
var loadFailures = []string{
	"Could not load", "could not load", "cannot open shared object", "undefined symbol",
	"Could not find function", "could not find function", "wrong ELF class", "version `GLIBC",
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: loadcheck <index.json>")
		os.Exit(2)
	}
	ctx := context.Background()

	// Serve the index under test to Brokoli's own reader, so it is held to
	// exactly the rules a server applies. Loopback is opened only for that.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal("listen: %v", err)
	}
	index := os.Args[1]
	go func() {
		_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, index) }))
	}()
	restore := netguard.SetOutboundForTesting(netguard.Policy{AllowLoopback: true})
	idx, err := drivers.FetchIndex(ctx, "http://"+listener.Addr().String()+"/index.json")
	restore()
	if err != nil {
		fatal("the index is not one Brokoli accepts: %v", err)
	}

	root, err := os.MkdirTemp("", "loadcheck-")
	if err != nil {
		fatal("%v", err)
	}
	defer os.RemoveAll(root)
	manager, err := drivers.NewManager(filepath.Join(root, "drivers"))
	if err != nil {
		fatal("%v", err)
	}

	failed, checked := 0, 0
	for _, entry := range idx.Drivers {
		if entry.OS != runtime.GOOS || entry.Arch != runtime.GOARCH {
			fmt.Printf("skip %-12s %-8s %s/%s (not this platform)\n", entry.Name, entry.Version, entry.OS, entry.Arch)
			continue
		}
		checked++
		if err := check(ctx, manager, root, entry); err != nil {
			failed++
			fmt.Printf("FAIL %-12s %-8s %v\n", entry.Name, entry.Version, err)
		}
	}
	fmt.Printf("\n%d checked, %d failed\n", checked, failed)
	if checked == 0 {
		fatal("no entry for %s/%s: a check that checked nothing has not passed", runtime.GOOS, runtime.GOARCH)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func check(ctx context.Context, manager *drivers.Manager, root string, entry drivers.IndexEntry) error {
	archive := filepath.Join(root, entry.Name+"-"+entry.Version+".tar.gz")
	if err := drivers.DownloadArchive(ctx, entry.ArchiveURL, entry.SHA256, archive, 512<<20); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	installed, err := manager.InstallArchive(archive, entry.SHA256)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if installed.Name != entry.Name || installed.Version != entry.Version {
		return fmt.Errorf("archive contains %s %s, but the entry is %s %s", installed.Name, installed.Version, entry.Name, entry.Version)
	}

	if runtime.GOOS == "linux" {
		if err := selfContained(installed.LibraryPath()); err != nil {
			return err
		}
	}

	var driver drivermgr.Driver
	db, err := driver.NewDatabase(map[string]string{
		"driver":          installed.LibraryPath(),
		"entrypoint":      installed.Entrypoint,
		adbc.OptionKeyURI: placeholderURI(entry.Name),
	})
	if err == nil {
		defer db.Close()
		var conn adbc.Connection
		if conn, err = db.Open(ctx); err == nil {
			_ = conn.Close()
		}
	}
	if err != nil {
		for _, marker := range loadFailures {
			if strings.Contains(err.Error(), marker) {
				return fmt.Errorf("does not load: %w", err)
			}
		}
	}
	fmt.Printf("ok   %-12s %-8s loads (library %s)\n", entry.Name, entry.Version, installed.Identity().LibrarySHA256[:12])
	return nil
}

// baseLibraries are the shared libraries a driver may depend on: the C and
// C++ runtime every Linux system and the native worker image provide.
// Anything else must be inside the driver, or the driver works only where
// that library happens to be installed -- which is how a PostgreSQL build
// that needed the system's libpq, and a SQLite build that needed its
// libsqlite3, got into the catalog.
var baseLibraries = map[string]bool{
	"libc.so.6": true, "libm.so.6": true, "libdl.so.2": true, "libpthread.so.0": true, "librt.so.1": true, "libresolv.so.2": true,
	"libstdc++.so.6": true, "libgcc_s.so.1": true, "ld-linux-x86-64.so.2": true, "ld-linux-aarch64.so.1": true,
}

// selfContained reports a shared-library dependency outside the base set.
func selfContained(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("not an ELF library: %w", err)
	}
	defer f.Close()
	needed, err := f.ImportedLibraries()
	if err != nil {
		return fmt.Errorf("read dependencies: %w", err)
	}
	var outside []string
	for _, lib := range needed {
		if !baseLibraries[lib] {
			outside = append(outside, lib)
		}
	}
	if len(outside) > 0 {
		return fmt.Errorf("depends on %s, which is not part of the driver; bundle or statically link it", strings.Join(outside, ", "))
	}
	return nil
}

// placeholderURI is a URI the driver can parse, pointing at nothing. Only
// whether the library loads is being checked.
func placeholderURI(name string) string {
	switch name {
	case "sqlite", "duckdb":
		return ":memory:"
	case "postgresql":
		return "postgresql://check@127.0.0.1:1/check"
	case "flightsql":
		return "grpc+tcp://127.0.0.1:1"
	default:
		return "http://127.0.0.1:1"
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "loadcheck: "+format+"\n", args...)
	os.Exit(1)
}
