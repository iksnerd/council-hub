package council

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Skill is one entry in the methodology registry — a task playbook made
// queryable from inside the DKR. Where rooms hold dialog and notebooks hold
// compiled knowledge, the skills registry holds Methodology and Training (two
// of Engelbart's under-served H-LAM/T legs): the standing "how we do X" that an
// agent can discover from any node without the skill files being on its local
// disk.
//
// A skill is discovery-first: Name + Description + WhenToUse are the catalog
// card an agent scans; Content (optional) carries the fuller playbook; Source
// points at where the canonical version lives (a skill directory, a URL).
// Project "" makes the skill global — listed in every project's view, the same
// rule a global notebook follows.
type Skill struct {
	Name        string
	Description string
	WhenToUse   string
	Content     string
	Project     string
	Tags        string
	Source      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RegisterSkill upserts a skill by name: registering an existing name updates
// the entry in place (a stable address, like a notebook ID or a typed link).
// created_at is preserved across updates; updated_at bumps.
func (s *Server) RegisterSkill(sk Skill) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	sk.Name = strings.TrimSpace(sk.Name)
	if sk.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	_, err := s.DB.Exec(`
		INSERT INTO skills (name, description, when_to_use, content, project, tags, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			when_to_use = excluded.when_to_use,
			content     = excluded.content,
			project     = excluded.project,
			tags        = excluded.tags,
			source      = excluded.source,
			updated_at  = CURRENT_TIMESTAMP`,
		sk.Name, strings.TrimSpace(sk.Description), strings.TrimSpace(sk.WhenToUse),
		sk.Content, normalizeProject(sk.Project), strings.TrimSpace(sk.Tags), strings.TrimSpace(sk.Source))
	return err
}

// RemoveSkill deletes a skill from the registry, erroring if no skill by that
// name exists.
func (s *Server) RemoveSkill(name string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	res, err := s.DB.Exec(`DELETE FROM skills WHERE name = ?`, strings.TrimSpace(name))
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("skill '%s' not found", name)
	}
	return nil
}

// GetSkill returns a single skill by exact name.
func (s *Server) GetSkill(name string) (Skill, error) {
	var sk Skill
	err := s.DB.QueryRow(`
		SELECT name, description, when_to_use, content, project, tags, source, created_at, updated_at
		FROM skills WHERE name = ?`, strings.TrimSpace(name),
	).Scan(&sk.Name, &sk.Description, &sk.WhenToUse, &sk.Content, &sk.Project, &sk.Tags, &sk.Source, &sk.CreatedAt, &sk.UpdatedAt)
	if err == sql.ErrNoRows {
		return sk, fmt.Errorf("skill '%s' not found", name)
	}
	return sk, err
}

// QuerySkills returns registry entries matching the filters, name-ordered. A
// non-empty project returns that project's skills plus global ones (project =
// "") — global methodology belongs to every project's playbook, the same rule
// ListNotebooks uses. query is a case-insensitive substring match across
// name/description/when_to_use/content; tag matches a single token within the
// comma-separated tags field. All filters are optional and AND together.
func (s *Server) QuerySkills(query, project, tag string) ([]Skill, error) {
	sqlStr := `SELECT name, description, when_to_use, content, project, tags, source, created_at, updated_at FROM skills`
	var where []string
	var args []any

	if project != "" {
		where = append(where, `project IN (?, '')`)
		args = append(args, normalizeProject(project))
	}
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		like := "%" + q + "%"
		where = append(where, `(LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(when_to_use) LIKE ? OR LOWER(content) LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if t := strings.ToLower(strings.TrimSpace(tag)); t != "" {
		// Bracket the normalized tag list with commas so a whole-token match
		// can't be fooled by a substring (tag "go" must not match "golang").
		where = append(where, `(',' || REPLACE(LOWER(tags), ' ', '') || ',') LIKE ?`)
		args = append(args, "%,"+t+",%")
	}
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY name ASC"

	rows, err := s.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var skills []Skill
	for rows.Next() {
		var sk Skill
		if err := rows.Scan(&sk.Name, &sk.Description, &sk.WhenToUse, &sk.Content, &sk.Project, &sk.Tags, &sk.Source, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, err
		}
		skills = append(skills, sk)
	}
	return skills, rows.Err()
}
