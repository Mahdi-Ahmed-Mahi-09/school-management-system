package memrepo

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
)

// ActivityRepo is an in-memory repository.ActivityRepository.
type ActivityRepo struct {
	mu      sync.Mutex
	entries []storedActivity
	// Fail makes Record return an error, to test that logging failures are non-fatal.
	Fail error
}

type storedActivity struct {
	id int64
	models.ActivityEntry
	createdAt time.Time
}

var _ repository.ActivityRepository = (*ActivityRepo)(nil)

// NewActivityRepo creates an empty ActivityRepo.
func NewActivityRepo() *ActivityRepo { return &ActivityRepo{} }

func (r *ActivityRepo) Record(_ context.Context, e models.ActivityEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Fail != nil {
		return r.Fail
	}
	r.entries = append(r.entries, storedActivity{id: int64(len(r.entries) + 1), ActivityEntry: e, createdAt: time.Now()})
	return nil
}

// Entries returns a copy of the recorded entries, oldest first.
func (r *ActivityRepo) Entries() []models.ActivityEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.ActivityEntry, len(r.entries))
	for i, e := range r.entries {
		out[i] = e.ActivityEntry
	}
	return out
}

// DashboardRepo computes the dashboard snapshot from the in-memory repos.
type DashboardRepo struct {
	Users    *UserRepo
	Activity *ActivityRepo
}

var _ repository.DashboardRepository = (*DashboardRepo)(nil)

func (r *DashboardRepo) AdminSnapshot(_ context.Context, q repository.DashboardQuery) (*repository.DashboardSnapshot, error) {
	loc, err := time.LoadLocation(q.Timezone)
	if err != nil {
		return nil, err
	}
	current := q.MonthStarts[len(q.MonthStarts)-1]
	first := q.MonthStarts[0]

	snap := &repository.DashboardSnapshot{RecentActivity: []models.ActivityItem{}}
	snap.Accounts.ByRole = map[models.Role]models.RoleMetric{}
	for _, role := range models.Roles {
		snap.Accounts.ByRole[role] = models.RoleMetric{}
	}
	points := make([]models.AccountGrowthPoint, len(q.MonthStarts))
	index := map[string]int{}
	for i, m := range q.MonthStarts {
		points[i].Month = m.Format("2006-01")
		index[points[i].Month] = i
	}

	r.Users.mu.Lock()
	byID := map[string]*models.User{}
	for _, u := range r.Users.users {
		byID[u.ID.String()] = u
		m := snap.Accounts.ByRole[u.Role]
		m.Total++
		if u.IsActive() {
			m.Active++
		} else {
			m.Inactive++
		}
		if !u.CreatedAt.Before(current) {
			m.NewThisMonth++
		}
		snap.Accounts.ByRole[u.Role] = m
		if !u.CreatedAt.Before(first) {
			if i, ok := index[u.CreatedAt.In(loc).Format("2006-01")]; ok {
				switch u.Role {
				case models.RoleAdmin:
					points[i].Admins++
				case models.RoleTeacher:
					points[i].Teachers++
				case models.RoleStudent:
					points[i].Students++
				case models.RoleGuardian:
					points[i].Guardians++
				}
			}
		}
	}
	r.Users.mu.Unlock()
	for _, m := range snap.Accounts.ByRole {
		snap.Accounts.Total += m.Total
		snap.Accounts.Active += m.Active
		snap.Accounts.Inactive += m.Inactive
		snap.Accounts.NewThisMonth += m.NewThisMonth
	}
	snap.AccountGrowth = points

	r.Activity.mu.Lock()
	entries := append([]storedActivity{}, r.Activity.entries...)
	r.Activity.mu.Unlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].id > entries[j].id })
	for i, e := range entries {
		if i >= q.ActivityLimit {
			break
		}
		meta := map[string]any{}
		for k, v := range e.Metadata {
			meta[k] = v
		}
		item := models.ActivityItem{ID: e.id, Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID, Summary: e.Summary, Metadata: meta, CreatedAt: e.createdAt}
		if e.ActorID != nil {
			if a, ok := byID[e.ActorID.String()]; ok {
				item.Actor = &models.ActivityActor{Name: a.FullName(), Role: a.Role}
			}
		}
		snap.RecentActivity = append(snap.RecentActivity, item)
	}
	return snap, nil
}
