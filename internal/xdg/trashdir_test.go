package xdg

import (
	"testing"

	"github.com/moby/sys/mountinfo"
	"github.com/stretchr/testify/require"
)

func TestGetMountpoint(t *testing.T) {
	// replace to stub
	mountinfo_GetMounts = func(f mountinfo.FilterFunc) ([]*mountinfo.Info, error) {
		return []*mountinfo.Info{
			{Mountpoint: "/"},
			{Mountpoint: "/foo/bar"},
			{Mountpoint: "/foo"},
			{Mountpoint: "/fooo/bar"},
			{Mountpoint: "/ffoo/bar"},
		}, nil
	}

	testsNormal := []struct {
		path string
		want string
	}{
		{path: "/a.txt", want: "/"},
		{path: "/foo/bar/a.txt", want: "/foo/bar"},
		{path: "/foo/bar/aaa/b.txt", want: "/foo/bar"},
		{path: "/ffoo/bar/a.txt", want: "/ffoo/bar"},
		{path: "/aaa/bbb/ccc/ddd.txt", want: "/"},
		{path: "/", want: "/"},
	}

	t.Run("normal", func(t *testing.T) {
		for _, tt := range testsNormal {
			got, err := getMountpoint(tt.path)
			require.NoError(t, err)
			if got != tt.want {
				t.Errorf("getMountpoint(%q) = %q, want %q", tt.path, got, tt.want)
			}
		}
	})

	t.Run("error", func(t *testing.T) {
		got, err := getMountpoint("")
		require.Error(t, err, got)
	})
}
