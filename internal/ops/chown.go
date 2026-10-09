package ops

import (
	"context"
	"errors"
	"strings"

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

	user = strings.TrimSpace(user)
	group = strings.TrimSpace(group)
	if user == "" && group == "" {
		return ChownPlan{}, &Error{Op: "chown", Text: "user or group is required"}
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

// ExecuteChown applies the ownership change to each entry. Changing the owner normally needs
// root, so a permission failure is retried as root when ctx carries an Elevator; entries the user
// skips are left unchanged.
func ExecuteChown(ctx context.Context, plan ChownPlan) error {
	for _, entry := range plan.Entries {
		err := withElevation(ctx, "chown", entry.Path, []string{"chown", chownArg(plan.UID, plan.GID), "--", entry.Path}, func() error {
			return localfs.Chown(entry.Path, plan.UID, plan.GID)
		})
		if errors.Is(err, ErrPermissionSkipped) {
			continue
		}
		if err != nil {
			if errors.Is(err, ErrUserCanceled) {
				return err
			}
			return &Error{Op: "chown", Text: "failed to change owner for " + entry.Name, Err: err}
		}
	}
	return nil
}
