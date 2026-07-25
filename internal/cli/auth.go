package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/iftimiemarius/dispatch/internal/config"
	"github.com/iftimiemarius/dispatch/internal/graph"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage integration authentication (Outlook)",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthLogoutCmd(), newAuthStatusCmd())
	return cmd
}

// loadOutlookConfig resolves the config file and returns an Authenticator if
// Outlook is configured, else returns a helpful error.
func loadOutlookConfig() (*config.Config, *graph.Authenticator, error) {
	paths := configPathsOrResolve()
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	if !cfg.OutlookEnabled() {
		return nil, nil, fmt.Errorf("Outlook not configured: set [outlook] client_id in %s\nSee the README for the Azure app registration steps", paths.ConfigFile)
	}
	a := graph.NewAuthenticator(cfg.Outlook.ClientID, cfg.Outlook.Tenant, cfg.Outlook.RedirectPort)
	return cfg, a, nil
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login outlook",
		Short: "Connect your Microsoft account (Outlook calendar)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "outlook" {
				return fmt.Errorf("only 'outlook' is supported")
			}
			_, a, err := loadOutlookConfig()
			if err != nil {
				return err
			}
			return runOutlookLogin(cmd, a)
		},
	}
}

// runOutlookLogin performs the OAuth Authorization Code + PKCE flow: it starts
// a local HTTP server to catch the redirect, opens the browser, and exchanges
// the code for tokens.
func runOutlookLogin(cmd *cobra.Command, a *graph.Authenticator) error {
	out := cmd.OutOrStdout()
	verifier, err := graph.PKCEVerifier()
	if err != nil {
		return err
	}
	state, err := graph.RandState()
	if err != nil {
		return err
	}
	authURL := a.LoginURL(state, verifier)

	// Channel to receive the code (or an error) from the callback handler.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			errCh <- fmt.Errorf("auth error: %s — %s", e, q.Get("error_description"))
			fmt.Fprintf(w, "Authentication failed. Return to your terminal.")
			return
		}
		if q.Get("state") != state {
			errCh <- fmt.Errorf("state mismatch — possible CSRF, aborting")
			fmt.Fprintf(w, "State mismatch. Authentication aborted.")
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code in callback")
			fmt.Fprintf(w, "No code received.")
			return
		}
		codeCh <- code
		// Friendly success page.
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body style="font-family:sans-serif;padding:2em">
<h2>Dispatch connected ✓</h2>
<p>You can close this tab and return to your terminal.</p></body></html>`)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", a.RedirectPort),
		Handler: mux,
	}
	go func() { _ = srv.ListenAndServe() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	fmt.Fprintln(out, ui.Bold("Connect your Microsoft account"))
	fmt.Fprintf(out, "  Open this URL in your browser:\n\n    %s\n\n", authURL)
	fmt.Fprintf(out, "  Waiting for login on port %d ...\n", a.RedirectPort)
	fmt.Fprintln(out, ui.Dim("  (Ctrl+C to cancel)"))

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timed out waiting for login")
	}

	if err := a.Exchange(context.Background(), code, verifier); err != nil {
		return err
	}
	fmt.Fprintln(out, ui.Green("  ✓ Outlook connected."))
	return nil
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout outlook",
		Short: "Disconnect your Microsoft account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "outlook" {
				return fmt.Errorf("only 'outlook' is supported")
			}
			if err := graph.Logout(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("  Outlook disconnected."))
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show which integrations are authenticated",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			outlook := "disconnected"
			if graph.HasToken() {
				outlook = ui.Green("connected")
			}
			fmt.Fprintf(out, "outlook: %s\n", outlook)
			return nil
		},
	}
}

// configPathsOrResolve returns the paths from context if available, else
// resolves fresh. Used by auth commands that run with skip_db (no paths in ctx).
func configPathsOrResolve() *config.Paths {
	if p, ok := pathsFromContext(rootContext()); ok {
		return p
	}
	p, err := config.Resolve()
	if err != nil {
		return &config.Paths{ConfigFile: ""}
	}
	return p
}

// rootContext returns a background context; auth commands set skip_db so the
// root's PersistentPreRunE never attached one. pathsFromContext on a background
// ctx simply returns false, so we fall back to resolving.
func rootContext() context.Context { return context.Background() }
