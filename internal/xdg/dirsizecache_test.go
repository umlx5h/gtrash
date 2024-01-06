package xdg

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDirCache(t *testing.T) {
	want := make(DirCache)
	want["bar"] = &struct {
		Item DirCacheItem
		Seen bool
	}{
		Item: DirCacheItem{
			Size:    10000,
			Mtime:   time.Unix(1672531200, 0),
			DirName: "bar",
		},
		Seen: false,
	}

	want["foo"] = &struct {
		Item DirCacheItem
		Seen bool
	}{
		Item: DirCacheItem{
			Size:    20000,
			Mtime:   time.Unix(1672531200, 0),
			DirName: "foo",
		},
		Seen: false,
	}

	want["あい うえお"] = &struct {
		Item DirCacheItem
		Seen bool
	}{
		Item: DirCacheItem{
			Size:    40000,
			Mtime:   time.Unix(1672531200, 0),
			DirName: "あい うえお",
		},
		Seen: false,
	}

	file := `10000 1672531200 bar
20000 1672531200 foo
40000 1672531200 %E3%81%82%E3%81%84%20%E3%81%86%E3%81%88%E3%81%8A
`

	got, err := NewDirCache(strings.NewReader(file))
	require.NoError(t, err)
	assert.EqualValues(t, want, got, "parse directorysizes")

	assert.Equal(t, file, got.ToFile(false), "back to directorysizes text")

	t.Run("skip not seen item when truncate on", func(t *testing.T) {
		got["foo"].Seen = true
		assert.Equal(t, "20000 1672531200 foo\n", got.ToFile(true))
	})
}
