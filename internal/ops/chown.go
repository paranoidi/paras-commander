package ops

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ChownPlan describes a validated chown operation.
type ChownPlan struct {
	Entries []localfs.Entry
	User    string // resolved username or empty
	Group   string // resolved group name or empty
	UID     int    // -1 means unchanged
	GID     int    // -1 means unchanged
}

// PlanChown validates a chown operation.
func PlanChown(source Source, user, group string) (ChownPlan, error) {
	if len(source.Entries) == 0 {
		return ChownPlan{}, &Error{Op: "chown", Text: "no entries to change"}
	}

	// Resolve user.
	var uid = -1
	if user != "" {
		var err error
		uid, err = localfs.LookupUser(user)
		if err != nil {
			return ChownPlan{}, &Error{Op: "chown", Text: "invalid user " + user, Err: err}
		}
	}

	// Resolve group.
	var gid = -1
	if group != "" {
		var err error
		gid, err = localfs.LookupGroup(group)
		if err != nil {
			return ChownPlan{}, &Error{Op: "chown", Text: "invalid group " + group, Err: err}
		}
	}

	return ChownPlan{
		Entries: source.Entries,
		User:    user,
		Group:   group,
		UID:     uid,
		GID:     gid,
	}, nil
}

// ExecuteChown applies the ownership change to each entry.
// Requires appropriate privileges.
func ExecuteChown(plan ChownPlan) error {
	for _, entry := range plan.Entries {
		if err := localfs.Chown(entry.Path, plan.UID, plan.GID); err != nil {
			return &Error{Op: "chown", Text: "failed to change owner for " + entry.Name, Err: err}
		}
	}
	return nil
}
