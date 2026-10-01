package test

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/xo/dbmeta"
)

// parityWalk asks a query that a walk answers, which has no one statement to
// run, through its typed iterator, and records the answer in the form
// parityAsk does. Each row is the value printed with %v. See D146 and D159.
type parityWalk func(ctx context.Context, m *dbmeta.Meta, db *sql.DB, args map[string]any) parityAnswer

// parityWalks holds a parityWalk for each query, by name.
var parityWalks = map[string]parityWalk{
	dbmeta.Tables.Name():                  walkAnswer(dbmeta.Tables),
	dbmeta.Schemas.Name():                 walkAnswer(dbmeta.Schemas),
	dbmeta.Columns.Name():                 walkAnswer(dbmeta.Columns),
	dbmeta.Indexes.Name():                 walkAnswer(dbmeta.Indexes),
	dbmeta.Databases.Name():               walkAnswer(dbmeta.Databases),
	dbmeta.Tablespaces.Name():             walkAnswer(dbmeta.Tablespaces),
	dbmeta.AccessMethods.Name():           walkAnswer(dbmeta.AccessMethods),
	dbmeta.Languages.Name():               walkAnswer(dbmeta.Languages),
	dbmeta.Conversions.Name():             walkAnswer(dbmeta.Conversions),
	dbmeta.Casts.Name():                   walkAnswer(dbmeta.Casts),
	dbmeta.Collations.Name():              walkAnswer(dbmeta.Collations),
	dbmeta.LargeObjects.Name():            walkAnswer(dbmeta.LargeObjects),
	dbmeta.EventTriggers.Name():           walkAnswer(dbmeta.EventTriggers),
	dbmeta.Settings.Name():                walkAnswer(dbmeta.Settings),
	dbmeta.Functions.Name():               walkAnswer(dbmeta.Functions),
	dbmeta.Aggregates.Name():              walkAnswer(dbmeta.Aggregates),
	dbmeta.Types.Name():                   walkAnswer(dbmeta.Types),
	dbmeta.Domains.Name():                 walkAnswer(dbmeta.Domains),
	dbmeta.Operators.Name():               walkAnswer(dbmeta.Operators),
	dbmeta.Roles.Name():                   walkAnswer(dbmeta.Roles),
	dbmeta.RoleSettings.Name():            walkAnswer(dbmeta.RoleSettings),
	dbmeta.RoleGrants.Name():              walkAnswer(dbmeta.RoleGrants),
	dbmeta.Privileges.Name():              walkAnswer(dbmeta.Privileges),
	dbmeta.DefaultACLs.Name():             walkAnswer(dbmeta.DefaultACLs),
	dbmeta.ForeignDataWrappers.Name():     walkAnswer(dbmeta.ForeignDataWrappers),
	dbmeta.ForeignServers.Name():          walkAnswer(dbmeta.ForeignServers),
	dbmeta.UserMappings.Name():            walkAnswer(dbmeta.UserMappings),
	dbmeta.ForeignTables.Name():           walkAnswer(dbmeta.ForeignTables),
	dbmeta.Publications.Name():            walkAnswer(dbmeta.Publications),
	dbmeta.PublicationTables.Name():       walkAnswer(dbmeta.PublicationTables),
	dbmeta.Subscriptions.Name():           walkAnswer(dbmeta.Subscriptions),
	dbmeta.TextSearchParsers.Name():       walkAnswer(dbmeta.TextSearchParsers),
	dbmeta.TextSearchDictionaries.Name():  walkAnswer(dbmeta.TextSearchDictionaries),
	dbmeta.TextSearchTemplates.Name():     walkAnswer(dbmeta.TextSearchTemplates),
	dbmeta.TextSearchConfigs.Name():       walkAnswer(dbmeta.TextSearchConfigs),
	dbmeta.TextSearchConfigMaps.Name():    walkAnswer(dbmeta.TextSearchConfigMaps),
	dbmeta.OperatorClasses.Name():         walkAnswer(dbmeta.OperatorClasses),
	dbmeta.OperatorFamilies.Name():        walkAnswer(dbmeta.OperatorFamilies),
	dbmeta.OperatorFamilyOperators.Name(): walkAnswer(dbmeta.OperatorFamilyOperators),
	dbmeta.OperatorFamilyFunctions.Name(): walkAnswer(dbmeta.OperatorFamilyFunctions),
	dbmeta.Extensions.Name():              walkAnswer(dbmeta.Extensions),
	dbmeta.ExtensionObjects.Name():        walkAnswer(dbmeta.ExtensionObjects),
	dbmeta.ExtendedStats.Name():           walkAnswer(dbmeta.ExtendedStats),
	dbmeta.Comments.Name():                walkAnswer(dbmeta.Comments),
	dbmeta.IndexColumns.Name():            walkAnswer(dbmeta.IndexColumns),
	dbmeta.Constraints.Name():             walkAnswer(dbmeta.Constraints),
	dbmeta.Triggers.Name():                walkAnswer(dbmeta.Triggers),
	dbmeta.Sequences.Name():               walkAnswer(dbmeta.Sequences),
	dbmeta.PartitionedTables.Name():       walkAnswer(dbmeta.PartitionedTables),
	dbmeta.ConstraintColumns.Name():       walkAnswer(dbmeta.ConstraintColumns),
	dbmeta.RoutineParameters.Name():       walkAnswer(dbmeta.RoutineParameters),
	dbmeta.EnumValues.Name():              walkAnswer(dbmeta.EnumValues),
	dbmeta.Views.Name():                   walkAnswer(dbmeta.Views),
	dbmeta.ColumnStats.Name():             walkAnswer(dbmeta.ColumnStats),
	dbmeta.CurrentSchema.Name():           walkAnswer(dbmeta.CurrentSchema),
	dbmeta.CurrentUser.Name():             walkAnswer(dbmeta.CurrentUser),
}

// walkAnswer returns the parityWalk of q.
func walkAnswer[T any](q *dbmeta.Query[T]) parityWalk {
	return func(ctx context.Context, m *dbmeta.Meta, db *sql.DB, args map[string]any) parityAnswer {
		var out []string
		for v, err := range q.All(ctx, m, db, args) {
			if err != nil {
				return parityAnswer{err: firstLine(err.Error())}
			}
			out = append(out, fmt.Sprintf("%v", v))
		}
		// Sorted, as parityAsk sorts.
		sort.Strings(out)
		return parityAnswer{rows: len(out), body: strings.Join(out, "\n")}
	}
}
