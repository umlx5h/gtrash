package trash

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/maps"
)

func FlagCompletionFunc(allCompletions []string) func(*cobra.Command, []string, string) (
	[]string, cobra.ShellCompDirective,
) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var completions []string
		for _, completion := range allCompletions {
			if strings.HasPrefix(completion, toComplete) {
				completions = append(completions, completion)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// --sort, -s

var (
	sortByWellKnownStrings = map[string]SortByType{
		"date": SortByDeletedAt,
		"size": SortBySize,
		"name": SortByName,
	}

	SortByFlagCompletionFunc = FlagCompletionFunc(
		maps.Keys(sortByWellKnownStrings),
	)
)

func (s *SortByType) Set(str string) error {
	if value, ok := sortByWellKnownStrings[strings.ToLower(str)]; ok {
		*s = value
		return nil
	}

	return fmt.Errorf("must be %s", s.Type())
}

func (s SortByType) String() string {
	switch s {
	case SortByDeletedAt:
		return "date"
	case SortBySize:
		return "size"
	case SortByName:
		return "name"
	default:
		panic("invalid SortByType value")
	}
}

func (s SortByType) Type() string {
	return "date|size|name"
}

// --mode, -m

var _ pflag.Value = (*ModeByType)(nil)

type ModeByType int

const (
	ModeByRegex ModeByType = iota
	ModeByGlob             // default
	ModeByLiteral
	ModeByFull
)

var (
	modeByWellKnownStrings = map[string]ModeByType{
		"regex":   ModeByRegex,
		"glob":    ModeByGlob,
		"literal": ModeByLiteral,
		"full":    ModeByFull,
	}

	ModeByFlagCompletionFunc = FlagCompletionFunc(
		maps.Keys(modeByWellKnownStrings),
	)
)

func (s *ModeByType) Set(str string) error {
	if value, ok := modeByWellKnownStrings[strings.ToLower(str)]; ok {
		*s = value
		return nil
	}

	return fmt.Errorf("must be %s", s.Type())
}

func (s ModeByType) String() string {
	switch s {
	case ModeByGlob:
		return "glob"
	case ModeByRegex:
		return "regex"
	case ModeByLiteral:
		return "literal"
	case ModeByFull:
		return "full"
	default:
		panic("invalid ModeByType value")
	}
}

func (s ModeByType) Type() string {
	return "regex|glob|literal|full"
}
