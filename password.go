package dbmeta

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Changing a password, which is the one statement here that does not read.
//
// # Why it is here at all
//
// D5 says dbmeta reads. This does not break that and it comes close enough to
// need saying why. Nothing here executes the statement. [Dialect.ChangePasswordSQL]
// returns text and takes no database, so a caller still hands dbmeta a read
// only connection for everything dbmeta runs. The statement is the caller's to
// execute, or not.
//
// What moved here is the knowledge rather than the authority. The statement
// differs per product, and so does the rule for putting a password into it,
// and both are exactly the kind of per product knowledge this module exists to
// hold in one reviewed place. usql carried seven copies and escaped nothing at
// all, so the status quo was not safe elsewhere. See D56.
//
// # Why a password cannot be a parameter
//
// It cannot be bound. PostgreSQL refuses to prepare the statement at all:
//
//	PREPARE t AS ALTER USER postgres PASSWORD $1
//	ERROR:  syntax error at or near "ALTER"
//
// and SQL Server refuses @p in ALTER LOGIN. So the password goes into the text
// and the escaping has to be right.
//
// # Why escaping needs the session
//
// There is no fixed rule. Whether a backslash escapes the next character
// inside a string literal is a server setting: sql_mode on MySQL and MariaDB,
// standard_conforming_strings on PostgreSQL. Getting it wrong is not a mangled
// password, it is the literal swallowing the rest of the statement. With
// quote doubling alone and the password x\ a real MariaDB was sent
//
//	CREATE USER t3@'%' IDENTIFIED BY 'x\';
//	SELECT 'statement completed' AS result;
//
// and the backslash escaped the closing quote, so the literal ate the
// semicolon and the line after it and the second statement never ran.
//
// [Quoting] carries that state and a product that needs it refuses rather than
// guesses. The caller reads it with [Dialect.Quoting], which is a read like
// any other here.

// PasswordChange is a request to set a password.
type PasswordChange struct {
	// User is the role, user or login whose password changes.
	User string
	// Password is the new password.
	Password string

	// Old is the current password. SQL Server and Oracle can require it and
	// every other product ignores it.
	Old string
}

// Quoting is the session state that decides how a string literal is escaped.
//
// A field is absent when the caller has not read it. A product that needs a
// field and does not get it returns [ErrQuotingUnknown], because the safe
// default is different per product and guessing is how the demonstration above
// happens.
type Quoting struct {
	// BackslashEscapes reports whether a backslash escapes the next character
	// inside a string literal.
	//
	// It is sql_mode not containing NO_BACKSLASH_ESCAPES on MySQL and MariaDB,
	// and standard_conforming_strings being off on PostgreSQL. SQL Server has
	// no such setting and leaves this absent.
	BackslashEscapes sql.Null[bool]
}

// QuotingQuery returns the statement that reads the session state
// [Dialect.ChangePasswordSQL] needs, and how many columns it returns.
//
// The result is false when the product needs none, which is not an error: SQL
// Server has no setting that changes how a literal is escaped.
func (d Dialect) QuotingQuery() (sqlstr string, cols int, ok bool) {
	info, found := d.Info()
	if !found || info.QuotingSQL == "" {
		return "", 0, false
	}
	return info.QuotingSQL, info.QuotingColumns, true
}

// ParseQuoting turns the columns returned by [Dialect.QuotingQuery] into the
// state.
func (d Dialect) ParseQuoting(cols []string) (Quoting, error) {
	info, ok := d.Info()
	if !ok {
		return Quoting{}, ErrModelNotBuilt
	}
	if info.ParseQuoting == nil {
		return Quoting{}, nil
	}
	return info.ParseQuoting(cols)
}

// Quoting reads the session state from the server.
//
// This is a read and it runs a query, the same way [Dialect.Version] does. A
// caller that would rather run the statement itself uses [Dialect.QuotingQuery]
// and [Dialect.ParseQuoting].
//
// A product with no such state returns the zero value and no error.
func (d Dialect) Quoting(ctx context.Context, db Querier) (Quoting, error) {
	sqlstr, n, ok := d.QuotingQuery()
	if !ok {
		if _, built := d.Info(); !built {
			return Quoting{}, ErrModelNotBuilt
		}
		return Quoting{}, nil
	}
	cols, err := readRow(ctx, db, d, sqlstr, n)
	if err != nil {
		return Quoting{}, err
	}
	return d.ParseQuoting(cols)
}

// ChangePasswordSQL returns the statement that sets the password, and does not
// run it.
//
// It takes no database on purpose. dbmeta executes reads and this is not one,
// so the caller runs it, logs what it chooses to log, and decides what to do
// with the text afterwards. The text contains the password in clear, because
// the server requires that, so a caller must keep it out of its logs.
//
// It returns [ErrNotSupported] when the product has no such statement or the
// model is not built, and [ErrQuotingUnknown] when the product needs session
// state that [Quoting] does not carry.
func (d Dialect) ChangePasswordSQL(c PasswordChange, q Quoting) (string, error) {
	info, ok := d.Info()
	if !ok {
		return "", ErrModelNotBuilt
	}
	if info.ChangePassword == nil {
		return "", ErrNotSupported
	}
	if c.User == "" {
		return "", fmt.Errorf("changing a password: %w", ErrMissingParam)
	}
	// A NUL cannot appear in a string literal on any product here, and a
	// password carrying one is a caller fault rather than a quoting problem.
	for _, s := range []string{c.User, c.Password, c.Old} {
		if strings.ContainsRune(s, 0) {
			return "", ErrInvalidPassword
		}
	}
	return info.ChangePassword(c, q)
}

// QuoteLiteral returns s as a single quoted SQL string literal.
//
// It doubles the quote always, and doubles a backslash when the session says a
// backslash escapes. That second part is why this takes the state rather than
// being a plain function: on MySQL and MariaDB the answer changes with
// sql_mode, and on PostgreSQL with standard_conforming_strings.
//
// It is exported because a model outside this repository needs the same rule,
// and because the rule is the thing worth reviewing once.
func QuoteLiteral(s string, q Quoting) string {
	s = strings.ReplaceAll(s, "'", "''")
	if q.BackslashEscapes.Valid && q.BackslashEscapes.V {
		s = strings.ReplaceAll(s, `\`, `\\`)
	}
	return "'" + s + "'"
}

// QuoteIdentifier returns name quoted with the pair given, doubling the
// closing character inside it.
//
// PostgreSQL passes a double quote for both, SQL Server passes a bracket pair,
// and MySQL passes a backtick. Doubling the closer is what every one of them
// specifies.
func QuoteIdentifier(name, open, closed string) string {
	return open + strings.ReplaceAll(name, closed, closed+closed) + closed
}
