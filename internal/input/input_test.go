package input

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenZIP(t *testing.T) {
	archive := writeZIP(t, []string{"folder/usage.xml"})
	reader, err := Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<feed/>" {
		t.Fatalf("unexpected content %q", data)
	}
}

func TestRejectUnsafeAndAmbiguousZIPs(t *testing.T) {
	for _, names := range [][]string{{"../usage.xml"}, {"..\\usage.xml"}, {"/usage.xml"}, {"a.xml", "b.xml"}, {"readme.txt"}} {
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			if reader, err := Open(writeZIP(t, names)); err == nil {
				_ = reader.Close()
				t.Fatalf("accepted %v", names)
			}
		})
	}
}

func TestAcceptFilenameContainingTwoDots(t *testing.T) {
	reader, err := Open(writeZIP(t, []string{"usage..backup.xml"}))
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
}

func TestRejectTooManyZIPMembers(t *testing.T) {
	names := make([]string, MaxArchiveEntries+1)
	for i := range names {
		names[i] = fmt.Sprintf("files/%05d.txt", i)
	}
	if reader, err := Open(writeZIP(t, names)); err == nil {
		_ = reader.Close()
		t.Fatal("accepted excessive ZIP members")
	}
}

func TestRejectNonRegularInput(t *testing.T) {
	if reader, err := Open(t.TempDir()); err == nil {
		_ = reader.Close()
		t.Fatal("accepted directory input")
	}
}

func writeZIP(t *testing.T, names []string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "fixture.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range names {
		member, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write([]byte("<feed/>")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archive
}
