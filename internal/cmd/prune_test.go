package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umlx5h/gtrash/internal/trash"
)

func newInt(i int64) *int64 {
	return &i
}

func TestGetPruneFiles(t *testing.T) {
	t.Run("should return prune files", func(t *testing.T) {
		got, deleted, total := getPruneFiles([]trash.File{
			{
				Name: "a",
				Size: newInt(20),
			},
			{
				Name: "b",
				Size: newInt(30),
			},
			{
				Name: "c",
				Size: newInt(50),
			},
			{
				Name: "d",
				Size: newInt(100),
			},
			{
				Name: "e",
				Size: newInt(150),
			},
		}, 100)

		want := []trash.File{
			{
				Name: "d",
				Size: newInt(100),
			},
			{
				Name: "e",
				Size: newInt(150),
			},
		}

		assert.Equal(t, want, got)
		assert.EqualValues(t, 250, deleted)
		assert.EqualValues(t, 350, total)
	})

	t.Run("should prune files from larger files", func(t *testing.T) {
		got, deleted, total := getPruneFiles([]trash.File{
			{
				Name: "a",
				Size: newInt(20),
			},
			{
				Name: "b",
				Size: newInt(30),
			},
			{
				Name: "c",
				Size: newInt(50),
			},
		}, 30)

		want := []trash.File{
			{
				Name: "b",
				Size: newInt(30),
			},
			{
				Name: "c",
				Size: newInt(50),
			},
		}

		assert.Equal(t, want, got)
		assert.EqualValues(t, 80, deleted)
		assert.EqualValues(t, 100, total)
	})

	t.Run("should return nil", func(t *testing.T) {
		got, deleted, total := getPruneFiles([]trash.File{
			{
				Name: "a",
				Size: newInt(20),
			},
			{
				Name: "b",
				Size: newInt(30),
			},
			{
				Name: "c",
				Size: newInt(50),
			},
		}, 100)

		assert.Nil(t, got)
		assert.EqualValues(t, 0, deleted)
		assert.EqualValues(t, 100, total)
	})

}
