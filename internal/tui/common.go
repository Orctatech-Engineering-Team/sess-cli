package tui

import (
	"context"
	"sync"

	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

// gitLineMsg represents a line of git output (stdout)
type gitLineMsg string

// gitErrLineMsg represents a line from stderr
type gitErrLineMsg string

// gitErrMsg represents a fatal error
type gitErrMsg struct{ error }

// gitSuccessMsg indicates the git command completed successfully
type gitSuccessMsg struct{}

// streamStep runs a git command, streams logs/errors into Update, then calls next if success
func streamStep(p *tea.Program, dir string, args []string, next func()) {
	ctx := context.Background()

	// Get the output channels
	channels := git.RunGitWithOutput(ctx, dir, args...)

	// Use WaitGroup to ensure all channels are fully read
	var wg sync.WaitGroup
	wg.Add(3) // stdout, stderr, and err channels

	// Forward stdout lines
	go func() {
		defer wg.Done()
		for line := range channels.Stdout {
			p.Send(gitLineMsg(line))
		}
	}()

	// Forward stderr lines (these are informational, not errors)
	go func() {
		defer wg.Done()
		for line := range channels.Stderr {
			p.Send(gitErrLineMsg(line))
		}
	}()

	// Handle the final error/success
	go func() {
		defer wg.Done()

		// Wait for the error channel (will receive nil on success, error on failure)
		err := <-channels.Err

		if err != nil {
			// Command failed
			p.Send(gitErrMsg{err})
			return
		}

		// Command succeeded - call next step if provided
		if next != nil {
			next()
		} else {
			// Or send success message
			p.Send(gitSuccessMsg{})
		}
	}()
}

func streamStepSimple(p *tea.Program, dir string, args []string, next func()) {
	ctx := context.Background()
	channels := git.RunGitWithOutput(ctx, dir, args...)

	// Combine stdout and stderr into one stream
	go func() {
		for {
			select {
			case line, ok := <-channels.Stdout:
				if !ok {
					channels.Stdout = nil
					continue
				}
				p.Send(gitLineMsg(line))
			case line, ok := <-channels.Stderr:
				if !ok {
					channels.Stderr = nil
					continue
				}
				p.Send(gitLineMsg("[stderr] " + line))
			}

			// Exit when both channels closed
			if channels.Stdout == nil && channels.Stderr == nil {
				break
			}
		}
	}()

	// Handle completion
	go func() {
		err := <-channels.Err
		if err != nil {
			p.Send(gitErrMsg{err})
			return
		}
		if next != nil {
			next()
		} else {
			p.Send(gitSuccessMsg{})
		}
	}()
}

func streamStepWithCancel(p *tea.Program, ctx context.Context, dir string, args []string, next func()) {
	channels := git.RunGitWithOutput(ctx, dir, args...)

	var wg sync.WaitGroup
	wg.Add(3)

	// Stdout
	go func() {
		defer wg.Done()
		for {
			select {
			case line, ok := <-channels.Stdout:
				if !ok {
					return
				}
				p.Send(gitLineMsg(line))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Stderr
	go func() {
		defer wg.Done()
		for {
			select {
			case line, ok := <-channels.Stderr:
				if !ok {
					return
				}
				p.Send(gitErrLineMsg(line))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Error handling
	go func() {
		defer wg.Done()

		select {
		case err := <-channels.Err:
			if err != nil {
				p.Send(gitErrMsg{err})
				return
			}
			if next != nil {
				next()
			} else {
				p.Send(gitSuccessMsg{})
			}
		case <-ctx.Done():
			p.Send(gitErrMsg{ctx.Err()})
		}
	}()
}

func performGitWorkflow(p *tea.Program, repoDir string) {
	// Step 1: Fetch
	streamStep(p, repoDir, []string{"fetch", "origin", "main"}, func() {
		// Step 2: Rebase (runs after fetch succeeds)
		streamStep(p, repoDir, []string{"rebase", "origin/main"}, func() {
			// Step 3: Push (runs after rebase succeeds)
			streamStep(p, repoDir, []string{"push", "origin", "HEAD"}, func() {
				// All done!
				p.Send(gitLineMsg("✓ Workflow complete!"))
			})
		})
	})
}
