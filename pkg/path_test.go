package pkg

import "testing"

func TestPath(t *testing.T) {

	t.Run("resolving", func(t *testing.T) {
		root := RootGitPath()

		fooBar, err := root.Resolve("./foo/bar.cc")
		if err != nil {
			t.Fatalf("failed to resolve './foo/bar.cc': %v", err)
		}

		if text := fooBar.String(); text != "foo/bar.cc" {
			t.Fatalf("resolved path expected 'foo/bar.cc', got: %s", text)
		}

		parentDir, err := fooBar.Resolve("..")
		if err != nil {
			t.Fatalf("failed to resolve '..': %v", err)
		}

		if text := parentDir.String(); text != "foo" {
			t.Fatalf("resolved path expected 'foo', got: %s", text)
		}
	})

	t.Run("parents", func(t *testing.T) {
		p := FilePath{
			Path: []string{
				"foo", "bar.cc",
			},
		}

		if p.String() != "foo/bar.cc" {
			t.Fatalf("p.String() should return foo/bar.cc got: '%s'", p.String())
		}

		parent := p.Parent()
		if parent.String() != "foo" {
			t.Fatalf("parent.String() should return foo got: '%s'", parent.String())
		}

		root := parent.Parent()
		if !root.IsRoot() {
			t.Fatalf("root.IsRoot() should return true got: false")
		}

		stillRoot := root.Parent()
		if !stillRoot.IsRoot() {
			t.Fatalf("taking Parent() of root should still be the root")
		}

	})

	t.Run("matches", func(t *testing.T) {
		p := FilePath{
			Path: []string{
				"branches", "main", "foo", "bar.cc",
			},
		}

		selected, remaining, err := p.ConsumeMatches("branches", "*", "...")
		if err != nil {
			t.Fatalf("expected to be able to match prefix: %v", err)
		}

		if selected[0] != "main" {
			t.Fatalf("expected for selected[0] == 'main': %v", selected)
		}

		if text := remaining.String(); text != "foo/bar.cc" {
			t.Fatalf("remaining should equal 'foo/bar.cc': %s", text)
		}
	})
}
