package main

// operatorctl — the server-side CLI of the Vihat operator realm (ADR 0048 §Chốt bước 1,
// 28/09/2026): devops creates operator accounts, grants and revokes `ops.*`, disables and
// enables, resets MFA, and lists. There is deliberately no screen and no permission for any of
// this — adding one is ADR 0048 stop condition #1.
//
// RUN WHERE THE IDENTITY SERVICE RUNS, WITH ITS ENVIRONMENT. Configuration comes from core/config
// exactly as the service binary reads it (rule 11: nothing else reads the environment), so
// DATABASE_DSN is the identity service's own database and nobody else's (rule 2).
//
// IT NEVER MIGRATES. The schema belongs to the service's start-up (step 2b of its main); a second,
// unscheduled path changing a live schema is not something a CLI gets to be. A database without
// migration 0012 is refused with one sentence.
//
// SECRETS GO TO STDOUT, ONCE, AND NOWHERE ELSE. A temporary password is printed on stdout for the
// person at the terminal to hand over; it is never logged (the security log goes to stderr and
// carries events, never values — rule 3, rule 8). Every mutating command demands --ticket: the
// trail records actor = system and reason = ticket:<n>, and without the ticket "system" names
// nobody.
//
// Enrolment (TOTP + own password) is NOT a CLI job: the operator does it at the first sign-in
// (ADR 0048 step 3).

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

const usage = `operatorctl — operator accounts of the Vihat operator realm (ADR 0048)

  operatorctl create    --email <addr> --name <display name> --ticket <n>
  operatorctl grant     --code VH-00001 --permission ops.<key> --ticket <n>
  operatorctl revoke    --code VH-00001 --permission ops.<key> --ticket <n>
  operatorctl disable   --code VH-00001 --reason <text> --ticket <n>
  operatorctl enable    --code VH-00001 --reason <text> --ticket <n>
  operatorctl unlock    --code VH-00001 --ticket <n>      lift a failure lockout (12 h) early
  operatorctl reset-mfa --code VH-00001 --ticket <n>      also reissues an expired temporary password
  operatorctl list

Reads the identity service's environment (DATABASE_DSN, ENV, ...). Does not run migrations.
`

// Exit codes: 0 done, 1 the act failed, 2 the command line is wrong.
const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command is one parsed invocation.
type command struct {
	verb        string
	email       string
	displayName string
	code        string
	permission  string
	reason      string
	ticket      string
}

// errUsage wraps every command-line refusal, so run can answer exit code 2 and the usage text.
var errUsage = errors.New("usage")

// parseArgs validates the command line BEFORE any configuration is read or connection opened: a
// mutating command without its ticket is refused without touching the database.
func parseArgs(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, fmt.Errorf("%w: no command", errUsage)
	}
	c := command{verb: args[0]}
	fs := flag.NewFlagSet(c.verb, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // errors are reported once, by run, with the usage text

	var required []string
	switch c.verb {
	case "create":
		fs.StringVar(&c.email, "email", "", "")
		fs.StringVar(&c.displayName, "name", "", "")
		required = []string{"email", "name", "ticket"}
	case "grant", "revoke":
		fs.StringVar(&c.code, "code", "", "")
		fs.StringVar(&c.permission, "permission", "", "")
		required = []string{"code", "permission", "ticket"}
	case "disable", "enable":
		fs.StringVar(&c.code, "code", "", "")
		fs.StringVar(&c.reason, "reason", "", "")
		required = []string{"code", "reason", "ticket"}
	case "reset-mfa", "unlock":
		fs.StringVar(&c.code, "code", "", "")
		required = []string{"code", "ticket"}
	case "list":
	default:
		return command{}, fmt.Errorf("%w: unknown command %q", errUsage, c.verb)
	}
	if c.verb != "list" {
		fs.StringVar(&c.ticket, "ticket", "", "")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return command{}, fmt.Errorf("%w: %s: %v", errUsage, c.verb, err)
	}
	if fs.NArg() > 0 {
		return command{}, fmt.Errorf("%w: %s: unexpected argument %q", errUsage, c.verb, fs.Arg(0))
	}
	values := map[string]string{"email": c.email, "name": c.displayName, "code": c.code,
		"permission": c.permission, "reason": c.reason, "ticket": c.ticket}
	for _, f := range required {
		if strings.TrimSpace(values[f]) == "" {
			return command{}, fmt.Errorf("%w: %s: --%s is required", errUsage, c.verb, f)
		}
	}
	return c, nil
}

// admin is what the commands call — app.OperatorAdmin, or a fake in tests.
type admin interface {
	CreateOperator(ctx context.Context, email, displayName, ticket string) (app.OperatorCreated, error)
	Grant(ctx context.Context, code, permission, ticket string) error
	Revoke(ctx context.Context, code, permission, ticket string) error
	Disable(ctx context.Context, code, reason, ticket string) error
	Enable(ctx context.Context, code, reason, ticket string) error
	Unlock(ctx context.Context, code, ticket string) error
	ResetMFA(ctx context.Context, code, ticket string) (secret.Secret, error)
	List(ctx context.Context) ([]app.OperatorListing, error)
}

// execute runs one parsed command and writes its result to stdout.
func execute(ctx context.Context, c command, a admin, stdout io.Writer) error {
	switch c.verb {
	case "create":
		res, err := a.CreateOperator(ctx, c.email, c.displayName, c.ticket)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created %s\n", res.Code)
		printOnce(stdout, res.TemporaryPassword)
	case "grant":
		if err := a.Grant(ctx, c.code, c.permission, c.ticket); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "granted %s to %s — the operator's sessions were revoked\n", c.permission, c.code)
	case "revoke":
		if err := a.Revoke(ctx, c.code, c.permission, c.ticket); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "revoked %s from %s — the operator's sessions were revoked\n", c.permission, c.code)
	case "disable":
		if err := a.Disable(ctx, c.code, c.reason, c.ticket); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "disabled %s — every session revoked\n", c.code)
	case "enable":
		if err := a.Enable(ctx, c.code, c.reason, c.ticket); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "enabled %s — a failure lockout, if any, is NOT lifted (use unlock)\n", c.code)
	case "unlock":
		if err := a.Unlock(ctx, c.code, c.ticket); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "unlocked %s — failure lockout lifted, failure count reset\n", c.code)
	case "reset-mfa":
		temp, err := a.ResetMFA(ctx, c.code, c.ticket)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "MFA reset for %s — TOTP removed, recovery codes voided, sessions revoked\n", c.code)
		printOnce(stdout, temp)
	case "list":
		// @cross-tenant: the operator register (ADR 0048 §Chốt #1) belongs to no commune; this
		// lists Vihat operator accounts, joins no table that carries tenant_id, emails masked.
		rows, err := a.List(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "CODE\tEMAIL\tNAME\tSTATUS\tPERMISSIONS")
		for _, r := range rows {
			perms := make([]string, 0, len(r.Permissions))
			for _, p := range r.Permissions {
				perms = append(perms, string(p))
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Code, r.MaskedEmail, r.DisplayName, r.Status, strings.Join(perms, ","))
		}
		return w.Flush()
	default:
		return fmt.Errorf("%w: unknown command %q", errUsage, c.verb)
	}
	return nil
}

// printOnce writes a temporary password to stdout — the one place it ever appears in the clear.
// .Lo() here and nowhere else: secret.Secret renders as *** on every other path.
func printOnce(stdout io.Writer, temp secret.Secret) {
	fmt.Fprintf(stdout, "temporary password (shown ONCE, not stored — hand it over; the operator "+
		"must change it and enrol TOTP at first sign-in):\n%s\n", temp.Lo())
}

func run(args []string, stdout, stderr io.Writer) int {
	c, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "operatorctl: %v\n\n%s", err, usage)
		return exitUsage
	}

	cfg, err := config.Load("identity")
	if err != nil {
		fmt.Fprintf(stderr, "operatorctl: configuration: %v\n", err)
		return exitFail
	}
	// .Lo() is the one place the DSN leaves secret.DSN with its password intact, as in the
	// service's own main: the driver argument, and nowhere else.
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		fmt.Fprintf(stderr, "operatorctl: open database: %v\n", err)
		return exitFail
	}
	defer db.Close()
	db.SetMaxOpenConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(stderr, "operatorctl: cannot reach the identity database: %v\n", err)
		return exitFail
	}
	store := operatorstore.New(db)
	if err := store.SchemaReady(ctx); err != nil {
		if errors.Is(err, operatorstore.ErrSchemaMissing) {
			fmt.Fprintln(stderr, "operatorctl: the operator tables do not exist in this database — "+
				"migration 0012 has not been applied. Start (or upgrade) the identity service, which "+
				"applies its migrations at start-up; this tool never migrates.")
			return exitFail
		}
		fmt.Fprintf(stderr, "operatorctl: %v\n", err)
		return exitFail
	}

	// The security log (events, never values) goes to stderr, so stdout holds only the result.
	log := slog.New(slog.NewJSONHandler(stderr, nil))
	a := app.NewOperatorAdmin(app.NewOperatorStore(store), time.Now, log)
	if err := execute(ctx, c, a, stdout); err != nil {
		fmt.Fprintf(stderr, "operatorctl: %s: %v\n", c.verb, err)
		return exitFail
	}
	return exitOK
}
