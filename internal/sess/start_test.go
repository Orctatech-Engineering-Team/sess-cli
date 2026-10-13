package sess

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestStartInvalidArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "adopt with feature name",
			args: []string{"--adopt", "feature/existing", "new-feature"},
			want: "--adopt cannot be combined with a feature name",
		},
		{
			name: "feature name before adopt",
			args: []string{"new-feature", "--adopt=feature/existing"},
			want: "--adopt cannot be combined with a feature name",
		},
		{
			name: "extra normal arguments",
			args: []string{"one", "two"},
			want: "accepts at most 1 arg(s)",
		},
		{
			name: "extra adopt arguments",
			args: []string{"--adopt", "existing", "one", "two"},
			want: "accepts at most 1 arg(s)",
		},
		{
			name: "empty adopt equals",
			args: []string{"--adopt="},
			want: "--adopt requires a non-empty branch name",
		},
		{
			name: "empty adopt value",
			args: []string{"--adopt", ""},
			want: "--adopt requires a non-empty branch name",
		},
		{
			name: "missing adopt value",
			args: []string{"--adopt"},
			want: "flag needs an argument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := freshStartTestCommand()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestStartValidArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantAdopt bool
		branch    string
	}{
		{name: "interactive normal mode", args: []string{}},
		{name: "named normal mode", args: []string{"new-feature"}},
		{
			name:      "adopt separate value",
			args:      []string{"--adopt", "feature/existing"},
			wantAdopt: true,
			branch:    "feature/existing",
		},
		{
			name:      "adopt equals value",
			args:      []string{"--adopt=MixedCase/existing"},
			wantAdopt: true,
			branch:    "MixedCase/existing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := freshStartTestCommand()
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Args(cmd, cmd.Flags().Args()); err != nil {
				t.Fatalf("Args() error = %v", err)
			}
			if got := cmd.Flags().Changed("adopt"); got != tt.wantAdopt {
				t.Fatalf("adopt mode = %v, want %v", got, tt.wantAdopt)
			}
			branch, err := cmd.Flags().GetString("adopt")
			if err != nil {
				t.Fatal(err)
			}
			if branch != tt.branch {
				t.Fatalf("adopt branch = %q, want %q", branch, tt.branch)
			}
		})
	}
}

func freshStartTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           startCmd.Use,
		Args:          startCmd.Args,
		RunE:          startCmd.RunE,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().String("adopt", "", "")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}
