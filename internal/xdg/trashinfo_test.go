package xdg

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInfoSuccess(t *testing.T) {
	wantInfo := Info{
		Path: "/dummy",
	}
	date, err := time.ParseInLocation(timeFormat, "2023-01-01T00:00:00", time.Local)
	if err != nil {
		panic(err)
	}
	wantInfo.DeletionDate = date

	t.Run("normal", func(t *testing.T) {
		info, err := NewInfo(strings.NewReader(`[Trash Info]
Path=/dummy
DeletionDate=2023-01-01T00:00:00
`))
		require.NoError(t, err)
		assert.Equal(t, wantInfo, info)
	})

	t.Run("ignore_comment_and_blankline", func(t *testing.T) {
		info, err := NewInfo(strings.NewReader(`# comment 1
[Trash Info]

Path=/dummy

# comment 2

DeletionDate=2023-01-01T00:00:00
`))
		require.NoError(t, err)
		assert.Equal(t, wantInfo, info)
	})

	t.Run("contain_space_between_key_value", func(t *testing.T) {
		info, err := NewInfo(strings.NewReader(`[Trash Info]
DeletionDate = 2023-01-01T00:00:00
Path = /dummy`))
		require.NoError(t, err)
		assert.Equal(t, wantInfo, info)
	})

	// xdg ref: If a string that starts with “Path=” or “DeletionDate=” occurs
	// several times, the first occurence is to be used.
	t.Run("high_priority_to_first_key_pair", func(t *testing.T) {
		info, err := NewInfo(strings.NewReader(`[Trash Info]
Path=/dummy
DeletionDate=2023-01-01T00:00:00
DeletionDate=2099-01-01T00:00:00
Path=/notused
`))
		require.NoError(t, err)
		assert.Equal(t, wantInfo, info)
	})
}

func TestNewInfoError(t *testing.T) {
	t.Run("detect_other_group", func(t *testing.T) {
		_, err := NewInfo(strings.NewReader(`[Trash Info]
Path=/dummy
[dummy group]
DeletionDate=2023-01-01T00:00:00
`))
		require.Error(t, err)
	})
}

func TestQueryEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/foo/bar", "/foo/bar"},
		{"/foo/foo bar", "/foo/foo%20bar"},
		{"/foo/b  a  r", "/foo/b%20%20a%20%20r"},
		{"/foo/あ い", "/foo/%E3%81%82%20%E3%81%84"},
		{"/foo/mycool+blog&about,stuff", "/foo/mycool%2Bblog%26about%2Cstuff"},
	}

	t.Run("escape", func(t *testing.T) {
		for _, tt := range tests {
			assert.Equal(t, tt.want, queryEscapePath(tt.input))
		}
	})

	t.Run("unescape", func(t *testing.T) {
		for _, tt := range tests {
			e, err := url.QueryUnescape(tt.want)
			require.NoError(t, err)
			assert.Equal(t, e, tt.input)
		}
	})
}
