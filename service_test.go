package k6build

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/grafana/k6catalog"
	"github.com/grafana/k6foundry"
	"github.com/grafana/k6foundry/pkg/testutils/goproxy"
)

func TestBuild(t *testing.T) {
	modules := []struct {
		path    string
		version string
		source  string
	}{
		{
			path:    "go.k6.io/k6",
			version: "v0.1.0",
			source:  "testdata/deps/k6",
		},
		{
			path:    "go.k6.io/k6",
			version: "v0.2.0",
			source:  "testdata/deps/k6",
		},
		{
			path:    "go.k6.io/k6ext",
			version: "v0.1.0",
			source:  "testdata/deps/k6ext",
		},
		{
			path:    "go.k6.io/k6ext",
			version: "v0.2.0",
			source:  "testdata/deps/k6ext",
		},
		{
			path:    "go.k6.io/k6ext2",
			version: "v0.1.0",
			source:  "testdata/deps/k6ext2",
		},
	}

	// creates a goproxy that serves the given modules
	proxy := goproxy.NewGoProxy()
	for _, m := range modules {
		err := proxy.AddModVersion(m.path, m.version, m.source)
		if err != nil {
			t.Fatalf("setup %v", err)
		}
	}

	goproxySrv := httptest.NewServer(proxy)

	opts := k6foundry.NativeBuilderOpts{
		GoOpts: k6foundry.GoOpts{
			CopyEnv:    true,
			GoProxy:    goproxySrv.URL,
			GoNoProxy:  "none",
			GoPrivate:  "go.k6.io",
			EphemeralCache: true,
		},
	}

	builder, err := k6foundry.NewNativeBuilder(context.Background(), opts)
	if err != nil {
		t.Fatalf("setting up test builder %v", err)
	}

	registry, err := k6catalog.NewRegistryFromJSON("testdata/registry.json")
	if err != nil {
		t.Fatalf("setting up test builder %v", err)
	}
	catalog := k6catalog.NewCatalog(registry)

	testCases := []struct {
		title        string
		k6Constrains string
		deps         []Dependency
		expectErr    error
	}{
		{
			title:        "build k6 v0.1.0 ",
			k6Constrains: "v0.1.0",
			deps:         []Dependency{},
			expectErr:    nil,
		},
		{
			title:        "build k6 >v0.1.0",
			k6Constrains: ">v0.1.0",
			deps:         []Dependency{},
			expectErr:    nil,
		},
		{
			title:        "build unsatisfied k6 constrain (>v0.2.0)",
			k6Constrains: ">v0.2.0",
			deps:         []Dependency{},
			expectErr:    k6catalog.ErrCannotSatisfy,
		},
		{
			title:        "build k6 v0.1.0 exact dependency constraint",
			k6Constrains: "v0.1.0",
			deps:         []Dependency{{Name: "k6/x/ext", Constraints: "v0.1.0"}},
			expectErr:    nil,
		},
		{
			title:        "build k6 v0.1.0 unsatisfied dependency constrain",
			k6Constrains: "v0.1.0",
			deps:         []Dependency{{Name: "k6/x/ext", Constraints: ">v0.2.0"}},
			expectErr:    k6catalog.ErrCannotSatisfy,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			cache, err := NewTempFileCache()
			if err != nil {
				t.Fatalf("creating temp cache %v", err)
			}

			buildsrv := NewBuildService(catalog, builder, cache)

			_, err = buildsrv.Build(
				context.TODO(),
				"linux/amd64",
				tc.k6Constrains,
				tc.deps,
			)

			if !errors.Is(err, tc.expectErr) {
				t.Fatalf("unexpected error wanted %v got %v", tc.expectErr, err)
			}
		})
	}
}
